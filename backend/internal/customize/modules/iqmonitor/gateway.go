package iqmonitor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// CallRequest is one probe: a single user message, optionally with a
// reasoning effort and output cap. Nothing else is injected.
type CallRequest struct {
	APIKey          string
	Model           string
	Prompt          string
	ReasoningEffort string
	MaxOutputTokens int
	Timeout         time.Duration
}

type CallResult struct {
	Content      string
	FinishReason string
	Truncated    bool
	InputTokens  int64
	OutputTokens int64
	FirstTokenMS *int
	TotalMS      int
	// StatusCode is the gateway HTTP status (0 for transport errors).
	StatusCode int
}

// CallError is a failed request. Retryable errors are retried up to the
// configured count; others (bad request, auth) fail at once.
type CallError struct {
	Message   string
	Retryable bool
	Result    CallResult
}

func (e *CallError) Error() string { return e.Message }

// Caller sends one streaming chat completion. The default implementation
// calls this server's own relay API; tests replace it.
type Caller interface {
	Call(ctx context.Context, req CallRequest) (CallResult, error)
}

// GatewayCaller posts to /v1/chat/completions on the local gateway, exactly
// like a user's client, so routing, effort policy and billing are the same.
type GatewayCaller struct {
	BaseURL func() string
	Client  *http.Client
	// Headers adds host-required headers to every loopback request, such as
	// the CDN origin secret; without it the host may refuse the request.
	Headers func(http.Header)
}

const maxResponseBytes = 4 << 20

