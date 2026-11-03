package gatewaylogarchive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// objectStore is the bucket port; tests replace it with a local fake.
type objectStore interface {
	Bucket() string
	Exists(ctx context.Context, key string) (bool, error)
	Upload(ctx context.Context, key, path string, progress func(int64)) error
	Presign(ctx context.Context, key, filename string, ttl time.Duration) (string, error)
	OpenRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error)
	Test(ctx context.Context) []TestStep
}

type TestStep struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type s3Objects struct {
	client *s3.Client
	cfg    Config
}

func newS3Objects(cfg Config) (objectStore, error) {
	if !cfg.StorageReady() {
		return nil, errNoStorage
	}
	region := cfg.Region
	if region == "" {
		region = "auto"
	}
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if cfg.AccessKeyID != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
		// Same compatibility settings as the host's backup storage client:
		// works with AWS S3, R2, OSS and MinIO.
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &s3Objects{client: client, cfg: cfg}, nil
}

func (s *s3Objects) Bucket() string { return s.cfg.Bucket }

func isNotFound(err error) bool {
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var nk *types.NoSuchKey
	if errors.As(err, &nk) {
		return true
	}
	var re *awshttp.ResponseError
	return errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound
}

func (s *s3Objects) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key)})
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, err
}

const (
	multipartThreshold = 64 << 20
	minPartSize        = 64 << 20
	maxParts           = 9000
	partConcurrency    = 4
)

func partSizeFor(size int64) int64 {
	part := int64(minPartSize)
	if need := (size + maxParts - 1) / maxParts; need > part {
		part = (need + (1<<20 - 1)) &^ (1<<20 - 1)
	}
	return part
}

// Upload sends small files with one PUT and large ones (up to the 10 GB
// default and beyond) as a multipart upload with concurrent parts.
func (s *s3Objects) Upload(ctx context.Context, key, path string, progress func(int64)) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()
	if size < multipartThreshold {
		_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), Body: io.NewSectionReader(f, 0, size),
			ContentLength: aws.Int64(size), ContentType: aws.String("application/zip"),
		})
		if err == nil && progress != nil {
			progress(size)
		}
		return err
	}
	created, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), ContentType: aws.String("application/zip"),
	})
	if err != nil {
		return err
	}
	abort := func() {
		actx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, _ = s.client.AbortMultipartUpload(actx, &s3.AbortMultipartUploadInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), UploadId: created.UploadId})
	}
	partSize := partSizeFor(size)
	count := int((size + partSize - 1) / partSize)
	parts := make([]types.CompletedPart, count)
	jobs := make(chan int)
	errs := make(chan error, count)
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	for w := 0; w < partConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				offset := int64(i) * partSize
				n := partSize
				if offset+n > size {
					n = size - offset
				}
				var out *s3.UploadPartOutput
				var perr error
				for attempt := 0; attempt < 3; attempt++ {
					out, perr = s.client.UploadPart(pctx, &s3.UploadPartInput{
						Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), UploadId: created.UploadId,
						PartNumber: aws.Int32(int32(i + 1)), Body: io.NewSectionReader(f, offset, n), ContentLength: aws.Int64(n),
					})
					if perr == nil || pctx.Err() != nil {
						break
					}
				}
				if perr != nil {
					errs <- fmt.Errorf("part %d: %w", i+1, perr)
					cancel()
					continue
				}
				parts[i] = types.CompletedPart{ETag: out.ETag, PartNumber: aws.Int32(int32(i + 1))}
				if progress != nil {
					progress(n)
				}
			}
		}()
	}
feed:
	for i := 0; i < count; i++ {
		select {
		case jobs <- i:
		case <-pctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	close(errs)
	if err := <-errs; err != nil {
		abort()
		return err
	}
	if ctx.Err() != nil {
		abort()
		return ctx.Err()
	}
	_, err = s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), UploadId: created.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		abort()
	}
	return err
}

func (s *s3Objects) Presign(ctx context.Context, key, filename string, ttl time.Duration) (string, error) {
	out, err := s3.NewPresignClient(s.client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key),
		ResponseContentDisposition: aws.String(contentDisposition(filename)),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (s *s3Objects) OpenRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key),
		Range: aws.String(fmt.Sprintf("bytes=%d-%d", offset, offset+length-1)),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

// Test checks bucket access and write permission with a tiny marker object
// that is deleted again (best effort).
func (s *s3Objects) Test(ctx context.Context) []TestStep {
	var steps []TestStep
	step := func(name string, err error) bool {
		st := TestStep{Name: name, OK: err == nil}
		if err != nil {
			st.Error = err.Error()
		}
		steps = append(steps, st)
		return err == nil
	}
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.cfg.Bucket)})
	step("访问存储桶", err)
	key := s.cfg.ObjectKey("_sub2api", "connection-test-"+fmt.Sprint(time.Now().UnixNano())+".txt")
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), Body: strings.NewReader("sub2api gateway log archive connection test"),
		ContentLength: aws.Int64(int64(len("sub2api gateway log archive connection test"))),
	})
	if step("写入测试文件 "+key, err) {
		_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key)})
		step("删除测试文件（无删除权限可忽略）", err)
	}
	return steps
}

