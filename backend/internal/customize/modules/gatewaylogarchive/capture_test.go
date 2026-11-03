package gatewaylogarchive

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type captured struct {
	mu   sync.Mutex
	list []*Exchange
}

func (c *captured) submit(x *Exchange) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.list = append(c.list, x)
	return true
}

func (c *captured) all() []*Exchange {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*Exchange(nil), c.list...)
}

func captureRuntime(cfg Config, sink *captured) *Runtime {
	r := &Runtime{Hooks: Hooks{
		Identity:          func(*gin.Context) Identity { return Identity{UserID: 3, APIKeyID: 9, Platform: "anthropic"} },
		HeaderSessionID:   func(c *gin.Context) string { return c.GetHeader("X-Test-Session") },
		MetadataSessionID: func(raw string) string { return strings.TrimPrefix(raw, "user_") },
	}}
	r.policy.Store(&capturePolicy{cfg: cfg, maxBody: 1 << 20, submit: sink.submit})
	return r
}

func gatewayEngine(r *Runtime) *gin.Engine {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	g := e.Group("/v1", r.CaptureMiddleware())
	g.POST("/messages", func(c *gin.Context) {
		body, _ := io.ReadAll(c.Request.Body)
		if strings.Contains(string(body), `"stream":true`) {
			c.Header("Content-Type", "text/event-stream")
			c.Status(http.StatusOK)
			_, _ = c.Writer.WriteString("event: message_start\ndata: {\"type\":\"message_start\"}\n\n")
			c.Writer.Flush()
			_, _ = c.Writer.Write([]byte("data: {\"type\":\"message_stop\",\"t\":\"<b>&\"}\n\n"))
			return
		}
		if strings.Contains(string(body), "fail") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bad"})
			return
		}
		c.Data(http.StatusOK, "application/json", append([]byte(`{"echo":`), append(body, '}')...))
	})
	g.POST("/messages/count_tokens", func(c *gin.Context) { c.JSON(200, gin.H{"input_tokens": 1}) })
	return e
}

func post(e http.Handler, path, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	return w
}

func TestCaptureHTTPKeepsRawBodiesAndPassesRequestThrough(t *testing.T) {
	sink := &captured{}
	cfg := DefaultConfig()
	e := gatewayEngine(captureRuntime(cfg, sink))

	body := "{\n  \"model\": \"claude-opus-5\",\n  \"metadata\": {\"user_id\": \"user_sess-1\"}\n}"
	w := post(e, "/v1/messages", body)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, `{"echo":`+body+`}`, w.Body.String(), "the handler still receives the original body")

	w = post(e, "/v1/messages", `{"model":"claude-opus-5","stream":true}`, "X-Test-Session", "hdr-1")
	require.Equal(t, http.StatusOK, w.Code)
	post(e, "/v1/messages", `{"model":"claude-opus-5","fail":1}`)
	post(e, "/v1/messages/count_tokens", `{"model":"claude-opus-5"}`)

	list := sink.all()
	require.Len(t, list, 2, "count_tokens and non-2xx responses are not archived")
	require.Equal(t, "sess-1", list[0].SessionID)
	require.Equal(t, "hdr-1", list[1].SessionID, "client session header wins")
	require.Equal(t, body, string(list[0].Request))
	require.Equal(t, int64(9), list[0].APIKeyID)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(encodeExchange(1, list[0]), &doc))
	require.Equal(t, map[string]any{"model": "claude-opus-5", "metadata": map[string]any{"user_id": "user_sess-1"}}, doc["request"])
	require.NoError(t, json.Unmarshal(encodeExchange(2, list[1]), &doc))
	require.Equal(t, true, doc["stream"])
	require.Equal(t, w.Body.String(), doc["response"], "SSE text is stored byte-for-byte as a JSON string")
	require.Contains(t, string(encodeExchange(2, list[1])), `<b>&`, "no HTML escaping")
}

