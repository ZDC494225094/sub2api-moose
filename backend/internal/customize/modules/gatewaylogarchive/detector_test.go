package gatewaylogarchive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// wordDetector reports every occurrence of its words, like a model would.
type wordDetector struct {
	words []string
	label string
	calls atomic.Int64
	sizes []int
	mu    sync.Mutex
	err   error
}

func (d *wordDetector) Detect(_ context.Context, texts []string) ([][]detectSpan, error) {
	d.calls.Add(1)
	d.mu.Lock()
	d.sizes = append(d.sizes, len(texts))
	d.mu.Unlock()
	if d.err != nil {
		return nil, d.err
	}
	out := make([][]detectSpan, len(texts))
	for i, text := range texts {
		for _, w := range d.words {
			for at := 0; ; {
				j := strings.Index(text[at:], w)
				if j < 0 {
					break
				}
				out[i] = append(out[i], detectSpan{Start: at + j, End: at + j + len(w), Label: d.label, Score: 0.99})
				at += j + len(w)
			}
		}
	}
	return out, nil
}

func redactWith(t *testing.T, det textDetector, input string) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, redactJSON(context.Background(), &out, strings.NewReader(input), true, det))
	require.True(t, json.Valid(out.Bytes()), out.String())
	return out.String()
}

func TestModelDetectionMasksContentAcrossNestedFormats(t *testing.T) {
	det := &wordDetector{words: []string{"王小明", "Wang Xiaoming", "北京市朝阳区"}, label: "private_person"}
	sse := "data: {\"delta\":{\"text\":\"我是王小明\"}}\n\ndata: [DONE]\n\n"
	input, err := json.Marshal(map[string]any{
		"messages":  []any{map[string]any{"role": "user", "content": "你好，我是王小明，住在北京市朝阳区，邮箱 a@example.com"}},
		"arguments": `{"query":"Wang Xiaoming","limit":3}`,
		"response":  sse,
		"tools":     []any{map[string]any{"name": "lookup", "description": "王小明 is an example name in the schema"}},
		"signature": "王小明",
	})
	require.NoError(t, err)
	got := redactWith(t, det, string(input))
	var doc map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &doc))
	content := doc["messages"].([]any)[0].(map[string]any)["content"]
	require.Equal(t, "你好，我是[REDACTED]，住在[REDACTED]，邮箱 [REDACTED]", content)
	require.Equal(t, `{"query":"[REDACTED]","limit":3}`, doc["arguments"])
	require.Equal(t, strings.Replace(sse, "王小明", piiMask, 1), doc["response"])
	// Tool schemas and opaque protocol fields are never sent to the model.
	require.Equal(t, "王小明 is an example name in the schema", doc["tools"].([]any)[0].(map[string]any)["description"])
	require.Equal(t, "王小明", doc["signature"])
	// Every pending text of the document goes out in one request.
	require.EqualValues(t, 1, det.calls.Load())
}

func TestModelDetectionKeepsOrderAcrossEarlyFlushes(t *testing.T) {
	det := &wordDetector{words: []string{"Alice"}, label: "private_person"}
	items := make([]string, detectBatchTexts*2+7)
	for i := range items {
		items[i] = "item " + strings.Repeat("x", i%5) + " for Alice"
	}
	input, err := json.Marshal(map[string]any{"items": items, "tail": "Alice again"})
	require.NoError(t, err)
	var doc struct {
		Items []string `json:"items"`
		Tail  string   `json:"tail"`
	}
	require.NoError(t, json.Unmarshal([]byte(redactWith(t, det, string(input))), &doc))
	require.Len(t, doc.Items, len(items))
	for i, item := range doc.Items {
		require.Equal(t, strings.Replace(items[i], "Alice", piiMask, 1), item)
	}
	require.Equal(t, "[REDACTED] again", doc.Tail)
	require.GreaterOrEqual(t, det.calls.Load(), int64(3))
}

