package gatewaylogarchive

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"hash/crc32"
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
	path     string
	fileName string
	entries  []zipEntry
	jsonl    bool
	redact   bool
	sizeHint int64
}

var errUploadSource = errors.New("读取或转换本地归档失败")

func (p *pipeline) prepareUpload(row *Bundle) (*preparedUpload, error) {
	info, err := os.Stat(row.LocalPath)
	if err != nil {
		return nil, fmt.Errorf("%w：%v", errLocalBundleMissing, err)
	}
	removeLegacyUploadTemp(row.LocalPath)
	cfg := p.cfg.Load()
	prepared := &preparedUpload{path: row.LocalPath, fileName: BundleFileName(row.ModelKey, row.Day, row.Part),
		jsonl: strings.HasSuffix(row.FileName, ".jsonl"), redact: cfg != nil && cfg.RedactPII}
	if prepared.jsonl && info.Size() != row.SizeBytes {
		return nil, errors.New("本地 JSONL 大小与已封存的索引不符")
	}
	if prepared.jsonl && !prepared.redact {
		return prepared, nil
	}
	entries, err := p.store.BundleEntries(p.ctx, row.ID)
	if err != nil {
		return nil, err
	}
	if len(entries) != row.EntryCount || len(entries) == 0 {
		return nil, errors.New("归档条目索引不完整")
	}
	for _, entry := range entries {
		if entry.Offset < 0 || entry.Offset > info.Size() || entry.USize < 0 || entry.CSize < 0 || entry.CSize > info.Size()-entry.Offset || (prepared.jsonl && entry.USize != entry.CSize) {
			return nil, fmt.Errorf("归档条目 %s 的索引范围无效", entry.Name)
		}
		prepared.sizeHint += entry.USize + 1
	}
	prepared.entries = entries
	return prepared, nil
}

func removeLegacyUploadTemp(source string) {
	path := source + ".upload-tmp"
	if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
		_ = os.Remove(path)
	}
}

func (u *preparedUpload) upload(ctx context.Context, objects objectStore, key string, progress func(int64)) (*uploadLayout, error) {
	if u.entries == nil {
		return nil, objects.Upload(ctx, key, u.path, progress)
	}
	var layout *uploadLayout
	err := objects.UploadStream(ctx, key, u.sizeHint, func(dst io.Writer) error {
		var err error
		layout, err = u.writeJSONL(ctx, dst)
		if err != nil {
			return fmt.Errorf("%w：%w", errUploadSource, err)
		}
		return nil
	}, progress)
	if err != nil {
		return nil, err
	}
	return layout, nil
}

func (u *preparedUpload) writeJSONL(ctx context.Context, dst io.Writer) (*uploadLayout, error) {
	source, err := os.Open(u.path)
	if err != nil {
		return nil, err
	}
	defer source.Close()
	layout := &uploadLayout{FileName: u.fileName, Entries: make([]zipEntry, 0, len(u.entries))}
	buffer := bufio.NewWriterSize(io.Discard, 64<<10)
	for _, entry := range u.entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		checksum := crc32.NewIEEE()
		count := &countWriter{w: io.MultiWriter(dst, checksum)}
		buffer.Reset(count)
		if err := u.writeEntry(ctx, source, entry, buffer); err != nil {
			return nil, fmt.Errorf("准备上传条目 %s 失败：%w", entry.Name, err)
		}
		if err := buffer.Flush(); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(dst, "\n"); err != nil {
			return nil, err
		}
		entry.Offset, entry.CSize, entry.USize, entry.CRC = layout.Size, count.n, count.n, checksum.Sum32()
		layout.Entries = append(layout.Entries, entry)
		layout.Size += count.n + 1
	}
	return layout, ctx.Err()
}

func (u *preparedUpload) writeEntry(ctx context.Context, source *os.File, entry zipEntry, dst io.Writer) error {
	var input io.Reader = io.NewSectionReader(source, entry.Offset, entry.USize)
	if !u.jsonl {
		legacy, err := readEntry(io.NewSectionReader(source, entry.Offset, 30+int64(len(entry.Name))+1024+entry.CSize), entry.CSize)
		if err != nil {
			return err
		}
		defer legacy.Close()
		input = legacy
	}
	if u.redact {
		return redactJSON(ctx, dst, input, true)
	}
	return compactJSON(dst, input)
}

func archiveContentType(key string) string {
	if strings.EqualFold(filepath.Ext(key), ".jsonl") {
		return "application/x-ndjson; charset=utf-8"
	}
	return "application/zip" // historical objects only
}
