package gatewaylogarchive

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"
)

// DetectorConfig points upload redaction at a model-based detection service
// (deploy/privacy-detector: AstrLink's privacy worker behind an HTTP API).
// Its findings are masked in addition to the built-in rules.
type DetectorConfig struct {
	Enabled        bool     `json:"enabled"`
	URL            string   `json:"url"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	MinConfidence  float64  `json:"min_confidence"`
	Kinds          []string `json:"kinds"`
	// FailOpen uploads with the built-in rules only while the service is
	// unreachable. Off by default: the upload waits and retries instead.
	FailOpen bool `json:"fail_open"`

	// APIKeyEnc is the stored, encrypted key. It never leaves the server.
	APIKeyEnc string `json:"api_key_enc,omitempty"`
	// Request-only: a new key, or clearing the stored one. Never persisted.
	APIKey      string `json:"api_key,omitempty"`
	ClearAPIKey bool   `json:"clear_api_key,omitempty"`
}

// detectorKinds are the canonical kinds of the AstrLink Guard model.
var detectorKinds = []string{"email", "phone", "account", "payment_card", "ip_address", "url", "common_secret", "private_address", "private_date", "private_person"}

// URLs and dates are common, non-personal content in prompts and code, so
// they are opt-in.
func defaultDetectorKinds() []string {
	return []string{"email", "phone", "account", "payment_card", "ip_address", "common_secret", "private_address", "private_person"}
}

func defaultDetectorConfig() DetectorConfig {
	return DetectorConfig{TimeoutSeconds: 120, MinConfidence: 0.8, Kinds: defaultDetectorKinds()}
}

func (d *DetectorConfig) Normalize() error {
	d.URL = strings.TrimRight(strings.TrimSpace(d.URL), "/")
	if d.URL != "" {
		u, err := url.Parse(d.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return errors.New("脱敏服务地址必须是 http:// 或 https:// 开头的地址，例如 http://privacy-detector:8090")
		}
		if len(d.URL) > 500 {
			return errors.New("脱敏服务地址过长")
		}
	}
	if d.Enabled && d.URL == "" {
		return errors.New("开启模型脱敏前请填写脱敏服务地址")
	}
	d.TimeoutSeconds = int(clamp(int64(d.TimeoutSeconds), 5, 600, 120))
	switch {
	case d.MinConfidence == 0:
		d.MinConfidence = 0.8
	case d.MinConfidence < 0.05:
		d.MinConfidence = 0.05
	case d.MinConfidence > 1:
		d.MinConfidence = 1
	}
	if d.Kinds == nil {
		d.Kinds = defaultDetectorKinds()
	}
	kinds := make([]string, 0, len(d.Kinds))
	for _, k := range d.Kinds {
		k = strings.TrimSpace(k)
		if slices.Contains(detectorKinds, k) && !slices.Contains(kinds, k) {
			kinds = append(kinds, k)
		}
	}
	slices.Sort(kinds)
	d.Kinds = kinds
	if d.Enabled && len(d.Kinds) == 0 {
		return errors.New("模型脱敏至少需要选择一种信息类型")
	}
	if len(d.APIKey) > 4096 {
		return errors.New("脱敏服务密钥过长")
	}
	return nil
}

// errDetector marks failures of the detection service, so callers can tell
// them from storage failures.
var errDetector = errors.New("模型脱敏服务")

func detectorError(format string, args ...any) error {
	return fmt.Errorf("%w：%s", errDetector, fmt.Sprintf(format, args...))
}

// detectSpan is a sensitive byte range of one text.
type detectSpan struct {
	Start int     `json:"start"`
	End   int     `json:"end"`
	Label string  `json:"label"`
	Score float64 `json:"score"`
}

type textDetector interface {
	Detect(ctx context.Context, texts []string) ([][]detectSpan, error)
}

// spanCache keeps raw service results by text hash. Session documents repeat
// the whole conversation in every exchange, so most texts were seen before.
// Two generations bound memory to 2*limit entries.
type spanCache struct {
	mu           sync.Mutex
	limit        int
	current, old map[[32]byte][]detectSpan
}

func newSpanCache(limit int) *spanCache {
	return &spanCache{limit: limit, current: map[[32]byte][]detectSpan{}, old: map[[32]byte][]detectSpan{}}
}

func (c *spanCache) get(key [32]byte) ([]detectSpan, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.current[key]; ok {
		return v, true
	}
	if v, ok := c.old[key]; ok {
		c.putLocked(key, v)
		return v, true
	}
	return nil, false
}

func (c *spanCache) put(key [32]byte, v []detectSpan) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.putLocked(key, v)
}

func (c *spanCache) putLocked(key [32]byte, v []detectSpan) {
	if len(c.current) >= c.limit {
		c.old, c.current = c.current, make(map[[32]byte][]detectSpan, c.limit/4)
	}
	c.current[key] = v
}

type detectorStats struct {
	requests, texts, bytes, cacheHits, errors, failOpen atomic.Int64
	mu                                                  sync.Mutex
	lastError                                           string
	lastErrorAt                                         *time.Time
	model                                               string
}

// DetectorStatus is reported with the runtime status.
type DetectorStatus struct {
	Enabled     bool       `json:"enabled"`
	URL         string     `json:"url"`
	Model       string     `json:"model,omitempty"`
	Requests    int64      `json:"requests"`
	Texts       int64      `json:"texts"`
	Bytes       int64      `json:"bytes"`
	CacheHits   int64      `json:"cache_hits"`
	Errors      int64      `json:"errors"`
	FailOpen    int64      `json:"fail_open_skips"`
	LastError   string     `json:"last_error,omitempty"`
	LastErrorAt *time.Time `json:"last_error_at,omitempty"`
}

// detectorClient is the transport to one service; its cache and statistics
// survive configuration reloads that keep the address and key.
type detectorClient struct {
	url, key string
	timeout  time.Duration
	keyErr   error // the stored key could not be decrypted
	http     *http.Client
	cache    *spanCache
	stats    *detectorStats
	// Limits per HTTP request, below the service defaults (16 MiB, 4096 texts).
	maxBody, maxTexts int
}

func newDetectorClient(rawURL, key string, timeout time.Duration, keyErr error) *detectorClient {
	return &detectorClient{
		url: rawURL, key: key, timeout: timeout, keyErr: keyErr,
		http: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
			return errors.New("脱敏服务不允许重定向")
		}},
		cache: newSpanCache(100_000), stats: &detectorStats{},
		maxBody: 8 << 20, maxTexts: 2048,
	}
}

func (c *detectorClient) sameTransport(rawURL, key string, timeout time.Duration, keyErr error) bool {
	return c != nil && c.url == rawURL && c.key == key && c.timeout == timeout && (c.keyErr == nil) == (keyErr == nil)
}

func (c *detectorClient) fail(err error) error {
	c.stats.errors.Add(1)
	now := time.Now()
	c.stats.mu.Lock()
	c.stats.lastError, c.stats.lastErrorAt = err.Error(), &now
	c.stats.mu.Unlock()
	return err
}

type detectorReply struct {
	Model   string         `json:"model"`
	Results [][]detectSpan `json:"results"`
	Error   string         `json:"error"`
	Message string         `json:"message"`
}

// post sends one request and validates the spans against the texts.
func (c *detectorClient) post(ctx context.Context, texts []string) ([][]detectSpan, string, error) {
	if c.keyErr != nil {
		return nil, "", detectorError("无法解密已保存的服务密钥，请重新填写：%v", c.keyErr)
	}
	body, err := json.Marshal(map[string][]string{"texts": texts})
	if err != nil {
		return nil, "", detectorError("编码请求失败：%v", err)
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+"/v1/detect", bytes.NewReader(body))
	if err != nil {
		return nil, "", detectorError("地址无效：%v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, "", detectorError("请求超过 %s 未完成", c.timeout)
		}
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, "", detectorError("请求失败：%v", err)
	}
	defer resp.Body.Close()
	var reply detectorReply
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 256<<20)).Decode(&reply)
	if resp.StatusCode != http.StatusOK {
		msg := strings.TrimSpace(reply.Error + " " + reply.Message)
		switch {
		case resp.StatusCode == http.StatusUnauthorized:
			return nil, "", detectorError("密钥错误（HTTP 401）")
		case msg != "":
			return nil, "", detectorError("返回 HTTP %d：%s", resp.StatusCode, limit(msg, 300))
		default:
			return nil, "", detectorError("返回 HTTP %d", resp.StatusCode)
		}
	}
	if decodeErr != nil || len(reply.Results) != len(texts) {
		return nil, "", detectorError("响应格式不符")
	}
	for i, spans := range reply.Results {
		valid := spans[:0]
		for _, s := range spans {
			if s.Start >= 0 && s.End > s.Start && s.End <= len(texts[i]) {
				valid = append(valid, s)
			}
		}
		reply.Results[i] = valid
	}
	return reply.Results, reply.Model, nil
}

// detect resolves texts through the cache, sends the misses (deduplicated,
// split to the request limits) and returns raw spans in input order.
func (c *detectorClient) detect(ctx context.Context, texts []string) ([][]detectSpan, error) {
	out := make([][]detectSpan, len(texts))
	keys := make([][32]byte, len(texts))
	waiting := map[[32]byte][]int{}
	var missing []string
	var missingKeys [][32]byte
	for i, t := range texts {
		keys[i] = sha256.Sum256([]byte(t))
		if v, ok := c.cache.get(keys[i]); ok {
			out[i] = v
			c.stats.cacheHits.Add(1)
			continue
		}
		if _, seen := waiting[keys[i]]; !seen {
			missing = append(missing, t)
			missingKeys = append(missingKeys, keys[i])
		}
		waiting[keys[i]] = append(waiting[keys[i]], i)
	}
	for start := 0; start < len(missing); {
		end, size := start, 0
		for end < len(missing) && end-start < c.maxTexts && (end == start || size+len(missing[end])+16 <= c.maxBody) {
			size += len(missing[end]) + 16
			end++
		}
		c.stats.requests.Add(1)
		c.stats.texts.Add(int64(end - start))
		c.stats.bytes.Add(int64(size))
		results, model, err := c.post(ctx, missing[start:end])
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, c.fail(err)
		}
		if model != "" {
			c.stats.mu.Lock()
			c.stats.model = model
			c.stats.mu.Unlock()
		}
		for j, spans := range results {
			key := missingKeys[start+j]
			c.cache.put(key, spans)
			for _, i := range waiting[key] {
				out[i] = spans
			}
		}
		start = end
	}
	return out, nil
}

// remoteDetector applies the configured filters to a client's results.
type remoteDetector struct {
	client        *detectorClient
	minConfidence float64
	kinds         map[string]bool
	failOpen      bool
	url           string
}

func newRemoteDetector(client *detectorClient, cfg DetectorConfig) *remoteDetector {
	kinds := make(map[string]bool, len(cfg.Kinds))
	for _, k := range cfg.Kinds {
		kinds[k] = true
	}
	return &remoteDetector{client: client, minConfidence: cfg.MinConfidence, kinds: kinds, failOpen: cfg.FailOpen, url: cfg.URL}
}

func (d *remoteDetector) Detect(ctx context.Context, texts []string) ([][]detectSpan, error) {
	raw, err := d.client.detect(ctx, texts)
	if err != nil {
		if d.failOpen && ctx.Err() == nil {
			d.client.stats.failOpen.Add(int64(len(texts)))
			return make([][]detectSpan, len(texts)), nil
		}
		return nil, err
	}
	out := make([][]detectSpan, len(raw))
	for i, spans := range raw {
		for _, s := range spans {
			if s.Score >= d.minConfidence && d.kinds[s.Label] {
				out[i] = append(out[i], s)
			}
		}
	}
	return out, nil
}

func (d *remoteDetector) status() *DetectorStatus {
	s := d.client.stats
	st := &DetectorStatus{Enabled: true, URL: d.url, Requests: s.requests.Load(), Texts: s.texts.Load(), Bytes: s.bytes.Load(),
		CacheHits: s.cacheHits.Load(), Errors: s.errors.Load(), FailOpen: s.failOpen.Load()}
	s.mu.Lock()
	st.Model, st.LastError, st.LastErrorAt = s.model, s.lastError, s.lastErrorAt
	s.mu.Unlock()
	return st
}

// applySpans masks the union of spans, widened to whole UTF-8 characters.
func applySpans(text string, spans []detectSpan) string {
	if len(spans) == 0 {
		return text
	}
	type rng struct{ start, end int }
	ranges := make([]rng, 0, len(spans))
	for _, s := range spans {
		start, end := s.Start, s.End
		if start < 0 || end > len(text) || end <= start {
			continue
		}
		for start > 0 && !utf8.RuneStart(text[start]) {
			start--
		}
		for end < len(text) && !utf8.RuneStart(text[end]) {
			end++
		}
		ranges = append(ranges, rng{start, end})
	}
	if len(ranges) == 0 {
		return text
	}
	slices.SortFunc(ranges, func(a, b rng) int { return a.start - b.start })
	var b strings.Builder
	b.Grow(len(text))
	pos := 0
	for i := 0; i < len(ranges); {
		start, end := ranges[i].start, ranges[i].end
		for i++; i < len(ranges) && ranges[i].start <= end; i++ {
			end = max(end, ranges[i].end)
		}
		if start < pos { // inside an earlier mask
			start = pos
		}
		if end <= pos {
			continue
		}
		b.WriteString(text[pos:start])
		b.WriteString(piiMask)
		pos = end
	}
	b.WriteString(text[pos:])
	return b.String()
}

// detectable reports whether a text is worth sending to the model: base64
// payloads (images, encrypted blobs) and fragments are skipped.
func detectable(s string) bool {
	if len(s) < 4 || s == piiMask {
		return false
	}
	if len(s) >= 256 && !strings.ContainsAny(s, " \t\r\n") {
		opaque := true
		for i := 0; i < len(s) && opaque; i++ {
			c := s[i]
			opaque = c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("+/=_-.:;,", c) >= 0
		}
		if opaque {
			return false
		}
	}
	return true
}
