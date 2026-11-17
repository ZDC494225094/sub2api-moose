package gatewaylogarchive

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// uploadLayout describes the uploaded representation. It is committed together
// with the destination and statistics after a successful upload. Local indexes
// remain valid for the original spool until that transaction commits.
type uploadLayout struct {
	FileName string
	Size     int64
	Entries  []zipEntry
}

type preparedUpload struct {
	path    string
	layout  *uploadLayout
	cleanup func()
}

func (p *pipeline) prepareUpload(row *Bundle) (*preparedUpload, error) {
	if _, err := os.Stat(row.LocalPath); err != nil {
		return nil, fmt.Errorf("%w：%v", errLocalBundleMissing, err)
	}
	cfg := p.cfg.Load()
	redact := cfg != nil && cfg.RedactPII
	jsonl := strings.HasSuffix(row.FileName, ".jsonl")
	if jsonl && !redact {
		return &preparedUpload{path: row.LocalPath, cleanup: func() { _ = os.Remove(row.LocalPath + ".upload-tmp") }}, nil
	}
	entries, err := p.store.BundleEntries(p.ctx, row.ID)
	if err != nil {
		return nil, err
	}
	if len(entries) != row.EntryCount || len(entries) == 0 {
		return nil, fmt.Errorf("归档条目索引不完整")
	}
	source, err := os.Open(row.LocalPath)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	// A deterministic node-owned scratch name is truncated on each attempt;
	// interrupted uploads cannot leave an ever-growing collection of temp files.
	path := row.LocalPath + ".upload-tmp"
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	j := &jsonlAppender{f: f}
	ok := false
	defer func() {
		_ = j.Close()
		if !ok {
			_ = os.Remove(path)
		}
	}()
	layout := &uploadLayout{FileName: BundleFileName(row.ModelKey, row.Day, row.Part)}
	for _, e := range entries {
		if err := p.ctx.Err(); err != nil {
			return nil, err
		}
		var input io.Reader = io.NewSectionReader(source, e.Offset, e.USize)
		var legacy io.ReadCloser
		if !jsonl {
			legacy, err = readEntry(io.NewSectionReader(source, e.Offset, 30+int64(len(e.Name))+1024+e.CSize), e.CSize)
			if err != nil {
				return nil, err
			}
			input = legacy
		}
		var transformed io.ReadCloser
		if redact {
			transformed = redactedReader(p.ctx, input)
			input = transformed
		}
		indexed, appendErr := j.Append(e.Name, e.Mod, input)
		if transformed != nil {
			_ = transformed.Close()
		}
		if legacy != nil {
			_ = legacy.Close()
		}
		if appendErr != nil {
			return nil, fmt.Errorf("准备上传条目 %s 失败：%w", e.Name, appendErr)
		}
		layout.Entries = append(layout.Entries, indexed)
	}
	layout.Size, err = j.Seal()
	if err != nil {
		return nil, err
	}
	ok = true
	return &preparedUpload{path: path, layout: layout, cleanup: func() { _ = os.Remove(path) }}, nil
}

// Close waits for the transform goroutine, so retries cannot accumulate
// readers when disk writes, cancellation or JSON parsing fail.
type transformReader struct {
	*io.PipeReader
	done <-chan struct{}
}

func (r *transformReader) Close() error {
	err := r.PipeReader.Close()
	<-r.done
	return err
}
func redactedReader(ctx context.Context, input io.Reader) io.ReadCloser {
	r, w := io.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buffer := bufio.NewWriterSize(w, 64<<10)
		err := redactJSON(ctx, buffer, input)
		if err == nil {
			err = buffer.Flush()
		}
		_ = w.CloseWithError(err)
	}()
	return &transformReader{PipeReader: r, done: done}
}

func archiveContentType(key string) string {
	if strings.EqualFold(filepath.Ext(key), ".jsonl") {
		return "application/x-ndjson; charset=utf-8"
	}
	return "application/zip" // historical objects only
}