func TestCaptureHonoursModelSelection(t *testing.T) {
	sink := &captured{}
	cfg := Config{Mode: ModeSelected, Rules: []ModelRule{{Pattern: "claude-opus-5*", Alias: "Opus5"}}}
	require.NoError(t, cfg.Normalize())
	e := gatewayEngine(captureRuntime(cfg, sink))
	post(e, "/v1/messages", `{"model":"claude-opus-5-20261001"}`)
	post(e, "/v1/messages", `{"model":"claude-haiku-5"}`)
	list := sink.all()
	require.Len(t, list, 1)
	require.Equal(t, "Opus5", list[0].ModelKey)
	require.True(t, list[0].Ephemeral, "no session id: one document per request")
}

func TestCaptureIsPassThroughWhenOff(t *testing.T) {
	r := &Runtime{}
	e := gatewayEngine(r)
	w := post(e, "/v1/messages", `{"model":"x"}`)
	require.Equal(t, `{"echo":{"model":"x"}}`, w.Body.String())
}

func TestEncodeExchangeBodies(t *testing.T) {
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	_, _ = zw.Write([]byte(`{"ok":true}`))
	_ = zw.Close()
	x := &Exchange{Transport: "http", Request: []byte{0xff, 0x00, 0x01}, Response: gz.Bytes(), ResponseEncode: "gzip"}
	var doc map[string]any
	require.NoError(t, json.Unmarshal(encodeExchange(1, x), &doc))
	require.Equal(t, "base64", doc["request_encoding"])
	require.Equal(t, map[string]any{"ok": true}, doc["response"])
}

func TestCaptureWebSocketWithContextTakeover(t *testing.T) {
	sink := &captured{}
	r := captureRuntime(DefaultConfig(), sink)
	gin.SetMode(gin.TestMode)
	e := gin.New()
	var negotiated string
	e.GET("/v1/responses", r.CaptureMiddleware(), func(c *gin.Context) {
		conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{CompressionMode: websocket.CompressionContextTakeover})
		if err != nil {
			return
		}
		negotiated = c.Writer.Header().Get("Sec-WebSocket-Extensions")
		defer conn.CloseNow()
		ctx := c.Request.Context()
		for turn := 0; turn < 2; turn++ {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				return
			}
			big := strings.Repeat("delta text ", 400)
			_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"response.output_text.delta","delta":"`+big+`"}`))
			_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"response.completed","echo":`+string(msg)+`}`))
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	srv := httptest.NewServer(e)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	header := http.Header{"X-Test-Session": {"ws-sess"}}
	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/v1/responses",
		&websocket.DialOptions{HTTPHeader: header, CompressionMode: websocket.CompressionContextTakeover})
	require.NoError(t, err)
	for turn := 0; turn < 2; turn++ {
		req := `{"type":"response.create","model":"gpt-6.1-sol","input":"` + strings.Repeat("question ", 100) + `"}`
		require.NoError(t, conn.Write(ctx, websocket.MessageText, []byte(req)))
		for i := 0; i < 2; i++ {
			_, _, err := conn.Read(ctx)
			require.NoError(t, err)
		}
	}
	_ = conn.Close(websocket.StatusNormalClosure, "")
	require.Eventually(t, func() bool { return len(sink.all()) == 2 }, 5*time.Second, 20*time.Millisecond)
	require.Equal(t, "permessage-deflate", negotiated, "context takeover in both directions")
	wantDelta := `{"type":"response.output_text.delta","delta":"` + strings.Repeat("delta text ", 400) + `"}`

	for _, x := range sink.all() {
		require.Equal(t, "websocket", x.Transport)
		require.Equal(t, "ws-sess", x.SessionID)
		require.Equal(t, "gpt-6.1-sol", x.Model)
		require.Contains(t, string(x.Request), `"response.create"`)
		require.Len(t, x.ResponseEvents, 2)
		require.Equal(t, wantDelta, string(x.ResponseEvents[0]))
		require.Equal(t, `{"type":"response.create","model":"gpt-6.1-sol","input":"`+strings.Repeat("question ", 100)+`"}`, string(x.Request))
		require.Contains(t, string(x.ResponseEvents[1]), `"response.completed"`)
		var doc map[string]any
		require.NoError(t, json.Unmarshal(encodeExchange(1, x), &doc))
		require.Len(t, doc["response"], 2)
	}
}
