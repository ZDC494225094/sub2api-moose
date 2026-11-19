package gatewaylogarchive

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

const (
	streamMinPart = 16 << 20
	streamMaxPart = 256 << 20
	s3PartLimit   = 10000
)

type streamPartWriter struct {
	store    *s3Objects
	ctx      context.Context
	key      string
	progress func(int64)
	limit    int
	buffer   []byte
	uploadID *string
	parts    []types.CompletedPart
	err      error
}

func (s *s3Objects) UploadStream(ctx context.Context, key string, sizeHint int64, produce func(io.Writer) error, progress func(int64)) error {
	if sizeHint < 0 {
		return errors.New("归档预计大小无效")
	}
	limit := min(partSizeFor(sizeHint, streamMinPart, maxParts/2), streamMaxPart)
	stream := &streamPartWriter{store: s, ctx: ctx, key: key, progress: progress, limit: int(limit)}
	stream.buffer = make([]byte, 0, int(min(sizeHint, limit)))
	defer func() {
		if stream.uploadID != nil {
			s.abortMultipart(key, stream.uploadID)
		}
	}()
	err := produce(stream)
	if stream.err != nil {
		return stream.err
	}
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if stream.uploadID == nil {
		return s.put(ctx, key, bytes.NewReader(stream.buffer), int64(len(stream.buffer)), progress)
	}
	if err := stream.flushPart(); err != nil {
		return err
	}
	if err := s.completeMultipart(ctx, key, stream.uploadID, stream.parts); err != nil {
		return err
	}
	stream.uploadID = nil
	return nil
}

func (w *streamPartWriter) Write(data []byte) (int, error) {
	written := 0
	for len(data) > 0 && w.err == nil {
		if w.err = w.ctx.Err(); w.err != nil {
			break
		}
		if len(w.buffer) == w.limit {
			if w.uploadID == nil {
				w.uploadID, w.err = w.store.startMultipart(w.ctx, w.key)
				if w.err != nil {
					break
				}
			}
			if w.err = w.flushPart(); w.err != nil {
				break
			}
		}
		count := min(w.limit-len(w.buffer), len(data))
		if needed := len(w.buffer) + count; needed > cap(w.buffer) {
			grown := make([]byte, len(w.buffer), min(w.limit, max(needed, 2*cap(w.buffer), 4096)))
			copy(grown, w.buffer)
			w.buffer = grown
		}
		w.buffer = append(w.buffer, data[:count]...)
		written += count
		data = data[count:]
	}
	return written, w.err
}

func (w *streamPartWriter) flushPart() error {
	if len(w.parts) >= s3PartLimit {
		return errors.New("上传超过存储桶的 10000 个分片限制")
	}
	number := int32(len(w.parts) + 1)
	size := int64(len(w.buffer))
	part, err := w.store.sendPart(w.ctx, w.key, w.uploadID, number, bytes.NewReader(w.buffer), size)
	if err != nil {
		return fmt.Errorf("part %d: %w", number, err)
	}
	w.parts = append(w.parts, part)
	w.buffer = w.buffer[:0]
	if w.progress != nil {
		w.progress(size)
	}
	return nil
}
