package gatewaylogarchive

import (
	"bufio"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// Responses WebSocket capture observes the hijacked connection passively: the
// bytes are tee'd into an RFC 6455 frame parser (with permessage-deflate) and
// never altered. A parser failure only stops capture for that connection.
func (r *Runtime) captureWebSocket(c *gin.Context, policy *capturePolicy) {
	who := r.identity(c)
	headerSession := ""
	if r.Hooks.HeaderSessionID != nil {
		headerSession = limit(strings.TrimSpace(r.Hooks.HeaderSessionID(c)), 255)
	}
	ws := &wsCapture{
		runtime: r, policy: policy, who: who, requestID: requestID(c),
		path: c.Request.URL.Path, session: headerSession,
		metadata: r.Hooks.MetadataSessionID,
	}
	ws.client.limit, ws.server.limit = policy.maxBody, policy.maxBody
	ws.client.masked = true
	ws.client.onMessage, ws.server.onMessage = ws.onClient, ws.onServer
	hw := &hijackTee{ResponseWriter: c.Writer, ws: ws}
	c.Writer = hw
	defer func() { c.Writer = hw.ResponseWriter }()
	c.Next()
	ws.close()
}

type hijackTee struct {
	gin.ResponseWriter
	ws *wsCapture
}

func (h *hijackTee) Unwrap() http.ResponseWriter { return h.ResponseWriter }

func (h *hijackTee) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, brw, err := h.ResponseWriter.Hijack()
	if err != nil || conn == nil || brw == nil {
		return conn, brw, err
	}
	h.ws.negotiate(h.ResponseWriter.Header().Get("Sec-WebSocket-Extensions"))
	// Bytes the HTTP server already buffered are replayed by the WebSocket
	// library from brw.Reader; observe them without consuming.
	if n := brw.Reader.Buffered(); n > 0 {
		if b, peekErr := brw.Reader.Peek(n); peekErr == nil {
			h.ws.client.feed(b)
		}
	}
	tee := &teeConn{Conn: conn, ws: h.ws}
	// net/http's hijack writer wraps the raw conn; route server frames through
	// the tee instead. It is freshly created and empty at this point.
	return tee, bufio.NewReadWriter(brw.Reader, bufio.NewWriter(tee)), nil
}

type teeConn struct {
	net.Conn
	ws *wsCapture
}

func (t *teeConn) Read(p []byte) (int, error) {
	n, err := t.Conn.Read(p)
	if n > 0 {
		t.ws.client.feed(p[:n])
	}
	return n, err
}

func (t *teeConn) Write(p []byte) (int, error) {
	n, err := t.Conn.Write(p)
	if n > 0 {
		t.ws.server.feed(p[:n])
	}
	return n, err
}

// wsCapture turns messages into exchanges: a client message opens one, server
// events accumulate until a terminal response event closes it.
type wsCapture struct {
	runtime   *Runtime
	policy    *capturePolicy
	who       Identity
	requestID string
	path      string
	session   string
	metadata  func(string) string

	client wsParser
	server wsParser

	mu      sync.Mutex
	cur     *Exchange
	curSize int
	seq     int
	closed  bool
}

func (w *wsCapture) negotiate(ext string) {
	lower := strings.ToLower(ext)
	if !strings.Contains(lower, "permessage-deflate") {
		return
	}
	w.client.deflate, w.server.deflate = true, true
	w.client.takeover = !strings.Contains(lower, "client_no_context_takeover")
	w.server.takeover = !strings.Contains(lower, "server_no_context_takeover")
}

func (w *wsCapture) onClient(payload []byte, truncated bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	if w.cur != nil {
		// A client message inside an open turn (e.g. cancel) is kept with it.
		w.cur.ResponseEvents = append(w.cur.ResponseEvents, wrapClientFollowup(payload))
		return
	}
	if w.session == "" {
		w.session = limit(bodySessionID(payload, w.metadata), 255)
	}
	w.seq++
	model := strings.TrimSpace(gjson.GetBytes(payload, "model").String())
	w.cur = &Exchange{
		RequestID: w.requestID + "-" + itoa(w.seq), Model: model, Platform: w.who.Platform,
		UserID: w.who.UserID, APIKeyID: w.who.APIKeyID, Method: http.MethodGet, Path: w.path,
		Transport: "websocket", StartedAt: time.Now(), Status: http.StatusSwitchingProtocols,
		RequestType: "application/json", Request: payload, RequestTrunc: truncated,
	}
	w.curSize = 0
}

func wrapClientFollowup(payload []byte) []byte {
	var b bytes.Buffer
	b.WriteString(`{"direction":"client","message":`)
	writeRaw(&b, payload)
	b.WriteByte('}')
	return b.Bytes()
}

func (w *wsCapture) onServer(payload []byte, truncated bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed || w.cur == nil {
		return
	}
	if w.curSize+len(payload) <= w.policy.maxBody && !truncated {
		w.cur.ResponseEvents = append(w.cur.ResponseEvents, payload)
		w.curSize += len(payload)
	} else {
		w.cur.ResponseTrunc = true
	}
	switch gjson.GetBytes(payload, "type").String() {
	case "response.completed", "response.failed", "response.incomplete", "response.cancelled", "error":
		w.emitLocked()
	}
}

func (w *wsCapture) emitLocked() {
	x := w.cur
	w.cur = nil
	if x == nil {
		return
	}
	x.FinishedAt = time.Now()
	key, ok := w.policy.cfg.ModelKey(x.Model)
	if !ok {
		w.runtime.stats.skippedModel.Add(1)
		return
	}
	x.ModelKey = key
	x.SessionID = w.session
	if x.SessionID == "" {
		x.SessionID = "ws-" + w.requestID
	}
	finishExchange(x)
	w.runtime.submit(w.policy, x)
}

