package gatewaylogarchive

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
)

// Identity is the authenticated caller as resolved by the host's API key
// middleware. It is only stored in the admin index, never in archived JSON.
type Identity struct {
	UserID   int64
	APIKeyID int64
	Platform string
}

// Hooks are host lookups injected by the composition root.
type Hooks struct {
	Identity          func(*gin.Context) Identity
	HeaderSessionID   func(*gin.Context) string
	MetadataSessionID func(raw string) string
}

// capturePolicy is an immutable snapshot swapped atomically by the runtime;
// nil means capture is off and the middleware is a pass-through.
type capturePolicy struct {
	cfg     Config
	maxBody int
	submit  func(*Exchange) bool
}

type routeKind int

const (
	routeNone routeKind = iota
	routeHTTP
	routeWebSocket
)

func classifyRoute(c *gin.Context) routeKind {
	path := c.FullPath()
	switch c.Request.Method {
	case http.MethodPost:
		switch path {
		case "/v1/messages", "/antigravity/v1/messages",
			"/v1/chat/completions", "/chat/completions",
			"/v1/responses", "/responses", "/backend-api/codex/responses":
			return routeHTTP
		case "/v1/responses/*subpath", "/responses/*subpath", "/backend-api/codex/responses/*subpath":
			sub := strings.Trim(c.Param("subpath"), "/")
			if sub == "input_tokens" || strings.HasSuffix(sub, "/input_tokens") {
				return routeNone
			}
			return routeHTTP
		case "/v1beta/models/*modelAction", "/antigravity/v1beta/models/*modelAction":
			action := c.Param("modelAction")
			if i := strings.LastIndex(action, ":"); i >= 0 {
				switch action[i+1:] {
				case "generateContent", "streamGenerateContent":
					return routeHTTP
				}
			}
		}
	case http.MethodGet:
		switch path {
		case "/v1/responses", "/responses", "/backend-api/codex/responses":
			if strings.EqualFold(c.GetHeader("Upgrade"), "websocket") {
				return routeWebSocket
			}
		}
	}
	return routeNone
}

func modelFromParams(c *gin.Context) string {
	action := strings.TrimPrefix(strings.TrimSpace(c.Param("modelAction")), "/")
	if action == "" {
		return ""
	}
	if i := strings.LastIndex(action, ":"); i >= 0 {
		action = action[:i]
	}
	return strings.TrimPrefix(action, "models/")
}

// CaptureMiddleware is mounted on gateway chains after API key auth and
// before any body rewriting, so it sees the client's own request bytes.
func (r *Runtime) CaptureMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		policy := r.policy.Load()
		if policy == nil || c.Request == nil {
			c.Next()
			return
		}
		switch classifyRoute(c) {
		case routeHTTP:
			r.captureHTTP(c, policy)
		case routeWebSocket:
			r.captureWebSocket(c, policy)
		default:
			c.Next()
		}
	}
}

func (r *Runtime) identity(c *gin.Context) Identity {
	if r.Hooks.Identity == nil {
		return Identity{}
	}
	return r.Hooks.Identity(c)
}

func requestID(c *gin.Context) string {
	if v, ok := c.Request.Context().Value(ctxkey.RequestID).(string); ok && v != "" {
		return v
	}
	if v := c.GetHeader("X-Request-Id"); v != "" {
		return v
	}
	return uuid.NewString()
}

// sessionID prefers explicit client headers, then Claude metadata.user_id,
// then the OpenAI prompt_cache_key. Empty means "one session per request".
func (r *Runtime) sessionID(c *gin.Context, body []byte) string {
	if r.Hooks.HeaderSessionID != nil {
		if id := strings.TrimSpace(r.Hooks.HeaderSessionID(c)); id != "" {
			return limit(id, 255)
		}
	}
	return limit(bodySessionID(body, r.Hooks.MetadataSessionID), 255)
}

func bodySessionID(body []byte, metadata func(string) string) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	if uid := gjson.GetBytes(body, "metadata.user_id"); uid.Type == gjson.String && metadata != nil {
		if id := strings.TrimSpace(metadata(uid.String())); id != "" {
			return id
		}
	}
	if key := gjson.GetBytes(body, "prompt_cache_key"); key.Type == gjson.String {
		return strings.TrimSpace(key.String())
	}
	return ""
}

