//go:build integration

package gatewaylogarchive

import (
	"context"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type relayTestDo func(*http.Request) (*http.Response, error)

func (f relayTestDo) Do(r *http.Request) (*http.Response, error) { return f(r) }

// Use the real SDK and TLS for both buckets. Inject the observed reset at the
// HTTP boundary, after a successful GET or PUT, without a production endpoint.
func TestRelayLargeJSONL(t *testing.T) {
	const rowBytes = 1 << 20
	const tenGiB = int64(10 << 30)
	padding := strings.Repeat("aBcD0123", rowBytes/8)
	for _, tc := range []struct {
		hundredths int64
		readCut    int64 // after receiving this many source bytes
		lostAck    int   // destination accepted this part but its reply was lost
		redact     bool
	}{
		{hundredths: 5785},
		{hundredths: 25706, readCut: (245 << 20) + 37, redact: true},
		{hundredths: 62819, lostAck: 39},
		{hundredths: 10 * 1024 * 100, readCut: tenGiB - (40 << 20) + 37, lostAck: 639, redact: true},
	} {
		size := (tc.hundredths*(1<<20) + 50) / 100
		name := fmt.Sprintf("%.2fMiB", float64(size)/(1<<20))
		if size == tenGiB {
			name = "10GiB"
		}
		t.Run(name, func(t *testing.T) {
			email := "relay@example.invalid"
			if size == tenGiB {
				if os.Getenv("GATEWAY_LOG_ARCHIVE_TEST_10GIB") != "1" {
					t.Skip("set GATEWAY_LOG_ARCHIVE_TEST_10GIB=1 to transfer 10 GiB over loopback")
				}
				// Same byte length as [REDACTED]: both the input and the verified
				// redacted output exercise the full 10 GiB boundary.
				email = "a@abc.test"
			}
			// Generate indexed JSON lines on demand, at most one MiB each. Each
			// row is distinct, so dropping or duplicating a whole row is visible.
			row := func(off int64) string {
				prefix := fmt.Sprintf(`{"index":%d,"email":"%s","signature":"`, off/rowBytes, email)
				const suffix = "\"}\n"
				n := min(int64(rowBytes), size-off) - int64(len(prefix)+len(suffix))
				return prefix + padding[:n] + suffix
			}
			var entries []zipEntry
			wantHash := sha256.New()
			var wantSize int64
			for off := int64(0); off < size; off += rowBytes {
				line := row(off)
				entries = append(entries, zipEntry{Name: EntryName(len(entries) + 1), Offset: off, USize: int64(len(line) - 1), CRC: crc32.ChecksumIEEE([]byte(line[:len(line)-1]))})
				if tc.redact {
					line = strings.Replace(line, email, piiMask, 1)
				}
				wantSize += int64(len(line))
				_, _ = io.WriteString(wantHash, line)
			}

			var mu sync.Mutex
			var ranges [][3]int64 // start, inclusive end, negotiated HTTP major
			var starts, completions int
			var received int64
			gotHash := sha256.New()
			tags := make(map[int]string)
			attempts := make(map[int][]int)
			server := newRelayTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet && r.URL.Path == "/primary/bundle.jsonl" {
					var start, end int64
					_, err := fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
					if err != nil || start < 0 || end < start || end >= size || end-start+1 > relayRangeBytes {
						http.Error(w, "unexpected range", http.StatusRequestedRangeNotSatisfiable)
						return
					}
					mu.Lock()
					ranges = append(ranges, [3]int64{start, end, int64(r.ProtoMajor)})
					mu.Unlock()
					w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
					w.WriteHeader(http.StatusPartialContent)
					for pos := start; pos <= end; {
						inRow := pos % rowBytes
						line := row(pos - inRow)
						n := min(end-pos+1, int64(len(line))-inRow)
						if _, err := io.WriteString(w, line[inRow:inRow+n]); err != nil {
							return
						}
						pos += n
					}
					return
				}
				mu.Lock()
				defer mu.Unlock()
				q := r.URL.Query()
				switch {
				case r.Method == http.MethodPost && q.Has("uploads"):
					starts++
					fmt.Fprint(w, `<InitiateMultipartUploadResult><UploadId>upload-1</UploadId></InitiateMultipartUploadResult>`)
				case r.Method == http.MethodPut && q.Get("uploadId") == "upload-1":
					part, err := strconv.Atoi(q.Get("partNumber"))
					if !assert.NoError(t, err) {
						http.Error(w, "missing part number", 400)
						return
					}
					attempts[part] = append(attempts[part], r.ProtoMajor)
					partHash := sha256.New()
					var sink io.Writer = partHash
					if _, seen := tags[part]; !seen {
						assert.Equal(t, len(tags)+1, part)
						sink = io.MultiWriter(partHash, gotHash)
						received += r.ContentLength
					}
					n, err := io.Copy(sink, r.Body)
					assert.NoError(t, err)
					assert.Equal(t, r.ContentLength, n)
					tag := fmt.Sprintf(`"%x"`, partHash.Sum(nil))
					if prev, seen := tags[part]; seen {
						assert.Equal(t, prev, tag, "replayed part must be identical")
					}
					tags[part] = tag
					w.Header().Set("ETag", tag)
					if size == tenGiB && part%128 == 0 {
						t.Logf("destination received %d unique parts (%.1f GiB)", len(tags), float64(received)/(1<<30))
					}
				case r.Method == http.MethodPost && q.Get("uploadId") == "upload-1":
					var manifest struct {
						Parts []struct {
							PartNumber int
							ETag       string
						} `xml:"Part"`
					}
					assert.NoError(t, xml.NewDecoder(r.Body).Decode(&manifest))
					assert.Len(t, manifest.Parts, len(tags))
					for i, p := range manifest.Parts {
						assert.Equal(t, i+1, p.PartNumber)
						assert.Equal(t, tags[i+1], p.ETag)
					}
					completions++
					fmt.Fprint(w, `<CompleteMultipartUploadResult><ETag>complete</ETag></CompleteMultipartUploadResult>`)
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL.RequestURI())
					http.NotFound(w, r)
				}
			})

			var interruptedRead, lostPartAck atomic.Bool
			bucket := func(name string) objectStore {
				obj, err := newS3Objects(BucketConfig{Bucket: name, Endpoint: server.URL, ForcePathStyle: true, Region: "us-east-1", AccessKeyID: "test", SecretAccessKey: "test"})
				require.NoError(t, err)
				s := obj.(*s3Objects)
				opts := s.client.Options()
				transport := opts.HTTPClient.(relayHTTPClient)
				auto := transport.auto
				transport.auto = relayTestDo(func(r *http.Request) (*http.Response, error) {
					resp, err := auto.Do(r)
					if err != nil || resp.StatusCode/100 != 2 || resp.ProtoMajor != 2 {
						return resp, err
					}
					if tc.readCut > 0 && r.Method == http.MethodGet {
						var start, end int64
						_, _ = fmt.Sscanf(r.Header.Get("Range"), "bytes=%d-%d", &start, &end)
						if start < tc.readCut && tc.readCut <= end && interruptedRead.CompareAndSwap(false, true) {
							body := resp.Body
							resp.Body = struct {
								io.Reader
								io.Closer
							}{io.MultiReader(io.LimitReader(body, tc.readCut-start), iotest.ErrReader(errors.New(peerReset))), body}
						}
					}
					if tc.lostAck > 0 && r.Method == http.MethodPut && r.URL.Query().Get("partNumber") == strconv.Itoa(tc.lostAck) && lostPartAck.CompareAndSwap(false, true) {
						_ = resp.Body.Close()
						return nil, errors.New(peerReset)
					}
					return resp, nil
				})
				opts.HTTPClient = transport
				s.client = s3.New(opts)
				return s
			}
			ctx, cancel := context.WithTimeout(context.Background(), 9*time.Minute)
			defer cancel()
			copied, err := transferRelay(ctx, bucket("primary"), bucket("destination"), Bundle{ObjectKey: "bundle.jsonl", SizeBytes: size}, "copy.jsonl", entries, tc.redact, nil)
			require.NoError(t, err)
			mu.Lock()
			defer mu.Unlock()
			require.Equal(t, tc.readCut > 0, interruptedRead.Load())
			require.Equal(t, tc.lostAck > 0, lostPartAck.Load())
			require.Equal(t, 1, starts)
			require.Equal(t, 1, completions)
			require.Equal(t, wantSize, copied)
			require.Equal(t, wantSize, received)
			require.Equal(t, wantHash.Sum(nil), gotHash.Sum(nil))
			if size == tenGiB {
				require.Equal(t, tenGiB, received)
			}
			parts := int((wantSize + streamMinPart - 1) / streamMinPart)
			require.Len(t, tags, parts)
			for i := 1; i <= parts; i++ {
				want := []int{2}
				if tc.lostAck > 0 && i >= tc.lostAck {
					want = []int{1}
					if i == tc.lostAck {
						want = []int{2, 1}
					}
				}
				require.Equal(t, want, attempts[i], "attempts for part %d", i)
			}
			var next int64
			for _, get := range ranges {
				require.Equal(t, next, get[0], "resume from the first unread byte")
				proto := int64(2)
				if tc.readCut > 0 && next >= tc.readCut {
					proto = 1
				}
				require.Equal(t, proto, get[2])
				if next < tc.readCut && tc.readCut <= get[1] {
					next = tc.readCut
				} else {
					next = get[1] + 1
				}
			}
			require.Equal(t, size, next)
			t.Logf("%.2f MiB: %d source ranges, %d destination parts, one completed object, SHA-256 matched", float64(size)/(1<<20), len(ranges), parts)
		})
	}
}
