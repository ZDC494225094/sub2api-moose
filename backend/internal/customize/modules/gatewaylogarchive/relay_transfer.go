package gatewaylogarchive

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/aws"
)

const (
	relayRangeBytes   = 32 << 20
	relayRangeRetries = 2
	relayHTTP1Retry   = "HTTP/1.1 转存重试"
)

type relayProtocolKey struct{}

// A preference belongs to one side of one transfer, never to the cached S3
// client. A retry on a different worker can start with HTTP/1.1 already set.
type relayProtocol struct{ http1 atomic.Bool }

func relayProtocolFor(ctx context.Context) *relayProtocol {
	p, _ := ctx.Value(relayProtocolKey{}).(*relayProtocol)
	return p
}

func withRelayProtocol(ctx context.Context, http1 bool) context.Context {
	p := &relayProtocol{}
	p.http1.Store(http1)
	return context.WithValue(ctx, relayProtocolKey{}, p)
}

func uploadProtocolContext(ctx context.Context) context.Context {
	if relayProtocolFor(ctx) != nil {
		return ctx
	}
	return withRelayProtocol(ctx, false)
}

func (p *relayProtocol) useHTTP1() bool { return p != nil && p.http1.Load() }

func (p *relayProtocol) observe(err error) {
	if p != nil && err != nil && (errors.Is(err, errUploadIdle) || relayProtocolError(err.Error())) {
		p.http1.Store(true)
	}
}

// Go's net/http stream error has an internal type. Old workers also saved
// just this text in last_error, so match the transport signature, not every
// S3 error containing the word "protocol".
func relayProtocolError(s string) bool {
	return strings.Contains(s, "PROTOCOL_ERROR") && (strings.Contains(s, "stream error: stream ID ") || strings.Contains(s, "http2:"))
}

func retryRelayHTTP1(lastError string) bool {
	return strings.HasPrefix(lastError, relayHTTP1Retry) || relayProtocolError(lastError)
}

type relayHTTPClient struct{ auto, http1 aws.HTTPClient }

func (c relayHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req = watchHTTPBody(req)
	p := relayProtocolFor(req.Context())
	if p.useHTTP1() {
		return c.http1.Do(req)
	}
	resp, err := c.auto.Do(req)
	// Let the SDK or the buffered part uploader replay the request safely.
	// Reusing that part number and upload ID keeps earlier successful parts.
	p.observe(err)
	return resp, err
}

// Read bounded ranges from the immutable primary object. An interrupted
// response body is outside the SDK's request retry loop. Resume at the next
// undelivered byte without restarting the JSON parser or the multipart PUT.
type relayRangeReader struct {
	ctx            context.Context
	from           objectStore
	key            string
	size, pos, end int64
	body           io.ReadCloser
	retries        int // per range, including partial reads
	err            error
}

func (r *relayRangeReader) closeBody() {
	if r.body != nil {
		_ = r.body.Close()
		r.body = nil
	}
}

func (r *relayRangeReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if err := r.ctx.Err(); err != nil {
			return 0, err
		}
		if r.err != nil {
			return 0, r.err
		}
		if r.pos == r.size {
			return 0, io.EOF
		}
		if r.pos == r.end {
			r.end = r.pos + min(int64(relayRangeBytes), r.size-r.pos)
			r.retries = 0
		}
		var n int
		var err error
		if r.body == nil {
			r.body, err = r.from.OpenRange(r.ctx, r.key, r.pos, r.end-r.pos)
		}
		if err == nil {
			n, err = r.body.Read(p[:min(int64(len(p)), r.end-r.pos)])
			r.pos += int64(n)
			switch {
			case err == io.EOF && r.pos < r.end:
				err = io.ErrUnexpectedEOF
			case err == io.EOF:
				err = nil
			case err == nil && n == 0:
				err = io.ErrNoProgress
			}
		}
		if err != nil || r.pos == r.end {
			r.closeBody()
		}
		if err == nil {
			return n, nil
		}
		h2 := relayProtocolError(err.Error())
		var netErr net.Error
		transient := h2 || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) || (errors.As(err, &netErr) && netErr.Timeout())
		if h2 && !relayProtocolFor(r.ctx).useHTTP1() {
			r.ctx = withRelayProtocol(r.ctx, true)
		}
		// If a reset accompanies the last bytes of this range there are no
		// missing bytes. The caller still verifies every line's length and CRC.
		if transient && r.pos == r.end {
			return n, nil
		}
		if !transient || r.retries >= relayRangeRetries {
			r.err = fmt.Errorf("读取主桶 %s 字节 %d/%d：%w", r.from.Bucket(), r.pos, r.size, err)
			return n, r.err
		}
		r.retries++
		// Deliver a partial Read's bytes exactly once before reopening.
		if n > 0 {
			return n, nil
		}
	}
}

// Distinguish errors from the destination writer inside the transform from a
// CRC or JSON error. UploadStream can also fail after the producer finishes.
type relayWriter struct {
	io.Writer
	err error
}

func (w *relayWriter) Write(p []byte) (int, error) {
	n, err := w.Writer.Write(p)
	if err != nil {
		w.err = err
	}
	return n, err
}

func transferRelay(ctx context.Context, from, to objectStore, row Bundle, key string, entries []zipEntry, redact bool) (int64, error) {
	http1 := relayProtocolFor(ctx).useHTTP1()
	r := &relayRangeReader{ctx: withRelayProtocol(ctx, http1), from: from, key: row.ObjectKey, size: row.SizeBytes}
	defer r.closeBody()
	var size int64
	var contentErr error
	err := to.UploadStream(withRelayProtocol(ctx, http1), key, row.SizeBytes, func(out io.Writer) error {
		reader := bufio.NewReaderSize(r, 64<<10)
		w := &relayWriter{Writer: out}
		var err error
		size, err = writeRelayLines(ctx, reader, w, row.SizeBytes, entries, redact)
		// bufio can consume all requested bytes before its trailing error.
		if r.err != nil {
			return r.err
		}
		if w.err == nil {
			contentErr = err
		}
		return err
	}, nil)
	switch {
	case err == nil:
		return size, nil
	case ctx.Err() != nil:
		return 0, ctx.Err()
	case r.err != nil:
		return 0, r.err
	case contentErr != nil:
		return 0, contentErr
	default:
		return 0, fmt.Errorf("写入目标桶 %s：%w", to.Bucket(), err)
	}
}

func writeRelayLines(ctx context.Context, reader *bufio.Reader, out io.Writer, sourceSize int64, entries []zipEntry, redact bool) (int64, error) {
	var offset, size int64
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if entry.Offset != offset || entry.USize < 0 || entry.USize >= sourceSize-offset {
			return 0, errors.New("主归档索引偏移或长度不符")
		}
		line, err := writeIndexedLine(out, func(dst io.Writer) error { return copyVerified(ctx, dst, reader, entry, redact) })
		if err != nil {
			return 0, fmt.Errorf("校验或转换主归档条目 %s：%w", entry.Name, err)
		}
		lf, err := reader.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("读取主归档条目 %s 的换行：%w", entry.Name, err)
		}
		if lf != '\n' {
			return 0, errors.New("主归档缺少 JSONL 换行")
		}
		offset += entry.USize + 1
		size += line.USize + 1
	}
	if offset != sourceSize {
		return 0, errors.New("主归档大小不符")
	}
	return size, nil
}
