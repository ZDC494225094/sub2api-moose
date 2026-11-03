package gatewaylogarchive

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/klauspost/compress/zstd"
)

// Exchange is one captured request/response pair, handed from the request
// goroutine to the spool goroutine. Body slices are never mutated afterwards.
type Exchange struct {
	RequestID      string
	SessionID      string
	SessionKey     string
	Model          string
	ModelKey       string
	Platform       string
	UserID         int64
	APIKeyID       int64
	Method         string
	Path           string
	Query          string
	Transport      string // "http" or "websocket"
	StartedAt      time.Time
	FinishedAt     time.Time
	Status         int
	RequestType    string
	ResponseType   string
	ResponseEncode string
	Request        []byte
	Response       []byte
	ResponseEvents [][]byte // websocket server messages
	RequestTrunc   bool
	ResponseTrunc  bool
	Ephemeral      bool
}

func (e *Exchange) size() int64 {
	n := int64(len(e.Request) + len(e.Response))
	for _, ev := range e.ResponseEvents {
		n += int64(len(ev))
	}
	return n
}

// sessionHeader is the first spool record. UserID and APIKeyID are kept for
// the admin index only and are not written into the archived document.
type sessionHeader struct {
	SessionID string    `json:"session_id"`
	Key       string    `json:"key"`
	Model     string    `json:"model"`
	ModelKey  string    `json:"model_key"`
	Platform  string    `json:"platform"`
	UserID    int64     `json:"user_id"`
	APIKeyID  int64     `json:"api_key_id"`
	Part      int       `json:"part"`
	StartedAt time.Time `json:"started_at"`
	Ephemeral bool      `json:"ephemeral"`
}

// Spool record: [len u32][crc u32][unix-nanos i64][payload]. A truncated or
// corrupt tail (crash mid-write) is ignored when the spool is read back.
const recordHeader = 16

