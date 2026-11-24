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
	UploadStream(ctx context.Context, key string, sizeHint int64, produce func(io.Writer) error, progress func(int64)) error
	Presign(ctx context.Context, key, filename string, ttl time.Duration) (string, error)
	OpenRange(ctx context.Context, key string, offset, length int64) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Test(ctx context.Context) []TestStep
}

type TestStep struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type s3Objects struct {
	client *s3.Client
	cfg    BucketConfig
}

func newS3Objects(cfg BucketConfig) (objectStore, error) {
	if cfg.Bucket == "" {
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
		// Clone the SDK transport *after* defaults and custom CA configuration.
		// Protocol fallback is scoped to one transfer, not the gateway transport.
		transport := relayHTTPClient{auto: o.HTTPClient, http1: o.HTTPClient}
		if base, ok := o.HTTPClient.(*awshttp.BuildableClient); ok {
			transport.http1 = base.WithTransportOptions(func(tr *http.Transport) {
				tr.Protocols = new(http.Protocols)
				tr.Protocols.SetHTTP1(true)
				tr.ForceAttemptHTTP2 = false
				if tr.TLSClientConfig != nil {
					tr.TLSClientConfig.NextProtos = []string{"http/1.1"}
				}
			})
		}
		o.HTTPClient = transport
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

func (s *s3Objects) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key)})
	return err
}

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
	ctx, cancel := context.WithTimeout(ctx, uploadControlTimeout)
	defer cancel()
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
	minPartSize        = 16 << 20
	multipartThreshold = minPartSize
	maxParts           = 9000
	partConcurrency    = 2
	partAttempts       = 6
)

// partRetryDelay is 2s, 4s, 8s, 16s, 30s; tests shorten it.
var partRetryDelay = func(attempt int) time.Duration {
	return min(time.Second<<attempt, 30*time.Second)
}

// withBucketProtocol starts on HTTP/1.1 when this bucket's endpoint already
// broke HTTP/2 (in this process, or in the error saved by the last attempt).
func (p *pipeline) withBucketProtocol(ctx context.Context, bucketID int64, lastError string) context.Context {
	_, known := p.http1Buckets.Load(bucketID)
	return withRelayProtocol(ctx, known || retryRelayHTTP1(lastError))
}

func (p *pipeline) rememberProtocol(ctx context.Context, bucketID int64) {
	if relayProtocolFor(ctx).useHTTP1() {
		p.http1Buckets.Store(bucketID, true)
	}
}

func partSizeFor(size, minimum, targetParts int64) int64 {
	part := minimum
	if need := (size-1)/targetParts + 1; need > part {
		part = (need + (1<<20 - 1)) &^ (1<<20 - 1)
	}
	return part
}

// Upload sends small files with one PUT and large ones (up to the 10 GB
// default and beyond) as a multipart upload with concurrent parts.
func (s *s3Objects) Upload(ctx context.Context, key, path string, progress func(int64)) error {
	ctx = uploadProtocolContext(ctx)
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
		return s.put(ctx, key, io.NewSectionReader(f, 0, size), size, progress)
	}
	uploadID, err := s.startMultipart(ctx, key)
	if err != nil {
		return err
	}
	partSize := partSizeFor(size, minPartSize, maxParts)
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
				part, perr := s.sendPart(pctx, key, uploadID, int32(i+1), io.NewSectionReader(f, offset, n), n)
				if perr != nil {
					errs <- fmt.Errorf("part %d: %w", i+1, perr)
					cancel()
					continue
				}
				parts[i] = part
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
		s.abortMultipart(ctx, key, uploadID)
		return err
	}
	if ctx.Err() != nil {
		s.abortMultipart(ctx, key, uploadID)
		return ctx.Err()
	}
	err = s.completeMultipart(ctx, key, uploadID, parts)
	if err != nil {
		s.abortMultipart(ctx, key, uploadID)
	}
	return err
}

func (s *s3Objects) put(ctx context.Context, key string, body io.ReadSeeker, size int64, progress func(int64)) error {
	progressFor(ctx).setPhase("发送对象，等待存储桶确认")
	err := watchUploadRequest(ctx, func(attempt context.Context) error {
		_, err := s.client.PutObject(attempt, &s3.PutObjectInput{
			Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), Body: body,
			ContentLength: aws.Int64(size), ContentType: aws.String(archiveContentType(key)),
		})
		return err
	})
	if err == nil && progress != nil {
		progress(size)
	}
	return err
}

