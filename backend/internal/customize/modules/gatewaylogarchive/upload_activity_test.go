package gatewaylogarchive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type uploadTestHTTP func(*http.Request) (*http.Response, error)

func (f uploadTestHTTP) Do(r *http.Request) (*http.Response, error) { return f(r) }

func testUploadClient(t *testing.T, httpDo uploadTestHTTP) *s3Objects {
	t.Helper()
	obj, err := newS3Objects(BucketConfig{Bucket: "test", Region: "us-east-1", ForcePathStyle: true, Endpoint: "http://s3.invalid", AccessKeyID: "test", SecretAccessKey: "test"})
	require.NoError(t, err)
	s := obj.(*s3Objects)
	opts := s.client.Options()
	opts.Retryer = aws.NopRetryer{}
	opts.HTTPClient = relayHTTPClient{auto: httpDo, http1: httpDo}
	s.client = s3.New(opts)
	return s
}

func testS3Response(req *http.Request, body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req, ContentLength: int64(len(body))}
}

// Exercise the real SDK, local file reader and progress accounting. Let a
// logical transfer last several minutes while bytes are flowing, then lose
// the ACK for one part. Virtual time makes this deterministic and fast.
func TestLocalArchiveSlowUploadAndMissingPartAck(t *testing.T) {
	const tail = "{}\n"
	data := bytes.Repeat([]byte("x"), 2*minPartSize)
	copy(data[minPartSize:], "second")
	data = append(data, tail...)
	path := filepath.Join(t.TempDir(), "old.jsonl")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		attempts := make(map[string]int)
		var starts, completes atomic.Int32
		var confirmed atomic.Int64
		p := &uploadProgress{}
		ctx := context.WithValue(t.Context(), uploadProgressKey{}, p)
		s := testUploadClient(t, func(r *http.Request) (*http.Response, error) {
			q := r.URL.Query()
			switch {
			case r.Method == "POST" && q.Has("uploads"):
				starts.Add(1)
				return testS3Response(r, `<InitiateMultipartUploadResult><UploadId>U</UploadId></InitiateMultipartUploadResult>`), nil
			case r.Method == "PUT" && q.Get("uploadId") == "U":
				num := q.Get("partNumber")
				mu.Lock()
				attempts[num]++
				attempt := attempts[num]
				mu.Unlock()
				h := sha256.New()
				for {
					n, err := io.CopyN(h, r.Body, 4<<20)
					if n > 0 {
						// Always make progress inside the idle interval, even
						// though a single full part takes over two minutes.
						time.Sleep(40 * time.Second)
					}
					if r.Context().Err() != nil {
						return nil, r.Context().Err()
					}
					if err != nil {
						assert.ErrorIs(t, err, io.EOF)
						break
					}
				}
				want := map[string][]byte{"1": data[:minPartSize], "2": data[minPartSize : 2*minPartSize], "3": data[2*minPartSize:]}[num]
				expected := sha256.Sum256(want)
				assert.Equal(t, expected[:], h.Sum(nil))
				if num == "2" && attempt == 1 {
					// Body fully accepted, no response. The exact same part
					// and ID must be replayed without recounting it as confirmed.
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				if num == "2" && attempt == 2 {
					assert.True(t, relayProtocolFor(r.Context()).useHTTP1())
				}
				resp := testS3Response(r, "")
				resp.Header.Set("ETag", `"p`+num+`"`)
				return resp, nil
			case r.Method == "POST" && q.Get("uploadId") == "U":
				completes.Add(1)
				return testS3Response(r, `<CompleteMultipartUploadResult><ETag>done</ETag></CompleteMultipartUploadResult>`), nil
			default:
				return nil, fmt.Errorf("unexpected %s %s", r.Method, r.URL.RequestURI())
			}
		})
		start := time.Now()
		require.NoError(t, s.Upload(ctx, "old.jsonl", path, func(n int64) { confirmed.Add(n) }))
		require.Greater(t, time.Since(start), 2*time.Minute)
		require.Equal(t, int32(1), starts.Load())
		require.Equal(t, int32(1), completes.Load())
		require.Equal(t, map[string]int{"1": 1, "2": 2, "3": 1}, attempts)
		require.Equal(t, int64(len(data)), confirmed.Load())
		require.Equal(t, int64(len(data)+minPartSize), p.bodyBytes.Load())
		require.Equal(t, int64(1), p.retries.Load())
	})
}

