package gatewaylogarchive

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// detectorTestRequest checks unsaved settings: an omitted or masked key uses
// the saved one.
type detectorTestRequest struct {
	URL            string   `json:"url"`
	APIKey         string   `json:"api_key"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	MinConfidence  float64  `json:"min_confidence"`
	Kinds          []string `json:"kinds"`
	Sample         string   `json:"sample"`
}

type detectorTestFinding struct {
	Label    string  `json:"label"`
	Score    float64 `json:"score"`
	Text     string  `json:"text"`
	Accepted bool    `json:"accepted"`
}

const defaultDetectorSample = "我叫王小明，手机 13812345678，邮箱 xiaoming.wang@example.com，住在北京市朝阳区建国路 88 号。My name is John Carter, card 4111 1111 1111 1111."

// testDetector sends a sample to the service with the given settings and
// shows what upload redaction would mask: the built-in rules first, then the
// model findings that pass the threshold and kind filters.
func (h *Handler) testDetector(c *gin.Context) {
	var req detectorTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "测试参数格式错误")
		return
	}
	ctx := c.Request.Context()
	stored, _, err := h.Runtime.Store.Config(ctx)
	if response.ErrorFrom(c, err) {
		return
	}
	cfg := DetectorConfig{Enabled: true, URL: req.URL, TimeoutSeconds: req.TimeoutSeconds, MinConfidence: req.MinConfidence, Kinds: req.Kinds}
	if strings.TrimSpace(cfg.URL) == "" {
		cfg.URL = stored.Detector.URL
	}
	if err := cfg.Normalize(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	key := strings.TrimSpace(req.APIKey)
	if key == "" || strings.Contains(key, "****") {
		var keyErr error
		key, keyErr = h.Runtime.detectorKey(stored.Detector)
		if keyErr != nil {
			response.BadRequest(c, "无法解密已保存的服务密钥，请重新填写")
			return
		}
	}
	sample := strings.TrimSpace(req.Sample)
	if sample == "" {
		sample = defaultDetectorSample
	}
	if len(sample) > 64<<10 {
		response.BadRequest(c, "测试文本不能超过 64 KB")
		return
	}
	client := newDetectorClient(cfg.URL, key, time.Duration(cfg.TimeoutSeconds)*time.Second, nil)
	testCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.TimeoutSeconds)*time.Second+5*time.Second)
	defer cancel()
	started := time.Now()
	// The service sees what upload redaction sends it: text after the rules.
	local := redactPlainText(sample, nil)
	raw, err := client.detect(testCtx, []string{local})
	elapsed := time.Since(started)
	if err != nil {
		msg := err.Error()
		if errors.Is(err, context.DeadlineExceeded) {
			msg = "脱敏服务在超时时间内没有响应"
		}
		response.Success(c, gin.H{"ok": false, "error": msg, "elapsed_ms": elapsed.Milliseconds()})
		return
	}
	accepted := newRemoteDetector(client, cfg)
	findings := make([]detectorTestFinding, 0, len(raw[0]))
	var keep []detectSpan
	for _, s := range raw[0] {
		ok := s.Score >= accepted.minConfidence && accepted.kinds[s.Label]
		if ok {
			keep = append(keep, s)
		}
		findings = append(findings, detectorTestFinding{Label: s.Label, Score: s.Score, Text: local[s.Start:s.End], Accepted: ok})
	}
	client.stats.mu.Lock()
	model := client.stats.model
	client.stats.mu.Unlock()
	response.Success(c, gin.H{
		"ok": true, "model": model, "elapsed_ms": elapsed.Milliseconds(),
		"sample": sample, "rules_only": local, "redacted": applySpans(local, keep), "findings": findings,
	})
}
