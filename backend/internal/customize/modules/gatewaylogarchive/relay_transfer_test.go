package gatewaylogarchive

import (
	"bytes"
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const peerReset = "stream error: stream ID 1; PROTOCOL_ERROR; received from peer"

type relayTestSource struct {
	*memObjects
	open func(context.Context, int64, int64) (io.ReadCloser, error)
}

func (s relayTestSource) OpenRange(ctx context.Context, _ string, offset, length int64) (io.ReadCloser, error) {
	return s.open(ctx, offset, length)
}

// Return data and the reset on the same Read. Retrying the original range
// or dropping those bytes would corrupt the line or its redacted output.
type lastReadError struct {
	data []byte
	err  error
}

func (r *lastReadError) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.err
	}
	return n, nil
}

func TestRelayResumesMidLineWithoutDuplicatingOrUnmaskingBytes(t *testing.T) {
	ctx := context.Background()
	lines := []string{`{"email":"alice@example.com","v":9007199254740993}`, `{"text":"second"}`}
	data := strings.Join(lines, "\n") + "\n"
	var entries []zipEntry
	var offset int64
	for i, line := range lines {
		entries = append(entries, zipEntry{Name: EntryName(i + 1), Offset: offset, USize: int64(len(line)), CRC: crc32.ChecksumIEEE([]byte(line))})
		offset += int64(len(line) + 1)
	}
	for _, redact := range []bool{false, true} {
		t.Run(fmt.Sprint(redact), func(t *testing.T) {
			const cut = 15
			var starts []int64
			source := relayTestSource{newMemObjects(), func(ctx context.Context, pos, length int64) (io.ReadCloser, error) {
				starts = append(starts, pos)
				require.Equal(t, int64(len(data)), pos+length)
				if pos == 0 {
					require.False(t, relayProtocolFor(ctx).useHTTP1())
					return io.NopCloser(&lastReadError{[]byte(data[:cut]), errors.New(peerReset)}), nil
				}
				require.True(t, relayProtocolFor(ctx).useHTTP1())
				return io.NopCloser(strings.NewReader(data[pos:])), nil
			}}
			dest := newMemObjects()
			size, err := transferRelay(ctx, source, dest, Bundle{SizeBytes: int64(len(data))}, "copy", entries, redact, nil)
			require.NoError(t, err)
			want := data
			if redact {
				want = strings.ReplaceAll(data, "alice@example.com", piiMask)
			}
			require.Equal(t, []int64{0, cut}, starts)
			require.Equal(t, int64(len(want)), size)
			require.Equal(t, want, string(dest.objects["copy"]))
		})
	}
}

func TestRelayBoundsEachRangeAndRetriesOnlyMissingSourceBytes(t *testing.T) {
	const size = int64(relayRangeBytes + 73)
	data := make([]byte, size)
	var reads [][2]int64
	source := relayTestSource{newMemObjects(), func(_ context.Context, pos, length int64) (io.ReadCloser, error) {
		reads = append(reads, [2]int64{pos, length})
		require.LessOrEqual(t, length, int64(relayRangeBytes))
		end := pos + length
		if len(reads) == 1 {
			end = 27
		} // premature EOF with no transport error
		return io.NopCloser(bytes.NewReader(data[pos:end])), nil
	}}
	r := &relayRangeReader{ctx: context.Background(), from: source, size: size}
	defer r.closeBody()
	n, err := io.Copy(io.Discard, r)
	require.NoError(t, err)
	require.Equal(t, size, n)
	require.Equal(t, [][2]int64{{0, relayRangeBytes}, {27, relayRangeBytes - 27}, {relayRangeBytes, 73}}, reads)
}

func TestRelayReportsWhichSideFailedAndNeverPublishesPartialCopy(t *testing.T) {
	const data = "{}\n"
	entry := zipEntry{Name: "item", USize: 2, CRC: crc32.ChecksumIEEE([]byte("{}"))}
	for _, scenario := range []string{"read", "write", "CRC", "denied", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reads := 0
			source := relayTestSource{newMemObjects(), func(ctx context.Context, pos, length int64) (io.ReadCloser, error) {
				reads++
				switch scenario {
				case "read":
					return nil, errors.New(peerReset)
				case "denied":
					return nil, errors.New("AccessDenied")
				case "canceled":
					cancel()
					return nil, errors.New(peerReset)
				}
				return io.NopCloser(strings.NewReader(data)), nil
			}}
			dest := newMemObjects()
			if scenario == "write" {
				dest.fail = errors.New(peerReset)
			}
			meta := entry
			if scenario == "CRC" {
				meta.CRC++
			}
			_, err := transferRelay(ctx, source, dest, Bundle{SizeBytes: int64(len(data))}, "copy", []zipEntry{meta}, false, nil)
			require.Error(t, err)
			require.Empty(t, dest.objects)
			switch scenario {
			case "read":
				require.Equal(t, 3, reads)
				require.ErrorContains(t, err, "读取主桶 test-bucket 字节 0/3")
			case "write":
				require.ErrorContains(t, err, "写入目标桶 test-bucket")
			case "CRC":
				require.ErrorContains(t, err, "校验或转换主归档条目 item")
			case "denied":
				require.Equal(t, 1, reads)
				require.ErrorContains(t, err, "AccessDenied")
			case "canceled":
				require.Equal(t, 1, reads)
				require.ErrorIs(t, err, context.Canceled)
			}
		})
	}
}

func newRelayTLSServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewUnstartedServer(handler)
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)
	cert := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600))
	t.Setenv("AWS_CA_BUNDLE", cert)
	return server
}

// Exercise real TLS/ALPN and the SDK, including its configured trust roots.
// Changing the protocol for one relay retry must not change the shared client.
func TestRelayHTTP1IsScopedAndRejectsIgnoredRanges(t *testing.T) {
	var protos = make(chan int, 5)
	server := newRelayTLSServer(t, func(w http.ResponseWriter, r *http.Request) {
		protos <- r.ProtoMajor
		if r.URL.Path != "/bucket/ignored" {
			assert.Equal(t, "bytes=1-1", r.Header.Get("Range"))
			w.Header().Set("Content-Range", "bytes 1-1/3")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, "b")
			return
		}
		_, _ = io.WriteString(w, "abc")
	})
	s, err := newS3Objects(BucketConfig{Bucket: "bucket", Region: "us-east-1", Endpoint: server.URL, ForcePathStyle: true, AccessKeyID: "test", SecretAccessKey: "test"})
	require.NoError(t, err)
	ctx := context.Background()
	for i, c := range []context.Context{ctx, withRelayProtocol(ctx, true), ctx} {
		r, err := s.OpenRange(c, "valid", 1, 1)
		require.NoError(t, err)
		b, err := io.ReadAll(r)
		_ = r.Close()
		require.NoError(t, err)
		require.Equal(t, "b", string(b))
		require.Equal(t, []int{2, 1, 2}[i], <-protos)
	}
	r, err := s.OpenRange(ctx, "ignored", 1, 1)
	require.Nil(t, r)
	require.ErrorContains(t, err, "未返回请求的 Range")
}
