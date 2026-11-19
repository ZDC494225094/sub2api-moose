package gatewaylogarchive

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestS3StreamsTransformedBytesAndAbortsUnfinishedObjects(t *testing.T) {
	const key = "gateway.jsonl"
	sourceError := errors.New("invalid late entry")
	for _, scenario := range []string{"single", "multipart", "exact multiple", "late source error", "part error", "complete error", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			var mu sync.Mutex
			var puts, starts, completes, aborts, retries int
			var parts [][]byte
			var uploaded []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				query := r.URL.Query()
				switch {
				case r.Method == http.MethodPost && query.Has("uploads"):
					starts++
					assert.Equal(t, archiveContentType(key), r.Header.Get("Content-Type"))
					fmt.Fprint(w, `<InitiateMultipartUploadResult><UploadId>id-1</UploadId></InitiateMultipartUploadResult>`)
				case r.Method == http.MethodPut:
					body, err := io.ReadAll(r.Body)
					assert.NoError(t, err)
					assert.Equal(t, int64(len(body)), r.ContentLength)
					if !query.Has("partNumber") {
						assert.Equal(t, archiveContentType(key), r.Header.Get("Content-Type"))
						puts++
						uploaded = body
						return
					}
					assert.Equal(t, "id-1", query.Get("uploadId"))
					number, err := strconv.Atoi(query.Get("partNumber"))
					assert.NoError(t, err)
					assert.Equal(t, len(parts)+1, number)
					if number == 2 && (scenario == "part error" || scenario == "multipart" && retries == 0) {
						retries++
						assert.Equal(t, "tail", string(body))
						http.Error(w, `<Error><Code>AccessDenied</Code></Error>`, http.StatusForbidden)
						return
					}
					parts = append(parts, body)
					w.Header().Set("ETag", fmt.Sprintf(`"part-%d"`, number))
				case r.Method == http.MethodPost && query.Get("uploadId") == "id-1":
					completes++
					if scenario == "complete error" {
						http.Error(w, `<Error><Code>AccessDenied</Code></Error>`, http.StatusForbidden)
						return
					}
					var result struct {
						Parts []struct {
							Number int `xml:"PartNumber"`
							ETag   string
						} `xml:"Part"`
					}
					assert.NoError(t, xml.NewDecoder(r.Body).Decode(&result))
					assert.Len(t, result.Parts, len(parts))
					for index, part := range result.Parts {
						assert.Equal(t, index+1, part.Number)
						assert.Equal(t, fmt.Sprintf(`"part-%d"`, index+1), part.ETag)
					}
					uploaded = bytes.Join(parts, nil)
					fmt.Fprint(w, `<CompleteMultipartUploadResult><ETag>done</ETag></CompleteMultipartUploadResult>`)
				case r.Method == http.MethodDelete && query.Get("uploadId") == "id-1":
					aborts++
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected S3 request: %s %s", r.Method, r.URL.RequestURI())
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			objects, err := newS3Objects(BucketConfig{Region: "us-east-1", Endpoint: server.URL, Bucket: "bucket", ForcePathStyle: true, AccessKeyID: "test", SecretAccessKey: "test"})
			require.NoError(t, err)
			size := streamMinPart + len("tail")
			if scenario == "single" {
				size = 31
			} else if scenario == "exact multiple" {
				size = 2 * streamMinPart
			}
			data := bytes.Repeat([]byte("x"), size)
			copy(data[len(data)-4:], "tail")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var sent int64
			err = objects.UploadStream(ctx, key, 1, func(dst io.Writer) error {
				if _, err := dst.Write(data); err != nil {
					return err
				}
				if scenario != "single" {
					require.Equal(t, int64(streamMinPart), sent, "first part reached storage before the producer finished")
				}
				switch scenario {
				case "late source error":
					return sourceError
				case "canceled":
					cancel()
				}
				return nil
			}, func(n int64) { sent += n })
			mu.Lock()
			defer mu.Unlock()
			switch scenario {
			case "single", "multipart", "exact multiple":
				require.NoError(t, err)
				require.Equal(t, int64(size), sent)
				require.True(t, bytes.Equal(data, uploaded))
				require.Zero(t, aborts)
				if scenario == "single" {
					require.Equal(t, 1, puts)
					require.Zero(t, starts)
				} else {
					require.Equal(t, 1, starts)
					require.Equal(t, 1, completes)
					require.Len(t, parts, 2)
					require.Len(t, parts[0], streamMinPart)
					if scenario == "multipart" {
						require.Equal(t, 1, retries)
					}
				}
			default:
				require.Error(t, err)
				if scenario == "late source error" {
					require.ErrorIs(t, err, sourceError)
				} else if scenario == "canceled" {
					require.ErrorIs(t, err, context.Canceled)
				}
				require.Nil(t, uploaded)
				require.Equal(t, 1, starts)
				require.Equal(t, 1, aborts)
			}
		})
	}
}