func (s *s3Objects) startMultipart(ctx context.Context, key string) (*string, error) {
	progressFor(ctx).setPhase("创建分片上传")
	ctx, cancel := context.WithTimeout(ctx, uploadControlTimeout)
	defer cancel()
	out, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), ContentType: aws.String(archiveContentType(key)),
	})
	if err != nil {
		return nil, fmt.Errorf("创建分片上传：%w", err)
	}
	if aws.ToString(out.UploadId) == "" {
		return nil, errors.New("存储桶未返回分片上传 ID")
	}
	return out.UploadId, nil
}

func (s *s3Objects) sendPart(ctx context.Context, key string, uploadID *string, number int32, body io.ReadSeeker, size int64) (types.CompletedPart, error) {
	var err error
	progress := progressFor(ctx)
	progress.setPhase("发送分片，等待存储桶确认")
	for attempt := 0; attempt < partAttempts; attempt++ {
		if ctx.Err() != nil {
			return types.CompletedPart{}, ctx.Err()
		}
		if attempt > 0 {
			// A failed part restarts only that part. Waiting lets a proxy that
			// reset the connection recover instead of failing the whole file.
			if progress != nil {
				progress.retries.Add(1)
				progress.setPhase(fmt.Sprintf("分片 %d 第 %d 次重试", number, attempt))
			}
			select {
			case <-ctx.Done():
				return types.CompletedPart{}, ctx.Err()
			case <-time.After(partRetryDelay(attempt)):
			}
			progress.setPhase("发送分片，等待存储桶确认")
		}
		if _, err = body.Seek(0, io.SeekStart); err != nil {
			break
		}
		var out *s3.UploadPartOutput
		err = watchUploadRequest(ctx, func(reqCtx context.Context) error {
			var reqErr error
			out, reqErr = s.client.UploadPart(reqCtx, &s3.UploadPartInput{
				Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), UploadId: uploadID,
				PartNumber: aws.Int32(number), Body: body, ContentLength: aws.Int64(size),
			})
			return reqErr
		})
		if err == nil {
			if aws.ToString(out.ETag) == "" {
				return types.CompletedPart{}, errors.New("存储桶未返回分片 ETag")
			}
			return types.CompletedPart{ETag: out.ETag, PartNumber: aws.Int32(number)}, nil
		}
		if ctx.Err() != nil {
			break
		}
		// The error may arrive after the transport returned (e.g. while
		// decoding an ACK), or from the request inactivity watchdog.
		relayProtocolFor(ctx).observe(err)
	}
	return types.CompletedPart{}, err
}

func (s *s3Objects) completeMultipart(ctx context.Context, key string, uploadID *string, parts []types.CompletedPart) error {
	progressFor(ctx).setPhase("等待存储桶合并分片")
	ctx, cancel := context.WithTimeout(ctx, uploadCommitTimeout)
	defer cancel()
	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), UploadId: uploadID,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		return fmt.Errorf("等待存储桶合并分片：%w", err)
	}
	return nil
}

func (s *s3Objects) abortMultipart(parent context.Context, key string, uploadID *string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), time.Minute)
	defer cancel()
	_, _ = s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key), UploadId: uploadID})
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
	// Never splice bytes from offset zero into a resumed stream if an endpoint
	// ignores Range. A range reaching EOF can legitimately be shorter.
	rangeHeader := aws.ToString(out.ContentRange)
	if (rangeHeader != "" && !strings.HasPrefix(rangeHeader, fmt.Sprintf("bytes %d-", offset))) ||
		(rangeHeader == "" && (offset != 0 || aws.ToInt64(out.ContentLength) > length)) {
		_ = out.Body.Close()
		return nil, fmt.Errorf("对象存储未返回请求的 Range（起始字节 %d，长度 %d）", offset, length)
	}
	return out.Body, nil
}

// A single tiny *last* part is legal S3 multipart. Checking the same API used
// by staging avoids clearing fallback merely because a small ordinary PUT
// works. This probes capability, not sustained bandwidth.
func (s *s3Objects) probeMultipart(ctx context.Context, key string) error {
	ctx = uploadProtocolContext(ctx)
	id, err := s.startMultipart(ctx, key)
	if err != nil {
		return err
	}
	const marker = "multipart connection test"
	part, err := s.sendPart(ctx, key, id, 1, strings.NewReader(marker), int64(len(marker)))
	if err == nil {
		err = s.completeMultipart(ctx, key, id, []types.CompletedPart{part})
	}
	if err != nil {
		s.abortMultipart(ctx, key, id)
	}
	return err
}