func (g *GatewayCaller) Call(ctx context.Context, req CallRequest) (CallResult, error) {
	var result CallResult
	payload := map[string]any{
		"model":          req.Model,
		"messages":       []map[string]any{{"role": "user", "content": req.Prompt}},
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	if req.ReasoningEffort != "" {
		payload["reasoning_effort"] = req.ReasoningEffort
	}
	if req.MaxOutputTokens > 0 {
		payload["max_tokens"] = req.MaxOutputTokens
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(g.BaseURL(), "/")+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	httpReq.Header.Set("User-Agent", "sub2api-iq-monitor")
	if g.Headers != nil {
		g.Headers(httpReq.Header)
	}
	client := g.Client
	if client == nil {
		client = &http.Client{}
	}
	start := time.Now()
	elapsed := func() int { return int(time.Since(start).Milliseconds()) }
	resp, err := client.Do(httpReq)
	if err != nil {
		result.TotalMS = elapsed()
		return result, &CallError{Message: transportMessage(ctx, err, req.Timeout), Retryable: true, Result: result}
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
		result.TotalMS = elapsed()
		retryable := resp.StatusCode == 408 || resp.StatusCode == 429 || resp.StatusCode >= 500
		return result, &CallError{Message: httpErrorMessage(resp.StatusCode, raw), Retryable: retryable, Result: result}
	}
	var content strings.Builder
	sawData := false
	var streamErr string
	reader := bufio.NewReaderSize(io.LimitReader(resp.Body, maxResponseBytes), 64<<10)
	plain := &bytes.Buffer{}
	for {
		line, readErr := reader.ReadString('\n')
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data:") {
			sawData = true
			data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if data != "" && data != "[DONE]" {
				if msg := applyChunk(data, &content, &result); msg != "" {
					streamErr = msg
				}
				if result.FirstTokenMS == nil && content.Len() > 0 {
					ms := elapsed()
					result.FirstTokenMS = &ms
				}
			}
		} else if !sawData && trimmed != "" {
			plain.WriteString(line)
		}
		if readErr != nil {
			if readErr != io.EOF {
				result.Content = content.String()
				result.TotalMS = elapsed()
				return result, &CallError{Message: transportMessage(ctx, readErr, req.Timeout), Retryable: true, Result: result}
			}
			break
		}
	}
	result.TotalMS = elapsed()
	if !sawData {
		// A gateway that ignored the stream flag answered plain JSON.
		if err := applyPlain(plain.Bytes(), &content, &result); err != nil {
			return result, &CallError{Message: err.Error(), Retryable: true, Result: result}
		}
		if result.FirstTokenMS == nil {
			ms := result.TotalMS
			result.FirstTokenMS = &ms
		}
	}
	result.Content = content.String()
	result.Truncated = isTruncation(result.FinishReason)
	if streamErr != "" && strings.TrimSpace(result.Content) == "" {
		return result, &CallError{Message: streamErr, Retryable: true, Result: result}
	}
	if strings.TrimSpace(result.Content) == "" && !result.Truncated {
		return result, &CallError{Message: "上游返回空内容", Retryable: true, Result: result}
	}
	return result, nil
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func applyChunk(data string, content *strings.Builder, result *CallResult) string {
	var chunk streamChunk
	if json.Unmarshal([]byte(data), &chunk) != nil {
		return ""
	}
	if chunk.Error != nil && chunk.Error.Message != "" {
		return "流式错误：" + truncateRunes(chunk.Error.Message, 200)
	}
	if chunk.Usage != nil {
		result.InputTokens = chunk.Usage.PromptTokens
		result.OutputTokens = chunk.Usage.CompletionTokens
	}
	for _, c := range chunk.Choices {
		content.WriteString(c.Delta.Content)
		if c.FinishReason != nil && *c.FinishReason != "" {
			result.FinishReason = *c.FinishReason
		}
	}
	return ""
}

func applyPlain(raw []byte, content *strings.Builder, result *CallResult) error {
	var chunk streamChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		return errors.New("网关返回了无法解析的响应")
	}
	if chunk.Error != nil && chunk.Error.Message != "" {
		return errors.New(truncateRunes(chunk.Error.Message, 200))
	}
	if chunk.Usage != nil {
		result.InputTokens = chunk.Usage.PromptTokens
		result.OutputTokens = chunk.Usage.CompletionTokens
	}
	for _, c := range chunk.Choices {
		content.WriteString(c.Message.Content)
		if c.FinishReason != nil {
			result.FinishReason = *c.FinishReason
		}
	}
	return nil
}

func isTruncation(reason string) bool {
	switch strings.ToLower(reason) {
	case "length", "max_tokens", "max_output_tokens":
		return true
	}
	return false
}

func transportMessage(ctx context.Context, err error, timeout time.Duration) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Sprintf("请求超时（%d 秒）", int(timeout.Seconds()))
	}
	if ctx.Err() != nil {
		return "请求被取消"
	}
	// The message names only the loopback URL, never the key.
	return "请求失败：" + truncateRunes(err.Error(), 200)
}

// httpErrorMessage names the status and the gateway's own error text. An
// empty body means the request was refused before reaching the gateway
// handler (so no usage log exists), which is worth saying explicitly.
func httpErrorMessage(status int, raw []byte) string {
	text := gatewayErrorText(raw)
	if text != "" {
		return fmt.Sprintf("HTTP %d：%s", status, text)
	}
	if status == http.StatusForbidden {
		return "HTTP 403：响应为空，请求在进入网关前被拦截（通常是 CDN 回源密钥 ORIGIN_SECRET 或反向代理规则），因此使用记录中没有这条请求"
	}
	return fmt.Sprintf("HTTP %d：响应为空", status)
}

func gatewayErrorText(raw []byte) string {
	var decoded struct {
		Error struct {
			Type    string `json:"type"`
			Code    any    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Code    any    `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &decoded) == nil {
		label := func(code any, msg string) string {
			if c, ok := code.(string); ok && c != "" && !strings.Contains(msg, c) {
				return truncateRunes(c+"："+msg, 300)
			}
			return truncateRunes(msg, 300)
		}
		if decoded.Error.Message != "" {
			code := decoded.Error.Code
			if c, _ := code.(string); c == "" {
				code = decoded.Error.Type
			}
			return label(code, decoded.Error.Message)
		}
		if decoded.Message != "" {
			return label(decoded.Code, decoded.Message)
		}
	}
	return truncateRunes(strings.TrimSpace(string(raw)), 300)
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
