package gatewaylogarchive

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

const (
	uploadIdleTimeout    = 2 * time.Minute
	uploadControlTimeout = time.Minute
	uploadCommitTimeout  = 5 * time.Minute
)

var errUploadIdle = fmt.Errorf("连续 %s 无发送进展且未收到存储桶确认：%w", uploadIdleTimeout, context.DeadlineExceeded)

type uploadProgressKey struct{}
type uploadWatchKey struct{}

func progressFor(ctx context.Context) *uploadProgress {
	p, _ := ctx.Value(uploadProgressKey{}).(*uploadProgress)
	return p
}

func (p *uploadProgress) setPhase(phase string) {
	if p != nil {
		p.phase.Store(&phase)
	}
}

// Watch a single PUT (or a single attempt at one UploadPart), never an entire
// archive. Track reads by the HTTP transport, after signing/SDK buffering. In
// particular a missing response after the last body byte still times out.
type uploadWatch struct {
	start    time.Time
	last     atomic.Int64 // monotonic time since start
	progress *uploadProgress
}

func watchUploadRequest(parent context.Context, send func(context.Context) error) error {
	w := &uploadWatch{start: time.Now(), progress: progressFor(parent)}
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	go func() {
		timer := time.NewTimer(uploadIdleTimeout)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				remaining := uploadIdleTimeout - (time.Since(w.start) - time.Duration(w.last.Load()))
				if remaining <= 0 {
					cancel(errUploadIdle)
					return
				}
				timer.Reset(remaining)
			}
		}
	}()
	err := send(context.WithValue(ctx, uploadWatchKey{}, w))
	if err != nil && context.Cause(ctx) == errUploadIdle {
		return errUploadIdle
	}
	return err
}

type watchedBody struct {
	io.ReadCloser
	w *uploadWatch
}

func (b watchedBody) Read(buf []byte) (int, error) {
	n, err := b.ReadCloser.Read(buf)
	if n > 0 {
		b.w.last.Store(int64(time.Since(b.w.start)))
		if p := b.w.progress; p != nil {
			p.bodyBytes.Add(int64(n))
		}
	}
	return n, err
}

func watchHTTPBody(req *http.Request) *http.Request {
	w, _ := req.Context().Value(uploadWatchKey{}).(*uploadWatch)
	if w == nil || req.Body == nil || req.Body == http.NoBody {
		return req
	}
	r := req.Clone(req.Context())
	r.Body = watchedBody{r.Body, w}
	if getBody := r.GetBody; getBody != nil {
		r.GetBody = func() (io.ReadCloser, error) {
			b, err := getBody()
			if err != nil {
				return nil, err
			}
			return watchedBody{b, w}, nil
		}
	}
	return r
}
