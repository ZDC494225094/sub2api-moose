package gatewaylogarchive

import (
	"bufio"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"time"
)

// archiveAppender is also implemented by the legacy ZIP writer, solely to
// finish open ZIPs during upgrades. New bundles always use jsonlAppender.
type archiveAppender interface {
	Size() int64
	Count() int
	Append(string, time.Time, io.Reader) (zipEntry, error)
	Sync() error
	Seal() (int64, error)
	Close() error
	RemoveLast()
}

// JSONL indexes reuse the existing offset/size columns. Offset points at the
// JSON, both sizes exclude the terminating LF, and the bundle size includes it.
type jsonlAppender struct {
	f          *os.File
	size       int64
	count      int
	lastOffset int64
}

func createJSONL(path string) (*jsonlAppender, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	return &jsonlAppender{f: f}, nil
}

func reopenJSONL(path string, size int64, count int) (*jsonlAppender, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil && info.Size() < size {
		err = errors.New("JSONL file is shorter than its committed size")
	}
	if err == nil {
		err = f.Truncate(size)
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &jsonlAppender{f: f, size: size, count: count}, nil
}

func (j *jsonlAppender) Size() int64  { return j.size }
func (j *jsonlAppender) Count() int   { return j.count }
func (j *jsonlAppender) Close() error { return j.f.Close() }
func (j *jsonlAppender) Sync() error  { return j.f.Sync() }
func (j *jsonlAppender) Seal() (int64, error) {
	if err := j.Sync(); err != nil {
		return 0, err
	}
	return j.size, j.Close()
}

func (j *jsonlAppender) Append(name string, mod time.Time, src io.Reader) (zipEntry, error) {
	if _, err := j.f.Seek(j.size, io.SeekStart); err != nil {
		return zipEntry{}, err
	}
	w := bufio.NewWriterSize(j.f, 1<<20)
	crc := crc32.NewIEEE()
	count := &countWriter{w: io.MultiWriter(w, crc)}
	err := compactJSON(count, src)
	if err == nil {
		err = w.WriteByte('\n')
	}
	if err == nil {
		err = w.Flush()
	}
	if err != nil {
		_ = j.f.Truncate(j.size)
		return zipEntry{}, err
	}
	e := zipEntry{Name: name, Offset: j.size, CSize: count.n, USize: count.n, CRC: crc.Sum32(), Mod: mod}
	j.lastOffset, j.size, j.count = j.size, j.size+count.n+1, j.count+1
	return e, nil
}

func (j *jsonlAppender) RemoveLast() {
	if j.count == 0 {
		return
	}
	_ = j.f.Truncate(j.lastOffset)
	j.size, j.count = j.lastOffset, j.count-1
}

// compactJSON removes only insignificant JSON whitespace. It streams large
// session documents, preserving string escapes, number spelling and key order.
// Its input is a valid session document produced by openDocument.
func compactJSON(w io.Writer, r io.Reader) error {
	buf := make([]byte, 64<<10)
	inString, escaped := false, false
	for {
		n, err := r.Read(buf)
		out := buf[:0]
		for _, c := range buf[:n] {
			if inString {
				out = append(out, c)
				if escaped {
					escaped = false
				} else if c == '\\' {
					escaped = true
				} else if c == '"' {
					inString = false
				}
			} else {
				if c == '"' {
					inString = true
				}
				if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
					out = append(out, c)
				}
			}
		}
		if _, werr := w.Write(out); werr != nil {
			return werr
		}
		if err == io.EOF {
			if inString {
				return errors.New("unterminated JSON string")
			}
			return nil
		}
		if err != nil {
			return err
		}
	}
}
