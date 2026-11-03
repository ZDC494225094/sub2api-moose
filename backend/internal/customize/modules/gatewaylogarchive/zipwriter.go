package gatewaylogarchive

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"time"

	"github.com/klauspost/compress/flate"
)

// zipEntry is everything the central directory needs; it is also persisted
// per entry so an open bundle can be rebuilt after a restart.
type zipEntry struct {
	Name   string
	Offset int64
	CRC    uint32
	CSize  int64
	USize  int64
	Mod    time.Time
}

const (
	zipFlagUTF8   = 1 << 11
	zipMethodFlat = 8
	max32         = 0xFFFFFFFF
	max16         = 0xFFFF
)

// zipAppender appends entries to a ZIP file that stays crash-recoverable: each
// local header is patched with its final CRC and sizes (no data descriptors),
// and the central directory is only written by Seal. Entries are limited to
// 4 GiB each; the archive itself may exceed 4 GiB (ZIP64 directory).
type zipAppender struct {
	f       *os.File
	size    int64
	entries []zipEntry
	// forceZip64 exercises the >4 GiB directory path in tests.
	forceZip64 bool
}

func createZip(path string) (*zipAppender, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	return &zipAppender{f: f}, nil
}

// reopenZip continues an unsealed archive whose committed length and entries
// are known; anything past size (an interrupted append) is discarded.
func reopenZip(path string, size int64, entries []zipEntry) (*zipAppender, error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if info.Size() < size {
		_ = f.Close()
		return nil, errors.New("bundle file is shorter than its committed size")
	}
	if err := f.Truncate(size); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &zipAppender{f: f, size: size, entries: entries}, nil
}

func (z *zipAppender) Size() int64 { return z.size }
func (z *zipAppender) Count() int  { return len(z.entries) }