func TestModelDetectionFailureFailsTheDocument(t *testing.T) {
	det := &wordDetector{err: detectorError("请求失败：connection refused")}
	var out bytes.Buffer
	err := redactJSON(context.Background(), &out, strings.NewReader(`{"content":"hello there"}`), true, det)
	require.ErrorIs(t, err, errDetector)
	// Nothing after the first pending value reaches the destination.
	require.Equal(t, `{"content":`, out.String())
}

func TestModelDetectionSkipsOpaqueText(t *testing.T) {
	det := &wordDetector{words: []string{"x"}}
	blob := strings.Repeat("QUJD", 100)
	got := redactWith(t, det, `{"image":"`+blob+`","n":"ab","s":"`+piiMask+`"}`)
	require.Equal(t, `{"image":"`+blob+`","n":"ab","s":"`+piiMask+`"}`, got)
	require.EqualValues(t, 0, det.calls.Load())
}

func TestApplySpans(t *testing.T) {
	text := "王小明和Bob"
	// Overlapping and adjacent spans merge; a span inside a rune widens to it.
	require.Equal(t, "[REDACTED]和[REDACTED]", applySpans(text, []detectSpan{{Start: 0, End: 4}, {Start: 3, End: 9}, {Start: 12, End: 15}}))
	require.Equal(t, text, applySpans(text, []detectSpan{{Start: -1, End: 2}, {Start: 5, End: 99}}))
	require.Equal(t, "[REDACTED]小明和Bob", applySpans(text, []detectSpan{{Start: 1, End: 2}}))
}