// Test checks bucket access and write permission with a tiny marker object.
func (s *s3Objects) Test(ctx context.Context) []TestStep {
	var steps []TestStep
	step := func(name string, err error) bool {
		st := TestStep{Name: name, OK: err == nil}
		if err != nil {
			st.Error = storageErrorText(err)
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
		if s.cfg.StorageRole == "rustfs" {
			firstByte := byte('s')
			if step("主归档分片写入与合并", s.probeMultipart(ctx, key)) {
				firstByte = 'm' // read the completed multipart representation
			}
			body, readErr := s.OpenRange(ctx, key, 0, 1)
			if readErr == nil {
				var probe [1]byte
				_, readErr = io.ReadFull(body, probe[:])
				_ = body.Close()
				if readErr == nil && probe[0] != firstByte {
					readErr = errors.New("读取的内容与测试文件不符")
				}
			}
			step("读取主归档测试文件", readErr)
		}
		_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.cfg.Bucket), Key: aws.String(key)})
		if s.cfg.StorageRole == "rustfs" {
			step("清理主归档测试文件", err)
		} else {
			step("删除测试文件（无删除权限可忽略）", err)
		}
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
	pass := &uploadPass{down: map[int64]bool{}}
	for p.ctx.Err() == nil {
		buckets := p.enabledBuckets()
		if len(buckets) == 0 {
			return
		}
		row, err := p.store.ClaimUpload(p.ctx, p.node, time.Now())
		if err != nil {
			p.report(fmt.Errorf("读取上传队列失败：%w", err))
			return
		}
		if row == nil {
			return
		}
		if row.StageBucketID != nil {
			if err := p.sealStaged(row); err != nil {
				p.failUpload(row, err)
			} else {
				p.stats.uploaded.Add(1)
			}
			continue
		}
		if primary, ok := primaryBucket(buckets); ok {
			if err := p.uploadLocalToPrimary(row, primary); err != nil {
				p.failUpload(row, err)
			} else {
				p.stats.uploaded.Add(1)
			}
			continue
		}
		if err := p.uploadToBuckets(buckets, pass, row); err != nil {
			p.failUpload(row, err)
			continue
		}
		p.stats.uploaded.Add(1)
	}
}

type uploadPass struct {
	down map[int64]bool
}

type bucketClient struct {
	config  BucketConfig
	objects objectStore
}

func (p *pipeline) bucketClient(cfg BucketConfig) (objectStore, error) {
	p.clientsMu.Lock()
	cached, ok := p.clients[cfg.ID]
	p.clientsMu.Unlock()
	if ok && cached.config == cfg {
		return cached.objects, nil
	}
	objects, err := p.objects(cfg)
	if err != nil {
		return nil, err
	}
	p.clientsMu.Lock()
	defer p.clientsMu.Unlock()
	if p.clients == nil {
		p.clients = make(map[int64]bucketClient)
	}
	p.clients[cfg.ID] = bucketClient{config: cfg, objects: objects}
	return objects, nil
}

// uploadToBuckets tries the enabled buckets in priority order and stops at the
// first success; the bundle records which bucket received it.
func (p *pipeline) uploadToBuckets(buckets []BucketConfig, pass *uploadPass, row *Bundle) error {
	order := make([]BucketConfig, 0, len(buckets))
	for _, b := range buckets {
		if !pass.down[b.ID] {
			order = append(order, b)
		}
	}
	if len(order) == 0 {
		order = buckets // everything failed earlier in this pass: try again in order
	}
	var failures []string
	for _, bucket := range order {
		if p.ctx.Err() != nil {
			return p.ctx.Err()
		}
		prepared, err := p.prepareUpload(row, bucket.redact(p.defaultPrivacy()))
		if err != nil {
			return err
		}
		objects, err := p.bucketClient(bucket)
		if err != nil {
			pass.down[bucket.ID] = true
			failures = append(failures, bucket.Label()+"："+storageErrorText(err))
			continue
		}
		err = p.uploadOne(bucket, objects, row, prepared)
		if err == nil {
			delete(pass.down, bucket.ID)
			return nil
		}
		if errors.Is(err, errLocalBundleMissing) || errors.Is(err, errUploadSource) || errors.Is(err, errBundleDeleted) {
			return err // no other bucket can help
		}
		pass.down[bucket.ID] = true
		failures = append(failures, bucket.Label()+"："+storageErrorText(err))
	}
	if len(failures) == 1 {
		return errors.New(failures[0])
	}
	return fmt.Errorf("全部 %d 个存储桶上传失败：%s", len(failures), strings.Join(failures, "；"))
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
	if p.ctx.Err() != nil {
		return // shutdown; the row is reset to "sealed" on the next start
	}
	if errors.Is(err, errBundleDeleted) {
		// Hand the row to the delete worker; it is not an upload failure.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if store, ok := p.store.(*Store); ok {
			_ = store.releaseDeleted(ctx, row.ID)
		}
		select {
		case p.deleteWake <- struct{}{}:
		default:
		}
		return
	}
	p.stats.uploadErrors.Add(1)
	p.report(fmt.Errorf("上传 %s 失败：%w", row.FileName, err))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = p.store.MarkUploadFailed(ctx, row.ID, storageErrorText(err), time.Now().Add(backoff(row.Attempts)))
}