type countWriter struct {
	w io.Writer
	n int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func dosTime(t time.Time) (uint16, uint16) {
	t = t.UTC()
	if t.Year() < 1980 {
		t = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	date := uint16(t.Day() + int(t.Month())<<5 + (t.Year()-1980)<<9)
	clock := uint16(t.Second()/2 + t.Minute()<<5 + t.Hour()<<11)
	return date, clock
}

// Append compresses src as a new entry. On error the file is truncated back so
// the archive stays consistent; the caller may retry.
func (z *zipAppender) Append(name string, mod time.Time, src io.Reader) (zipEntry, error) {
	start := z.size
	entry, err := z.append(name, mod, src)
	if err != nil {
		_ = z.f.Truncate(start)
		z.size = start
		return zipEntry{}, err
	}
	z.entries = append(z.entries, entry)
	z.size = entry.Offset + 30 + int64(len(name)) + entry.CSize
	return entry, nil
}

func (z *zipAppender) append(name string, mod time.Time, src io.Reader) (zipEntry, error) {
	if len(name) > max16 {
		return zipEntry{}, errors.New("entry name too long")
	}
	if _, err := z.f.Seek(z.size, io.SeekStart); err != nil {
		return zipEntry{}, err
	}
	date, clock := dosTime(mod)
	header := make([]byte, 30+len(name))
	binary.LittleEndian.PutUint32(header[0:], 0x04034b50)
	binary.LittleEndian.PutUint16(header[4:], 20)
	binary.LittleEndian.PutUint16(header[6:], zipFlagUTF8)
	binary.LittleEndian.PutUint16(header[8:], zipMethodFlat)
	binary.LittleEndian.PutUint16(header[10:], clock)
	binary.LittleEndian.PutUint16(header[12:], date)
	binary.LittleEndian.PutUint16(header[26:], uint16(len(name)))
	copy(header[30:], name)
	buffered := bufio.NewWriterSize(z.f, 1<<20)
	if _, err := buffered.Write(header); err != nil {
		return zipEntry{}, err
	}
	counted := &countWriter{w: buffered}
	fw, err := flate.NewWriter(counted, 3)
	if err != nil {
		return zipEntry{}, err
	}
	crc := crc32.NewIEEE()
	usize, err := io.Copy(io.MultiWriter(fw, crc), src)
	if err != nil {
		return zipEntry{}, err
	}
	if err := fw.Close(); err != nil {
		return zipEntry{}, err
	}
	if err := buffered.Flush(); err != nil {
		return zipEntry{}, err
	}
	if usize >= max32 || counted.n >= max32 {
		return zipEntry{}, errors.New("entry exceeds 4 GiB")
	}
	patch := make([]byte, 12)
	binary.LittleEndian.PutUint32(patch[0:], crc.Sum32())
	binary.LittleEndian.PutUint32(patch[4:], uint32(counted.n))
	binary.LittleEndian.PutUint32(patch[8:], uint32(usize))
	if _, err := z.f.WriteAt(patch, z.size+14); err != nil {
		return zipEntry{}, err
	}
	return zipEntry{Name: name, Offset: z.size, CRC: crc.Sum32(), CSize: counted.n, USize: usize, Mod: mod.UTC().Truncate(time.Second)}, nil
}

// Sync makes appended entries durable before they are committed to the DB.
func (z *zipAppender) Sync() error { return z.f.Sync() }

// Seal writes the central directory (ZIP64 when needed) and closes the file.
func (z *zipAppender) Seal() (int64, error) {
	if _, err := z.f.Seek(z.size, io.SeekStart); err != nil {
		return 0, err
	}
	w := bufio.NewWriterSize(z.f, 1<<20)
	dirStart := z.size
	var dirSize int64
	for _, e := range z.entries {
		var extra []byte
		offset := uint32(e.Offset)
		version := uint16(20)
		if e.Offset >= max32 || z.forceZip64 {
			offset = max32
			version = 45
			extra = make([]byte, 12)
			binary.LittleEndian.PutUint16(extra[0:], 0x0001)
			binary.LittleEndian.PutUint16(extra[2:], 8)
			binary.LittleEndian.PutUint64(extra[4:], uint64(e.Offset))
		}
		date, clock := dosTime(e.Mod)
		rec := make([]byte, 46+len(e.Name)+len(extra))
		binary.LittleEndian.PutUint32(rec[0:], 0x02014b50)
		binary.LittleEndian.PutUint16(rec[4:], 3<<8|version)
		binary.LittleEndian.PutUint16(rec[6:], version)
		binary.LittleEndian.PutUint16(rec[8:], zipFlagUTF8)
		binary.LittleEndian.PutUint16(rec[10:], zipMethodFlat)
		binary.LittleEndian.PutUint16(rec[12:], clock)
		binary.LittleEndian.PutUint16(rec[14:], date)
		binary.LittleEndian.PutUint32(rec[16:], e.CRC)
		binary.LittleEndian.PutUint32(rec[20:], uint32(e.CSize))
		binary.LittleEndian.PutUint32(rec[24:], uint32(e.USize))
		binary.LittleEndian.PutUint16(rec[28:], uint16(len(e.Name)))
		binary.LittleEndian.PutUint16(rec[30:], uint16(len(extra)))
		binary.LittleEndian.PutUint32(rec[38:], 0o100644<<16)
		binary.LittleEndian.PutUint32(rec[42:], offset)
		copy(rec[46:], e.Name)
		copy(rec[46+len(e.Name):], extra)
		if _, err := w.Write(rec); err != nil {
			return 0, err
		}
		dirSize += int64(len(rec))
	}
	count := int64(len(z.entries))
	zip64 := z.forceZip64 || count >= max16 || dirStart >= max32 || dirSize >= max32
	if zip64 {
		end := dirStart + dirSize
		rec := make([]byte, 56+20)
		binary.LittleEndian.PutUint32(rec[0:], 0x06064b50)
		binary.LittleEndian.PutUint64(rec[4:], 44)
		binary.LittleEndian.PutUint16(rec[12:], 45)
		binary.LittleEndian.PutUint16(rec[14:], 45)
		binary.LittleEndian.PutUint64(rec[24:], uint64(count))
		binary.LittleEndian.PutUint64(rec[32:], uint64(count))
		binary.LittleEndian.PutUint64(rec[40:], uint64(dirSize))
		binary.LittleEndian.PutUint64(rec[48:], uint64(dirStart))
		binary.LittleEndian.PutUint32(rec[56:], 0x07064b50)
		binary.LittleEndian.PutUint64(rec[64:], uint64(end))
		binary.LittleEndian.PutUint32(rec[72:], 1)
		if _, err := w.Write(rec); err != nil {
			return 0, err
		}
	}
	eocd := make([]byte, 22)
	binary.LittleEndian.PutUint32(eocd[0:], 0x06054b50)
	c16 := uint16(max16)
	if count < max16 && !zip64 {
		c16 = uint16(count)
	}
	binary.LittleEndian.PutUint16(eocd[8:], c16)
	binary.LittleEndian.PutUint16(eocd[10:], c16)
	ds, do := uint32(max32), uint32(max32)
	if dirSize < max32 && !zip64 {
		ds = uint32(dirSize)
	}
	if dirStart < max32 && !zip64 {
		do = uint32(dirStart)
	}
	binary.LittleEndian.PutUint32(eocd[12:], ds)
	binary.LittleEndian.PutUint32(eocd[16:], do)
	if _, err := w.Write(eocd); err != nil {
		return 0, err
	}
	if err := w.Flush(); err != nil {
		return 0, err
	}
	if err := z.f.Sync(); err != nil {
		return 0, err
	}
	info, err := z.f.Stat()
	if err != nil {
		return 0, err
	}
	return info.Size(), z.f.Close()
}

func (z *zipAppender) Close() error { return z.f.Close() }

// readEntry decompresses one entry starting at its local header. r must
// expose the bytes from the local header offset onward.
func readEntry(r io.Reader, csize int64) (io.ReadCloser, error) {
	header := make([]byte, 30)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if binary.LittleEndian.Uint32(header) != 0x04034b50 {
		return nil, errors.New("invalid local header")
	}
	skip := int64(binary.LittleEndian.Uint16(header[26:])) + int64(binary.LittleEndian.Uint16(header[28:]))
	if _, err := io.CopyN(io.Discard, r, skip); err != nil {
		return nil, err
	}
	return flate.NewReader(io.LimitReader(r, csize)), nil
}