func encodeRecord(at time.Time, payload []byte) []byte {
	buf := make([]byte, recordHeader+len(payload))
	binary.LittleEndian.PutUint32(buf[0:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(buf[4:], crc32.ChecksumIEEE(payload))
	binary.LittleEndian.PutUint64(buf[8:], uint64(at.UnixNano()))
	copy(buf[recordHeader:], payload)
	return buf
}

func appendRecord(path string, at time.Time, payload []byte) (int64, error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	rec := encodeRecord(at, payload)
	_, werr := f.Write(rec)
	cerr := f.Close()
	if werr != nil {
		return 0, werr
	}
	return int64(len(rec)), cerr
}

type spoolRecord struct {
	at     time.Time
	offset int64
	length int
}

// scanSpool returns the header and the valid record positions.
func scanSpool(path string) (sessionHeader, []spoolRecord, error) {
	var header sessionHeader
	f, err := os.Open(path)
	if err != nil {
		return header, nil, err
	}
	defer f.Close()
	r := bufio.NewReaderSize(f, 1<<16)
	var records []spoolRecord
	var offset int64
	head := make([]byte, recordHeader)
	for {
		if _, err := io.ReadFull(r, head); err != nil {
			break
		}
		n := int(binary.LittleEndian.Uint32(head[0:]))
		sum := binary.LittleEndian.Uint32(head[4:])
		at := time.Unix(0, int64(binary.LittleEndian.Uint64(head[8:])))
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil || crc32.ChecksumIEEE(payload) != sum {
			break
		}
		if offset == 0 {
			if err := json.Unmarshal(payload, &header); err != nil {
				return header, nil, err
			}
		} else {
			records = append(records, spoolRecord{at: at, offset: offset + recordHeader, length: n})
		}
		offset += int64(recordHeader + n)
	}
	if offset == 0 {
		return header, nil, errors.New("empty spool")
	}
	return header, records, nil
}

// documentReader streams the session JSON:
// {"session_id":..,"model":..,..,"exchanges":[<record>,<record>]}
type documentReader struct {
	f      *os.File
	parts  []io.Reader
	reader io.Reader
}

func openDocument(path string, header sessionHeader, records []spoolRecord) (*documentReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var ended time.Time
	if len(records) > 0 {
		ended = records[len(records)-1].at
	}
	meta := struct {
		Format    string    `json:"format"`
		SessionID string    `json:"session_id"`
		Model     string    `json:"model"`
		ModelKey  string    `json:"model_key"`
		Platform  string    `json:"platform"`
		Part      int       `json:"part,omitempty"`
		StartedAt time.Time `json:"started_at"`
		EndedAt   time.Time `json:"ended_at"`
		Exchanges int       `json:"exchange_count"`
	}{"sub2api.gateway-log.v1", header.SessionID, header.Model, header.ModelKey, header.Platform, header.Part, header.StartedAt.UTC(), ended.UTC(), len(records)}
	prefix, _ := json.Marshal(meta)
	prefix = append(prefix[:len(prefix)-1], []byte(`,"exchanges":[`)...)
	parts := []io.Reader{bytes.NewReader(prefix)}
	for i, rec := range records {
		if i > 0 {
			parts = append(parts, strings.NewReader(","))
		}
		parts = append(parts, io.NewSectionReader(f, rec.offset, int64(rec.length)))
	}
	parts = append(parts, strings.NewReader("]}"))
	return &documentReader{f: f, parts: parts, reader: io.MultiReader(parts...)}, nil
}

func (d *documentReader) Read(p []byte) (int, error) { return d.reader.Read(p) }
func (d *documentReader) Close() error               { return d.f.Close() }

// encodeExchange renders one exchange record. JSON bodies are embedded
// verbatim (the original bytes, no re-encoding); other text (SSE streams) is a
// JSON string whose decoded value is byte-identical to what was sent; binary
// data is base64 with an explicit *_encoding marker.
func encodeExchange(index int, e *Exchange) []byte {
	var b bytes.Buffer
	b.Grow(int(e.size()) + 512)
	b.WriteString(`{"index":`)
	b.WriteString(strconv.Itoa(index))
	field := func(name, value string) {
		b.WriteString(`,"` + name + `":`)
		writeJSONString(&b, value)
	}
	field("request_id", e.RequestID)
	field("transport", e.Transport)
	field("started_at", e.StartedAt.UTC().Format(time.RFC3339Nano))
	field("finished_at", e.FinishedAt.UTC().Format(time.RFC3339Nano))
	b.WriteString(`,"duration_ms":` + strconv.FormatInt(e.FinishedAt.Sub(e.StartedAt).Milliseconds(), 10))
	field("method", e.Method)
	field("path", e.Path)
	if e.Query != "" {
		field("query", e.Query)
	}
	field("model", e.Model)
	b.WriteString(`,"status":` + strconv.Itoa(e.Status))
	if e.RequestType != "" {
		field("request_content_type", e.RequestType)
	}
	if e.ResponseType != "" {
		field("response_content_type", e.ResponseType)
	}
	b.WriteString(`,"stream":` + strconv.FormatBool(strings.Contains(strings.ToLower(e.ResponseType), "text/event-stream")))
	writeBody(&b, "request", e.Request, e.RequestTrunc)
	if e.Transport == "websocket" {
		b.WriteString(`,"response":[`)
		for i, ev := range e.ResponseEvents {
			if i > 0 {
				b.WriteByte(',')
			}
			writeRaw(&b, ev)
		}
		b.WriteByte(']')
		if e.ResponseTrunc {
			b.WriteString(`,"response_truncated":true`)
		}
	} else {
		body := e.Response
		if decoded, ok := decodeContent(e.ResponseEncode, body); ok {
			body = decoded
		} else if e.ResponseEncode != "" {
			field("response_content_encoding", e.ResponseEncode)
		}
		writeBody(&b, "response", body, e.ResponseTrunc)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func writeBody(b *bytes.Buffer, name string, body []byte, truncated bool) {
	b.WriteString(`,"` + name + `":`)
	switch {
	case len(body) == 0:
		b.WriteString("null")
	case !truncated && json.Valid(body):
		writeRaw(b, body)
	case utf8.Valid(body):
		writeJSONString(b, string(body))
	default:
		b.WriteByte('"')
		b.WriteString(base64.StdEncoding.EncodeToString(body))
		b.WriteByte('"')
		b.WriteString(`,"` + name + `_encoding":"base64"`)
	}
	if truncated {
		b.WriteString(`,"` + name + `_truncated":true`)
	}
}

// writeRaw embeds valid JSON as-is and anything else as a string.
func writeRaw(b *bytes.Buffer, body []byte) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) > 0 && json.Valid(trimmed) {
		b.Write(trimmed)
		return
	}
	writeJSONString(b, string(body))
}

// writeJSONString escapes without HTML escaping so "<", ">" and "&" survive
// unchanged.
func writeJSONString(b *bytes.Buffer, s string) {
	const hex = "0123456789abcdef"
	b.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' && c < utf8.RuneSelf {
			i++
			continue
		}
		if c < utf8.RuneSelf {
			b.WriteString(s[start:i])
			switch c {
			case '"', '\\':
				b.WriteByte('\\')
				b.WriteByte(c)
			case '\n':
				b.WriteString(`\n`)
			case '\r':
				b.WriteString(`\r`)
			case '\t':
				b.WriteString(`\t`)
			default:
				b.WriteString(`\u00`)
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&0xf])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteString(s[start:i])
			b.WriteString(`�`)
			i++
			start = i
			continue
		}
		if r == ' ' || r == ' ' {
			b.WriteString(s[start:i])
			b.WriteString(`\u202`)
			b.WriteByte(hex[r&0xf])
			i += size
			start = i
			continue
		}
		i += size
	}
	b.WriteString(s[start:])
	b.WriteByte('"')
}

const maxDecodedBody = 512 << 20

func decodeContent(encoding string, body []byte) ([]byte, bool) {
	var r io.Reader
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "identity":
		return body, true
	case "gzip", "x-gzip":
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, false
		}
		defer gr.Close()
		r = gr
	case "deflate":
		zr, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, false
		}
		defer zr.Close()
		r = zr
	case "zstd":
		dec, err := zstd.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, false
		}
		defer dec.Close()
		r = dec
	default:
		return nil, false
	}
	out, err := io.ReadAll(io.LimitReader(r, maxDecodedBody))
	if err != nil {
		return nil, false
	}
	return out, true
}