// chooseKey keeps "<model>-<day>.jsonl" unless the object already exists (for
// example written by another deployment); then the next free part name.
func (p *pipeline) chooseKey(ctx context.Context, cfg BucketConfig, objects objectStore, row *Bundle) (string, error) {
	for part := row.Part; part < row.Part+100; part++ {
		key := cfg.ObjectKey(row.Day, BundleFileName(row.ModelKey, row.Day, part))
		taken, err := p.store.ObjectKeyTaken(ctx, objects.Bucket(), key, row.ID)
		if err != nil {
			return "", err
		}
		if taken {
			continue
		}
		exists, err := objects.Exists(ctx, key)
		if err != nil {
			// A write-only credential may not HEAD. A network timeout does not
			// establish that the key is free: retry rather than overwrite it.
			var response *awshttp.ResponseError
			if errors.As(err, &response) && response.HTTPStatusCode() == http.StatusForbidden {
				return key, nil
			}
			return "", fmt.Errorf("检查存储桶中的归档名称：%w", err)
		}
		if !exists {
			return key, nil
		}
	}
	return "", errors.New("找不到可用的对象名称")
}

var errLocalBundleMissing = errors.New("本地归档文件不存在")

func (p *pipeline) uploadOne(cfg BucketConfig, objects objectStore, row *Bundle, prepared *preparedUpload) error {
	if _, err := os.Stat(row.LocalPath); err != nil {
		return fmt.Errorf("%w：%v", errLocalBundleMissing, err)
	}
	progress := &uploadProgress{FileName: prepared.fileName, Total: row.SizeBytes}
	if prepared.entries != nil {
		progress.Total = prepared.sizeHint // redaction may change the final size slightly
	}
	progress.setPhase("检查归档名称")
	uploadCtx, finish := p.beginUpload(row, progress, cfg.Label())
	defer finish()
	uploadCtx = p.withBucketProtocol(uploadCtx, cfg.ID, row.LastError)
	defer p.rememberProtocol(uploadCtx, cfg.ID)
	key, err := p.chooseKey(uploadCtx, cfg, objects, row)
	if err != nil {
		return uploadError(uploadCtx, err)
	}
	progress.setPhase("读取归档")
	layout, err := prepared.upload(uploadCtx, objects, key, func(n int64) { progress.sent.Add(n) })
	if err != nil {
		return uploadError(uploadCtx, err)
	}
	progress.setPhase("记录上传结果")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var commitErr error
	if cfg.StorageRole == "rustfs" {
		commitErr = p.store.(*Store).commitUploaded(ctx, row.ID, cfg.ID, objects.Bucket(), key, layout, p.deliveryTargets(row, cfg), prepared.redact)
	} else {
		commitErr = p.store.MarkUploaded(ctx, row.ID, cfg.ID, objects.Bucket(), key, layout)
	}
	if errors.Is(commitErr, errBundleDeleted) {
		p.discardUploaded(objects, key)
	}
	if commitErr != nil {
		return commitErr
	}
	if cfg.StorageRole == "rustfs" {
		p.wakeRelay()
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
		if row.StageBucketID != nil {
			if err := p.cleanupStaged(ctx, row); err != nil {
				p.report(err)
			}
			continue
		}
		if err := os.Remove(row.LocalPath); err == nil || os.IsNotExist(err) {
			_ = p.store.MarkLocalDeleted(ctx, row.ID)
		}
		removeLegacyUploadTemp(row.LocalPath)
	}
}