func contentDisposition(filename string) string {
	ascii := strings.Map(func(r rune) rune {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, filename)
	return `attachment; filename="` + ascii + `"; filename*=UTF-8''` + url.PathEscape(filename)
}

// uploadLoop uploads sealed bundles of this node one at a time, retrying
// failures with backoff, and removes local copies after the keep window.
func (p *pipeline) uploadLoop() {
	defer p.workers.Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		p.uploadPending()
		p.cleanupLocal()
		select {
		case <-p.ctx.Done():
			return
		case <-p.uploadWake:
		case <-ticker.C:
		}
	}
}

func (p *pipeline) uploadPending() {
	cfg := p.cfg.Load()
	if cfg == nil || !cfg.StorageReady() {
		return
	}
	var objects objectStore
	for p.ctx.Err() == nil {
		row, err := p.store.ClaimUpload(p.ctx, p.node, time.Now())
		if err != nil {
			p.report(fmt.Errorf("读取上传队列失败：%w", err))
			return
		}
		if row == nil {
			return
		}
		if objects == nil {
			if objects, err = p.objects(*cfg); err != nil {
				p.failUpload(row, err)
				return
			}
		}
		if err := p.uploadOne(*cfg, objects, row); err != nil {
			p.failUpload(row, err)
			continue
		}
		p.stats.uploaded.Add(1)
	}
}

func backoff(attempts int) time.Duration {
	d := time.Minute
	for i := 1; i < attempts && d < time.Hour; i++ {
		d *= 2
	}
	if d > time.Hour {
		d = time.Hour
	}
	return d
}

func (p *pipeline) failUpload(row *Bundle, err error) {
	p.stats.uploadErrors.Add(1)
	if p.ctx.Err() != nil {
		return // shutdown; the row is reset to "sealed" on the next start
	}
	p.report(fmt.Errorf("上传 %s 失败：%w", row.FileName, err))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = p.store.MarkUploadFailed(ctx, row.ID, err.Error(), time.Now().Add(backoff(row.Attempts)))
}

// chooseKey keeps "<model>-<day>.zip" unless the object already exists (for
// example written by another deployment); then the next free part name.
func (p *pipeline) chooseKey(cfg Config, objects objectStore, row *Bundle) (string, error) {
	for part := row.Part; part < row.Part+100; part++ {
		key := cfg.ObjectKey(row.Day, BundleFileName(row.ModelKey, row.Day, part))
		taken, err := p.store.ObjectKeyTaken(p.ctx, objects.Bucket(), key, row.ID)
		if err != nil {
			return "", err
		}
		if taken {
			continue
		}
		exists, err := objects.Exists(p.ctx, key)
		if err != nil {
			// Without HeadObject permission the configured name is used.
			return key, nil
		}
		if !exists {
			return key, nil
		}
	}
	return "", errors.New("找不到可用的对象名称")
}

func (p *pipeline) uploadOne(cfg Config, objects objectStore, row *Bundle) error {
	if _, err := os.Stat(row.LocalPath); err != nil {
		return fmt.Errorf("本地压缩包不存在：%w", err)
	}
	key, err := p.chooseKey(cfg, objects, row)
	if err != nil {
		return err
	}
	progress := &uploadProgress{BundleID: row.ID, FileName: row.FileName, Total: row.SizeBytes}
	p.uploading.Store(progress)
	defer p.uploading.Store(nil)
	if err := objects.Upload(p.ctx, key, row.LocalPath, func(n int64) { progress.sent.Add(n) }); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.store.MarkUploaded(ctx, row.ID, objects.Bucket(), key); err != nil {
		return err
	}
	if p.keepLocal.Load() == 0 {
		if err := os.Remove(row.LocalPath); err == nil || os.IsNotExist(err) {
			_ = p.store.MarkLocalDeleted(ctx, row.ID)
		}
	}
	return nil
}

func (p *pipeline) cleanupLocal() {
	keep := time.Duration(p.keepLocal.Load())
	ctx, cancel := context.WithTimeout(p.ctx, time.Minute)
	defer cancel()
	rows, err := p.store.LocalExpired(ctx, p.node, time.Now().Add(-keep))
	if err != nil {
		return
	}
	for _, row := range rows {
		if err := os.Remove(row.LocalPath); err == nil || os.IsNotExist(err) {
			_ = p.store.MarkLocalDeleted(ctx, row.ID)
		}
	}
}
