//go:build integration

package gatewaylogarchive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This test needs space for ONE real 18.35 GiB JSONL. The TLS S3 receiver
// hashes each part as it arrives rather than storing a second file.
func TestLocalArchive18GiB(t *testing.T) {
	if os.Getenv("GATEWAY_LOG_ARCHIVE_TEST_18GIB") != "1" {
		t.Skip("set GATEWAY_LOG_ARCHIVE_TEST_18GIB=1 with 19 GiB free in the temp directory")
	}
	const size = int64((1835*(1<<30) + 50) / 100)
	partSize := partSizeFor(size, minPartSize, maxParts)
	source := filepath.Join(t.TempDir(), "old.jsonl")
	f, err := os.Create(source)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	chunk := bytes.Repeat([]byte{' '}, int(partSize))
	var tags []string
	for off := int64(0); off < size; off += partSize {
		line := chunk[:min(partSize, size-off)]
		copy(line, fmt.Sprintf(`{"part":"%04d"}`, len(tags)+1))
		line[len(line)-1] = '\n'
		_, err := f.Write(line)
		require.NoError(t, err)
		tags = append(tags, fmt.Sprintf(`"%x"`, sha256.Sum256(line)))
	}
	require.NoError(t, f.Close())
	var starts, completes atomic.Int32
	var received, confirmed atomic.Int64
	var mu sync.Mutex
	seen := make(map[int]bool)
	server := newRelayTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch {
		case r.Method == "POST" && q.Has("uploads"):
			starts.Add(1)
			fmt.Fprint(w, `<InitiateMultipartUploadResult><UploadId>U</UploadId></InitiateMultipartUploadResult>`)
		case r.Method == "PUT" && q.Get("uploadId") == "U":
			num, _ := strconv.Atoi(q.Get("partNumber"))
			if num < 1 || num > len(tags) {
				http.Error(w, "invalid part", 400)
				return
			}
			h := sha256.New()
			n, err := io.Copy(h, r.Body)
			if err != nil || fmt.Sprintf(`"%x"`, h.Sum(nil)) != tags[num-1] || n != r.ContentLength {
				http.Error(w, "incorrect part", 400)
				return
			}
			mu.Lock()
			if !seen[num] {
				received.Add(n)
				seen[num] = true
			}
			mu.Unlock()
			w.Header().Set("ETag", tags[num-1])
		case r.Method == "POST" && q.Get("uploadId") == "U":
			var manifest struct {
				Parts []struct {
					Number int `xml:"PartNumber"`
					ETag   string
				} `xml:"Part"`
			}
			if err := xml.NewDecoder(r.Body).Decode(&manifest); err != nil || len(manifest.Parts) != len(tags) {
				http.Error(w, "incomplete manifest", 400)
				return
			}
			for i, part := range manifest.Parts {
				assert.Equal(t, i+1, part.Number)
				assert.Equal(t, tags[i], part.ETag)
			}
			completes.Add(1)
			fmt.Fprint(w, `<CompleteMultipartUploadResult><ETag>done</ETag></CompleteMultipartUploadResult>`)
		default:
			http.Error(w, "unexpected S3 operation", 400)
		}
	})
	s, err := newS3Objects(BucketConfig{Bucket: "primary", Region: "us-east-1", Endpoint: server.URL, ForcePathStyle: true, AccessKeyID: "test", SecretAccessKey: "test"})
	require.NoError(t, err)
	p := &uploadProgress{}
	ctx := context.WithValue(t.Context(), uploadProgressKey{}, p)
	require.NoError(t, s.Upload(ctx, "old.jsonl", source, func(n int64) { confirmed.Add(n) }))
	require.Equal(t, size, confirmed.Load())
	require.Equal(t, size, received.Load())
	require.GreaterOrEqual(t, p.bodyBytes.Load(), size)
	require.Equal(t, int32(1), starts.Load())
	require.Equal(t, int32(1), completes.Load())
	t.Logf("18.35 GiB: %d bytes, %d parts verified by SHA-256 and ordered manifest, one completed object", size, len(tags))
}