func (w *wsCapture) close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cur != nil {
		w.cur.ResponseTrunc = true
		w.emitLocked()
	}
	w.closed = true
}

// wsParser is an incremental RFC 6455 frame decoder for one direction.
type wsParser struct {
	mu        sync.Mutex
	masked    bool
	deflate   bool
	takeover  bool
	limit     int
	onMessage func([]byte, bool)

	head        []byte
	remaining   int64
	mask        [4]byte
	maskPos     int
	frameMasked bool
	control     bool
	fin         bool

	inMessage  bool
	compressed bool
	msg        []byte
	overflow   bool
	window     []byte
	broken     bool
}

// Compressed payloads are buffered whole (they must be inflated to keep the
// context-takeover window in sync); beyond this the direction stops capture.
const maxCompressedMessage = 256 << 20

func (p *wsParser) feed(b []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() {
		if recover() != nil {
			p.broken = true
		}
	}()
	for len(b) > 0 && !p.broken {
		if p.remaining == 0 && !p.headerDone() {
			need := p.headerLen() - len(p.head)
			if need > len(b) {
				need = len(b)
			}
			p.head = append(p.head, b[:need]...)
			b = b[need:]
			if p.headerDone() {
				p.startFrame()
			}
			continue
		}
		n := int64(len(b))
		if n > p.remaining {
			n = p.remaining
		}
		p.payload(b[:n])
		b = b[n:]
		p.remaining -= n
		if p.remaining == 0 {
			p.endFrame()
		}
	}
}

func (p *wsParser) headerLen() int {
	if len(p.head) < 2 {
		return 2
	}
	n := 2
	switch p.head[1] & 0x7f {
	case 126:
		n += 2
	case 127:
		n += 8
	}
	if p.head[1]&0x80 != 0 {
		n += 4
	}
	return n
}

func (p *wsParser) headerDone() bool { return len(p.head) >= 2 && len(p.head) == p.headerLen() }

func (p *wsParser) startFrame() {
	h := p.head
	p.fin = h[0]&0x80 != 0
	rsv1 := h[0]&0x40 != 0
	opcode := h[0] & 0x0f
	masked := h[1]&0x80 != 0
	pos := 2
	length := int64(h[1] & 0x7f)
	switch length {
	case 126:
		length = int64(binary.BigEndian.Uint16(h[2:]))
		pos += 2
	case 127:
		length = int64(binary.BigEndian.Uint64(h[2:]))
		pos += 8
	}
	// Clients must mask (RFC 6455 5.1); anything else is not a frame we know.
	if length < 0 || (p.masked && !masked) {
		p.broken = true
		return
	}
	p.frameMasked = masked
	if masked {
		copy(p.mask[:], h[pos:pos+4])
	}
	p.maskPos = 0
	p.remaining = length
	p.control = opcode >= 8
	if !p.control {
		if opcode != 0 {
			p.inMessage = true
			p.compressed = rsv1 && p.deflate
			p.msg = p.msg[:0]
			p.overflow = false
		} else if !p.inMessage {
			p.broken = true
			return
		}
	}
	p.head = p.head[:0]
	if length == 0 {
		p.endFrame()
	}
}

func (p *wsParser) payload(b []byte) {
	if p.control || !p.inMessage {
		return
	}
	limit := p.limit
	if p.compressed {
		limit = maxCompressedMessage
	}
	room := limit - len(p.msg)
	if room <= 0 {
		p.overflow = true
		if p.compressed {
			p.broken = true
		}
		return
	}
	if len(b) > room {
		b = b[:room]
		p.overflow = true
	}
	start := len(p.msg)
	p.msg = append(p.msg, b...)
	if p.frameMasked {
		for i := start; i < len(p.msg); i++ {
			p.msg[i] ^= p.mask[p.maskPos&3]
			p.maskPos++
		}
	}
}

func (p *wsParser) endFrame() {
	if p.control || !p.fin || !p.inMessage {
		return
	}
	p.inMessage = false
	data := p.msg
	truncated := p.overflow
	if p.compressed {
		if p.overflow {
			p.broken = true
			return
		}
		out, trunc, err := p.inflate(data)
		if err != nil {
			p.broken = true
			return
		}
		data, truncated = out, trunc
	}
	msg := make([]byte, len(data))
	copy(msg, data)
	if p.onMessage != nil {
		p.onMessage(msg, truncated)
	}
}

// inflate decodes one permessage-deflate message; with context takeover the
// previous 32 KiB of output is the dictionary, as in RFC 7692.
func (p *wsParser) inflate(data []byte) ([]byte, bool, error) {
	var dict []byte
	if p.takeover {
		dict = p.window
	}
	src := io.MultiReader(bytes.NewReader(data), strings.NewReader("\x00\x00\xff\xff"))
	fr := flate.NewReaderDict(src, dict)
	defer fr.Close()
	var out []byte
	truncated := false
	chunk := make([]byte, 32<<10)
	for {
		n, err := fr.Read(chunk)
		if n > 0 {
			if p.takeover {
				p.window = append(p.window, chunk[:n]...)
				if len(p.window) > 64<<10 {
					p.window = append(p.window[:0], p.window[len(p.window)-32<<10:]...)
				}
			}
			room := p.limit - len(out)
			if room > 0 {
				if n > room {
					out = append(out, chunk[:room]...)
					truncated = true
				} else {
					out = append(out, chunk[:n]...)
				}
			} else {
				truncated = true
			}
		}
		if err == io.EOF || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
		if err != nil {
			return nil, false, err
		}
	}
	if len(p.window) > 32<<10 {
		p.window = append(p.window[:0], p.window[len(p.window)-32<<10:]...)
	}
	return out, truncated, nil
}