func detectorServer(t *testing.T, handler func(w http.ResponseWriter, texts []string)) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1/detect" || r.Header.Get("Authorization") != "Bearer secret-key-0123456789" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var req struct {
			Texts []string `json:"texts"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		handler(w, req.Texts)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func emailSpans(w http.ResponseWriter, texts []string) {
	results := make([][]detectSpan, len(texts))
	for i, text := range texts {
		results[i] = []detectSpan{}
		if j := strings.Index(text, "Bob"); j >= 0 {
			results[i] = append(results[i], detectSpan{Start: j, End: j + 3, Label: "private_person", Score: 0.95})
		}
		if j := strings.Index(text, "2026-01-02"); j >= 0 {
			results[i] = append(results[i], detectSpan{Start: j, End: j + 10, Label: "private_date", Score: 0.99})
		}
		if j := strings.Index(text, "Eve"); j >= 0 {
			results[i] = append(results[i], detectSpan{Start: j, End: j + 3, Label: "private_person", Score: 0.4})
		}
		// An out-of-range span from a misbehaving service is dropped.
		results[i] = append(results[i], detectSpan{Start: 0, End: len(text) + 10, Label: "email", Score: 1})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"model": "guard-test", "results": results})
}

func TestRemoteDetectorFiltersCachesAndSplits(t *testing.T) {
	srv, calls := detectorServer(t, emailSpans)
	client := newDetectorClient(srv.URL, "secret-key-0123456789", 5*time.Second, nil)
	client.maxTexts = 2
	cfg := defaultDetectorConfig()
	cfg.Enabled, cfg.URL = true, srv.URL
	require.NoError(t, cfg.Normalize())
	det := newRemoteDetector(client, cfg)

	texts := []string{"Bob on 2026-01-02", "Eve", "Bob on 2026-01-02", "none", "other"}
	got, err := det.Detect(context.Background(), texts)
	require.NoError(t, err)
	// Dates are off by default and Eve is below the 0.8 threshold.
	require.Equal(t, [][]detectSpan{{{Start: 0, End: 3, Label: "private_person", Score: 0.95}}, nil, {{Start: 0, End: 3, Label: "private_person", Score: 0.95}}, nil, nil}, got)
	// Four distinct texts, two per request.
	require.EqualValues(t, 2, calls.Load())
	_, err = det.Detect(context.Background(), []string{"Eve", "none"})
	require.NoError(t, err)
	require.EqualValues(t, 2, calls.Load(), "cached texts are not sent again")
	st := det.status()
	require.Equal(t, "guard-test", st.Model)
	// The duplicate in the first call is deduplicated, not a cache hit.
	require.EqualValues(t, 2, st.CacheHits)
	require.EqualValues(t, 0, st.Errors)
}

func TestRemoteDetectorErrorsAndFailOpen(t *testing.T) {
	srv, _ := detectorServer(t, func(w http.ResponseWriter, _ []string) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_ready", "message": "privacy model is not ready"})
	})
	cfg := DetectorConfig{Enabled: true, URL: srv.URL}
	require.NoError(t, cfg.Normalize())

	closed := newRemoteDetector(newDetectorClient(srv.URL, "secret-key-0123456789", 5*time.Second, nil), cfg)
	_, err := closed.Detect(context.Background(), []string{"x"})
	require.ErrorIs(t, err, errDetector)
	require.Contains(t, err.Error(), "HTTP 503：not_ready privacy model is not ready")
	require.Equal(t, err.Error(), closed.status().LastError)

	_, err = newRemoteDetector(newDetectorClient(srv.URL, "wrong", 5*time.Second, nil), cfg).Detect(context.Background(), []string{"x"})
	require.ErrorContains(t, err, "密钥错误")

	_, err = newRemoteDetector(newDetectorClient(srv.URL, "", 5*time.Second, errors.New("bad cipher")), cfg).Detect(context.Background(), []string{"x"})
	require.ErrorContains(t, err, "无法解密")

	cfg.FailOpen = true
	open := newRemoteDetector(newDetectorClient(srv.URL, "secret-key-0123456789", 5*time.Second, nil), cfg)
	got, err := open.Detect(context.Background(), []string{"a", "b"})
	require.NoError(t, err)
	require.Equal(t, [][]detectSpan{nil, nil}, got)
	require.EqualValues(t, 2, open.status().FailOpen)
}

func TestRemoteDetectorTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)
	client := newDetectorClient(srv.URL, "", 100*time.Millisecond, nil)
	_, err := client.detect(context.Background(), []string{"x"})
	require.ErrorIs(t, err, errDetector)
	require.Contains(t, err.Error(), "未完成")
}

func TestDetectorConfigNormalize(t *testing.T) {
	cfg := DetectorConfig{Enabled: true, URL: " http://privacy-detector:8090/ ", Kinds: []string{"email", "bogus", "email", "url"}, MinConfidence: 3}
	require.NoError(t, cfg.Normalize())
	require.Equal(t, "http://privacy-detector:8090", cfg.URL)
	require.Equal(t, []string{"email", "url"}, cfg.Kinds)
	require.Equal(t, 1.0, cfg.MinConfidence)
	require.Equal(t, 120, cfg.TimeoutSeconds)

	for _, bad := range []DetectorConfig{
		{Enabled: true},
		{URL: "ftp://host"},
		{URL: "http://user:pw@host"},
		{URL: "http://host/?q=1"},
		{Enabled: true, URL: "http://host", Kinds: []string{}},
	} {
		require.Error(t, bad.Normalize(), "%+v", bad)
	}
	// Disabled without an address is the default and valid.
	def := DefaultConfig()
	require.NoError(t, def.Normalize())
	require.False(t, def.Detector.Enabled)
	require.NotContains(t, def.Detector.Kinds, "url")
}

type prefixCipher struct{}

func (prefixCipher) Encrypt(v string) (string, error) { return "enc:" + v, nil }
func (prefixCipher) Decrypt(v string) (string, error) {
	if !strings.HasPrefix(v, "enc:") {
		return "", errors.New("bad ciphertext")
	}
	return strings.TrimPrefix(v, "enc:"), nil
}

func TestDetectorKeyIsEncryptedKeptAndHidden(t *testing.T) {
	s := &Store{Cipher: prefixCipher{}}
	stored := DetectorConfig{APIKeyEnc: "enc:old"}

	next := DetectorConfig{APIKey: " new-key "}
	require.NoError(t, s.resolveDetectorKey(&next, stored))
	require.Equal(t, DetectorConfig{APIKeyEnc: "enc:new-key"}, next)

	for _, keep := range []DetectorConfig{{}, {APIKey: "abcd****wxyz"}, {APIKeyEnc: "enc:injected"}} {
		require.NoError(t, s.resolveDetectorKey(&keep, stored))
		require.Equal(t, "enc:old", keep.APIKeyEnc)
	}

	clear := DetectorConfig{ClearAPIKey: true}
	require.NoError(t, s.resolveDetectorKey(&clear, stored))
	require.Empty(t, clear.APIKeyEnc)

	cfg := DefaultConfig()
	cfg.Detector.APIKeyEnc = "enc:old"
	raw, err := json.Marshal(newConfigView(cfg, nil))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "enc:old")
	require.NotContains(t, string(raw), "api_key_enc")
	require.Contains(t, string(raw), `"detector_api_key_set":true`)
}

func TestPipelineKeepsDetectorClientAcrossReloads(t *testing.T) {
	p := &pipeline{}
	require.Nil(t, p.detector())
	cfg := DetectorConfig{Enabled: true, URL: "http://a:1", TimeoutSeconds: 10, MinConfidence: 0.8, Kinds: []string{"email"}}
	p.setDetector(cfg, "k", nil)
	first := p.detectorClient
	require.NotNil(t, p.detector())
	cfg.MinConfidence = 0.5
	p.setDetector(cfg, "k", nil)
	require.Same(t, first, p.detectorClient, "a filter change keeps the cache")
	p.setDetector(cfg, "other", nil)
	require.NotSame(t, first, p.detectorClient)
	cfg.Enabled = false
	p.setDetector(cfg, "other", nil)
	require.Nil(t, p.detector())
}

// The upload path sends archive text to the service and fails closed: while
// it is down the bundle fails with the reason and no object is written; once
// it is back the retry uploads model-masked content.
func TestUploadUsesDetectorAndFailsClosedWhileItIsDown(t *testing.T) {
	var down atomic.Bool
	down.Store(true)
	srv, _ := detectorServer(t, func(w http.ResponseWriter, texts []string) {
		if down.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "not_ready"})
			return
		}
		results := make([][]detectSpan, len(texts))
		for i, text := range texts {
			results[i] = []detectSpan{}
			if j := strings.Index(text, "王小明"); j >= 0 {
				results[i] = append(results[i], detectSpan{Start: j, End: j + len("王小明"), Label: "private_person", Score: 0.97})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
	})
	store, objects := newMemStore(), newMemObjects()
	cfg := DefaultConfig()
	cfg.KeepLocalHours = 24
	p, err := startPipeline(store, func(BucketConfig) (objectStore, error) { return objects, nil }, "node-a", t.TempDir(), func(time.Time) string { return "2026-11-17" }, cfg, func(err error) { t.Log(err) }, &stats{})
	require.NoError(t, err)
	defer p.stop(false)
	cfg.Detector = DetectorConfig{Enabled: true, URL: srv.URL}
	require.NoError(t, cfg.Detector.Normalize())
	p.setDetector(cfg.Detector, "secret-key-0123456789", nil)

	r := captureRuntime(cfg, &captured{})
	r.policy.Load().submit = p.submit
	require.Equal(t, 200, post(gatewayEngine(r), "/v1/messages", `{"model":"m","messages":[{"role":"user","content":"请联系王小明处理，邮箱 a@example.com"}]}`).Code)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, p.flushNow(ctx))

	p.setBuckets([]BucketConfig{{ID: 1, Bucket: objects.Bucket(), Enabled: true}})
	p.wakeUploader()
	require.Eventually(t, func() bool {
		rows := store.bundleList()
		return len(rows) == 1 && rows[0].Status == StatusFailed
	}, 10*time.Second, 10*time.Millisecond)
	row := store.bundleList()[0]
	require.Contains(t, row.LastError, "模型脱敏服务")
	require.Contains(t, row.LastError, "HTTP 503")
	objects.mu.Lock()
	require.Empty(t, objects.objects, "nothing is uploaded without detection")
	objects.mu.Unlock()

	down.Store(false)
	require.NoError(t, store.update(row.ID, func(b *Bundle) { b.Status = StatusSealed }))
	p.wakeUploader()
	waitUploaded(t, store, 1)
	row = store.bundleList()[0]
	objects.mu.Lock()
	uploaded := string(objects.objects[row.ObjectKey])
	objects.mu.Unlock()
	require.NotContains(t, uploaded, "王小明")
	require.NotContains(t, uploaded, "a@example.com")
	require.Contains(t, uploaded, "请联系[REDACTED]处理，邮箱 [REDACTED]")
}

// Saving keeps detection settings that a request omits, stores the key only
// encrypted, never returns it, and the test endpoint uses the saved key.
func TestDetectorConfigRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := sqliteStore(t)
	store.Cipher = plainCipher{}
	srv, _ := detectorServer(t, emailSpans)
	h := &Handler{Runtime: NewRuntime(store, nil, Hooks{}, t.TempDir())}
	admin := gin.New()
	h.Register(admin.Group("/admin"), customize.NewManager(flagStore{"true"}))
	request := func(method, path, payload string, status int) string {
		t.Helper()
		w := httptest.NewRecorder()
		admin.ServeHTTP(w, httptest.NewRequest(method, "/admin/gateway-log-archive"+path, strings.NewReader(payload)))
		require.Equal(t, status, w.Code, w.Body.String())
		return w.Body.String()
	}
	body := request(http.MethodPut, "/config", `{"detector":{"enabled":true,"url":"`+srv.URL+`/","api_key":"secret-key-0123456789","kinds":["private_person","private_date"]}}`, 200)
	require.NotContains(t, body, "secret-key")
	require.Contains(t, body, `"detector_api_key_set":true`)
	require.Contains(t, body, `"url":"`+srv.URL+`"`)

	var raw string
	require.NoError(t, store.DB.QueryRow(`SELECT config FROM gateway_log_archive_config WHERE id = 1`).Scan(&raw))
	require.Contains(t, raw, `"api_key_enc":"enc:secret-key-0123456789"`)
	require.NotContains(t, raw, `"api_key":`)

	// A save from an older page without the detector block keeps it.
	body = request(http.MethodPut, "/config", `{"min_free_disk_gb":0}`, 200)
	require.Contains(t, body, `"enabled":true,"url":"`+srv.URL+`"`)
	require.Contains(t, body, `"detector_api_key_set":true`)

	body = request(http.MethodPost, "/detector/test", `{"kinds":["private_person"],"sample":"Bob on 2026-01-02, mail bob@example.com"}`, 200)
	var result struct {
		Data struct {
			OK       bool                  `json:"ok"`
			Model    string                `json:"model"`
			Redacted string                `json:"redacted"`
			Findings []detectorTestFinding `json:"findings"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &result), body)
	require.True(t, result.Data.OK, body)
	require.Equal(t, "guard-test", result.Data.Model)
	require.Equal(t, "[REDACTED] on 2026-01-02, mail [REDACTED]", result.Data.Redacted)
	require.Len(t, result.Data.Findings, 2)
	require.False(t, result.Data.Findings[1].Accepted, "dates were not selected")

	body = request(http.MethodPost, "/detector/test", `{"api_key":"wrong-key"}`, 200)
	require.Contains(t, body, `"ok":false`)
	require.Contains(t, body, "密钥错误")

	request(http.MethodPut, "/config", `{"detector":{"enabled":true,"url":"ftp://x"}}`, http.StatusBadRequest)
	body = request(http.MethodPut, "/config", `{"detector":{"enabled":false,"clear_api_key":true}}`, 200)
	require.Contains(t, body, `"detector_api_key_set":false`)
}