func TestArchiveUploadSilentStorageReleasesQueue(t *testing.T) {
	for _, stage := range []string{"PutObject", "CreateMultipartUpload", "UploadPart", "CompleteMultipartUpload"} {
		t.Run(stage, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var aborts, attempts int
				s := testUploadClient(t, func(r *http.Request) (*http.Response, error) {
					q := r.URL.Query()
					switch {
					case r.Method == "DELETE":
						aborts++
						return testS3Response(r, ""), nil
					case q.Has("uploads") && stage != "CreateMultipartUpload":
						return testS3Response(r, `<InitiateMultipartUploadResult><UploadId>U</UploadId></InitiateMultipartUploadResult>`), nil
					case q.Has("partNumber") && stage != "UploadPart":
						resp := testS3Response(r, "")
						resp.Header.Set("ETag", `"p"`)
						return resp, nil
					}
					attempts++
					<-r.Context().Done()
					return nil, r.Context().Err()
				})
				ctx := t.Context()
				var sent int64
				size := streamMinPart + 1
				if stage == "PutObject" {
					size = 1
				}
				err := s.UploadStream(ctx, "file.jsonl", int64(size), func(w io.Writer) error {
					_, err := w.Write(make([]byte, size))
					return err
				}, func(n int64) { sent += n })
				require.ErrorIs(t, err, context.DeadlineExceeded)
				require.NoError(t, ctx.Err(), "only the storage operation was canceled")
				switch stage {
				case "PutObject", "CreateMultipartUpload":
					require.Zero(t, aborts)
					require.Zero(t, sent)
				case "UploadPart":
					require.Equal(t, 3, attempts)
					require.Equal(t, 1, aborts)
					require.Zero(t, sent)
				default:
					require.Equal(t, 1, aborts)
					require.Equal(t, int64(streamMinPart+1), sent)
				}
			})
		})
	}
}

func TestRustFSProbeRequiresMultipartAndCleansItsMarker(t *testing.T) {
	for _, broken := range []bool{false, true} {
		t.Run(fmt.Sprintf("multipartDenied=%t", broken), func(t *testing.T) {
			var marker string
			var completed, aborted, deleted bool
			s := testUploadClient(t, func(r *http.Request) (*http.Response, error) {
				q := r.URL.Query()
				resp := testS3Response(r, "")
				switch {
				case r.Method == "HEAD":
				case r.Method == "PUT" && !q.Has("partNumber"):
					b, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					marker = string(b)
				case r.Method == "POST" && q.Has("uploads"):
					return testS3Response(r, `<InitiateMultipartUploadResult><UploadId>U</UploadId></InitiateMultipartUploadResult>`), nil
				case r.Method == "PUT" && q.Get("uploadId") == "U":
					if broken {
						resp.StatusCode = http.StatusForbidden
						break
					}
					resp.Header.Set("ETag", `"p"`)
				case r.Method == "POST" && q.Get("uploadId") == "U":
					completed, marker = true, "multipart connection test"
					return testS3Response(r, `<CompleteMultipartUploadResult><ETag>done</ETag></CompleteMultipartUploadResult>`), nil
				case r.Method == "GET" && marker != "":
					assert.Equal(t, "bytes=0-0", r.Header.Get("Range"))
					return testS3Response(r, marker[:1]), nil
				case r.Method == "DELETE" && q.Has("uploadId"):
					aborted = true
				case r.Method == "DELETE":
					deleted = true
				default:
					return nil, fmt.Errorf("unexpected %s %s", r.Method, r.URL.RequestURI())
				}
				return resp, nil
			})
			s.cfg.StorageRole = "rustfs"
			var failures []string
			for _, step := range s.Test(t.Context()) {
				if !step.OK {
					failures = append(failures, step.Name)
				}
			}
			if broken {
				require.Equal(t, []string{"主归档分片写入与合并"}, failures)
			} else {
				require.Empty(t, failures)
			}
			require.Equal(t, !broken, completed)
			require.Equal(t, broken, aborted)
			require.True(t, deleted)
		})
	}
}