func limit(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func sessionKey(apiKeyID int64, sessionID, modelKey string) string {
	sum := sha256.Sum256([]byte(strconv.FormatInt(apiKeyID, 10) + "\x00" + sessionID + "\x00" + modelKey))
	return hex.EncodeToString(sum[:12])
}

// redactQuery drops credential-bearing query parameters (Gemini "?key=").
func redactQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return ""
	}
	for name := range values {
		switch strings.ToLower(name) {
		case "key", "api_key", "apikey", "access_token", "token":
			values.Del(name)
		}
	}
	return values.Encode()
}

func (r *Runtime) captureHTTP(c *gin.Context, policy *capturePolicy) {
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		status, message := http.StatusBadRequest, "Failed to read request body"
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			status, message = http.StatusRequestEntityTooLarge, "Request body is too large"
		}
		c.JSON(status, gin.H{"error": gin.H{"type": "invalid_request_error", "message": message}})
		c.Abort()
		return
	}
	requestmodel.ResetRequestBody(c.Request, body)
	model := modelFromParams(c)
	if model == "" {
		model = requestmodel.FromBodyForRoute(c.FullPath(), c.GetHeader("Content-Type"), body)
	}
	modelKey, ok := policy.cfg.ModelKey(model)
	if !ok {
		r.stats.skippedModel.Add(1)
		c.Next()
		return
	}
	started := time.Now()
	who := r.identity(c)
	sid := r.sessionID(c, body)
	reqType := c.GetHeader("Content-Type")
	tee := &responseTee{ResponseWriter: c.Writer, limit: policy.maxBody}
	c.Writer = tee
	defer func() { c.Writer = tee.ResponseWriter }()
	c.Next()
	captured, truncated := tee.finish()
	status := tee.Status()
	if policy.cfg.SuccessOnly && (status < 200 || status > 299) {
		r.stats.skippedStatus.Add(1)
		return
	}
	reqBody, reqTrunc := body, false
	if len(reqBody) > policy.maxBody {
		reqBody, reqTrunc = reqBody[:policy.maxBody], true
	}
	x := &Exchange{
		RequestID:      requestID(c),
		SessionID:      sid,
		Model:          model,
		ModelKey:       modelKey,
		Platform:       who.Platform,
		UserID:         who.UserID,
		APIKeyID:       who.APIKeyID,
		Method:         c.Request.Method,
		Path:           c.Request.URL.Path,
		Query:          redactQuery(c.Request.URL.RawQuery),
		Transport:      "http",
		StartedAt:      started,
		FinishedAt:     time.Now(),
		Status:         status,
		RequestType:    reqType,
		ResponseType:   tee.Header().Get("Content-Type"),
		ResponseEncode: tee.Header().Get("Content-Encoding"),
		Request:        reqBody,
		Response:       captured,
		RequestTrunc:   reqTrunc,
		ResponseTrunc:  truncated,
	}
	finishExchange(x)
	r.submit(policy, x)
}

// finishExchange assigns the session identity; requests without a client
// session become their own single-exchange session.
func finishExchange(x *Exchange) {
	if x.SessionID == "" {
		x.Ephemeral = true
		x.SessionID = "req-" + x.RequestID
	}
	x.SessionKey = sessionKey(x.APIKeyID, x.SessionID, x.ModelKey)
	if x.Ephemeral {
		x.SessionKey += "-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
}

func (r *Runtime) submit(policy *capturePolicy, x *Exchange) {
	if policy.submit != nil && policy.submit(x) {
		r.stats.captured.Add(1)
		return
	}
	r.stats.dropped.Add(1)
}

// responseTee copies what the handler writes to the client, up to limit
// bytes. Writes after finish (late keep-alive goroutines) pass through only.
type responseTee struct {
	gin.ResponseWriter
	mu        sync.Mutex
	buf       bytes.Buffer
	limit     int
	truncated bool
	closed    bool
}

func (t *responseTee) record(p []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || len(p) == 0 {
		return
	}
	room := t.limit - t.buf.Len()
	if room <= 0 {
		t.truncated = true
		return
	}
	if len(p) > room {
		p = p[:room]
		t.truncated = true
	}
	t.buf.Write(p)
}

func (t *responseTee) Write(p []byte) (int, error) {
	n, err := t.ResponseWriter.Write(p)
	t.record(p[:n])
	return n, err
}

func (t *responseTee) WriteString(s string) (int, error) {
	n, err := t.ResponseWriter.WriteString(s)
	t.record([]byte(s[:n]))
	return n, err
}

func (t *responseTee) Unwrap() http.ResponseWriter { return t.ResponseWriter }

func (t *responseTee) finish() ([]byte, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closed = true
	return t.buf.Bytes(), t.truncated
}
