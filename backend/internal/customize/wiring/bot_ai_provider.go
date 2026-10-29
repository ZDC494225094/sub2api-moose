package wiring

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
)

// botAIProvider answers bot messages by calling this server's own
// OpenAI-compatible gateway with the API key the admin selected for the
// bot. Billing, group routing and model access therefore follow the exact
// same rules as a direct API call with that key.
//
// The model gets read-only "skills" (tools, see botAITools) for live public
// site information instead of volatile data in the system prompt; the
// prompt itself is built by botnotify.BuildSystemPrompt and stays stable.
type botAIProvider struct {
	db     *sql.DB
	tools  *botAITools
	client *http.Client
	// gatewayURL overrides the loopback gateway address (tests only).
	gatewayURL string

	mu   sync.Mutex
	caps map[string]modelCaps
}

// modelCaps remembers optional request features a model rejected, so later
// answers skip the failing attempt for a while instead of paying for it.
type modelCaps struct {
	noEffort bool
	noTools  bool
	until    time.Time
}

const modelCapsTTL = 30 * time.Minute

func newBotAIProvider(db *sql.DB, tools *botAITools) *botAIProvider {
	return &botAIProvider{db: db, tools: tools, client: &http.Client{Timeout: 120 * time.Second}, caps: map[string]modelCaps{}}
}

type aiChatMessage struct {
	Role       string       `json:"role"`
	Content    any          `json:"content,omitempty"`
	ToolCalls  []aiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string       `json:"tool_call_id,omitempty"`
}

type aiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// aiReply is one assembled assistant turn plus its token usage.
type aiReply struct {
	Content   string
	ToolCalls []aiToolCall
	Usage     aiUsage
}

type aiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// aiAttempt is the optional request shape of one gateway call.
type aiAttempt struct {
	effort string
	tools  bool
	images bool
}

func (p *botAIProvider) capsFor(model string) modelCaps {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.caps[model]
	if time.Now().After(c.until) {
		return modelCaps{}
	}
	return c
}

func (p *botAIProvider) remember(model string, update func(*modelCaps)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.caps[model]
	if time.Now().After(c.until) {
		c = modelCaps{}
	}
	update(&c)
	c.until = time.Now().Add(modelCapsTTL)
	p.caps[model] = c
}

func (p *botAIProvider) Complete(ctx context.Context, req botnotify.AIRequest) (botnotify.AIResult, error) {
	var result botnotify.AIResult
	cfg := req.Config
	if cfg.APIKeyID == nil || strings.TrimSpace(cfg.Model) == "" {
		return result, errors.New("ai_not_configured")
	}
	var key string
	err := p.db.QueryRowContext(ctx, `SELECT key FROM api_keys WHERE id=$1 AND status='active' AND deleted_at IS NULL`, *cfg.APIKeyID).Scan(&key)
	if err != nil {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		return result, errors.New("ai_key_unavailable")
	}
	messages := []aiChatMessage{{Role: "system", Content: botnotify.BuildSystemPrompt(req)}}
	// Conversation history keeps follow-ups in context; the long shared
	// prefix (system prompt plus prior turns) is also what upstream prompt
	// caches reuse, instead of every message starting cold.
	for _, turn := range req.History {
		if turn.User != "" {
			messages = append(messages, aiChatMessage{Role: "user", Content: turn.User})
		}
		if turn.Assistant != "" {
			messages = append(messages, aiChatMessage{Role: "assistant", Content: turn.Assistant})
		}
	}
	userIndex := len(messages)
	messages = append(messages, userMessage(req, true))

	caps := p.capsFor(cfg.Model)
	at := aiAttempt{effort: cfg.ReasoningEffort, tools: !caps.noTools, images: len(req.Images) > 0}
	if caps.noEffort {
		at.effort = ""
	}
	toolsUsable := at.tools
	// Up to three tool rounds, then a final answer without tools.
	for round := 0; round < 4; round++ {
		at.tools = toolsUsable && round < 3
		reply, err := p.chat(ctx, key, cfg, messages, at)
		// The first call also learns what the model accepts: a 400-type
		// rejection drops one optional feature at a time (reasoning effort,
		// images, tools) and retries, so basic Q&A works with any model.
		for err != nil && round == 0 && isRequestShapeError(err) {
			switch {
			case at.effort != "":
				at.effort = ""
				p.remember(cfg.Model, func(c *modelCaps) { c.noEffort = true })
			case at.images:
				at.images = false
				result.ImagesDropped = true
				messages[userIndex] = userMessage(req, false)
			case at.tools:
				at.tools, toolsUsable = false, false
				p.remember(cfg.Model, func(c *modelCaps) { c.noTools = true })
			default:
				return result, err
			}
			reply, err = p.chat(ctx, key, cfg, messages, at)
		}
		if err != nil {
			return result, err
		}
		result.InputTokens += reply.Usage.PromptTokens
		result.OutputTokens += reply.Usage.CompletionTokens
		if len(reply.ToolCalls) == 0 || !at.tools {
			content := strings.TrimSpace(reply.Content)
			if content == "" {
				return result, errors.New("ai_provider_empty")
			}
			result.Text = content
			return result, nil
		}
		messages = append(messages, aiChatMessage{Role: "assistant", ToolCalls: reply.ToolCalls})
		for _, tc := range reply.ToolCalls {
			output := p.tools.exec(ctx, cfg, tc.Function.Name, tc.Function.Arguments)
			result.Tools = append(result.Tools, tc.Function.Name)
			result.ToolOutputs = append(result.ToolOutputs, output)
			messages = append(messages, aiChatMessage{Role: "tool", ToolCallID: tc.ID, Content: output})
		}
	}
	return result, errors.New("ai_tool_loop_exceeded")
}

// userMessage renders the current turn: text, plus image parts when the
// model takes images. Without them the model is told what it cannot see.
func userMessage(req botnotify.AIRequest, withImages bool) aiChatMessage {
	turn := req
	if !withImages && len(req.Images) > 0 {
		dropped := fmt.Sprintf("用户发送了 %d 张图片，但当前模型无法识别图片；请先根据文字回答，必要时请用户把图片中的报错文字发过来", len(req.Images))
		if turn.ImageNote != "" {
			dropped = turn.ImageNote + "；" + dropped
		}
		turn.ImageNote = dropped
		turn.Images = nil
		if strings.TrimSpace(turn.Message.Text) == "" {
			turn.Message.Text = "（用户只发送了图片）"
		}
	}
	text := botnotify.UserTurnText(turn)
	if len(turn.Images) == 0 {
		return aiChatMessage{Role: "user", Content: text}
	}
	parts := []map[string]any{{"type": "text", "text": text}}
	for _, img := range turn.Images {
		parts = append(parts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": img.DataURI()}})
	}
	return aiChatMessage{Role: "user", Content: parts}
}

// isRequestShapeError reports gateway rejections caused by the request
// format (unsupported parameter, image input, payload size) rather than by
// the key, balance or upstream health.
func isRequestShapeError(err error) bool {
	switch err.Error() {
	case "ai_provider_http_400", "ai_provider_http_413", "ai_provider_http_415", "ai_provider_http_422":
		return true
	}
	return false
}

func (p *botAIProvider) chat(ctx context.Context, key string, cfg botnotify.AIConfig, messages []aiChatMessage, at aiAttempt) (aiReply, error) {
	payload := map[string]any{
		"model":          cfg.Model,
		"messages":       messages,
		"max_tokens":     cfg.MaxOutputTokens,
		"temperature":    0.3,
		"stream":         true,
		"stream_options": map[string]any{"include_usage": true},
	}
	if at.effort != "" {
		payload["reasoning_effort"] = at.effort
	}
	if at.tools {
		payload["tools"] = p.tools.definitions(cfg)
		payload["tool_choice"] = "auto"
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return aiReply{}, err
	}
	base := p.gatewayURL
	if base == "" {
		base = "http://" + gatewayLoopbackAddr()
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return aiReply{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("User-Agent", "sub2api-bot-notify")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return aiReply{}, ctx.Err()
		}
		// The error text names only the loopback URL, never the key.
		return aiReply{}, fmt.Errorf("ai_provider_unreachable: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		if ctx.Err() != nil {
			return aiReply{}, ctx.Err()
		}
		return aiReply{}, errors.New("ai_provider_interrupted")
	}
	if resp.StatusCode != http.StatusOK {
		return aiReply{}, fmt.Errorf("ai_provider_http_%d", resp.StatusCode)
	}
	// Streaming responses are assembled from SSE deltas; a gateway that
	// ignores the stream flag and answers plain JSON is handled below.
	if msg, ok := parseAIStreamBody(raw); ok {
		return msg, nil
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content   string       `json:"content"`
				ToolCalls []aiToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage aiUsage `json:"usage"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return aiReply{}, errors.New("ai_provider_bad_response")
	}
	if len(decoded.Choices) == 0 {
		return aiReply{}, errors.New("ai_provider_empty")
	}
	m := decoded.Choices[0].Message
	return aiReply{Content: m.Content, ToolCalls: m.ToolCalls, Usage: decoded.Usage}, nil
}

// parseAIStreamBody assembles an SSE chat completions stream into a single
// reply: content deltas are concatenated, tool calls are merged by delta
// index (id and name arrive in the first fragment, arguments stream in
// pieces) and the trailing usage chunk is kept.
func parseAIStreamBody(body []byte) (aiReply, bool) {
	text := strings.TrimSpace(string(body))
	if !strings.HasPrefix(text, "data:") {
		return aiReply{}, false
	}
	var content strings.Builder
	type callAcc struct{ id, name, args string }
	calls := map[int]*callAcc{}
	var order []int
	var usage aiUsage
	sawChunk := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *aiUsage `json:"usage"`
		}
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		sawChunk = true
		if chunk.Usage != nil {
			usage = *chunk.Usage
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta
		content.WriteString(delta.Content)
		for _, tc := range delta.ToolCalls {
			acc, ok := calls[tc.Index]
			if !ok {
				acc = &callAcc{}
				calls[tc.Index] = acc
				order = append(order, tc.Index)
			}
			if tc.ID != "" {
				acc.id = tc.ID
			}
			if tc.Function.Name != "" {
				acc.name = tc.Function.Name
			}
			acc.args += tc.Function.Arguments
		}
	}
	if !sawChunk {
		return aiReply{}, false
	}
	msg := aiReply{Content: content.String(), Usage: usage}
	sort.Ints(order)
	for _, idx := range order {
		acc := calls[idx]
		if acc.name == "" {
			continue
		}
		call := aiToolCall{ID: acc.id, Type: "function"}
		call.Function.Name = acc.name
		call.Function.Arguments = acc.args
		msg.ToolCalls = append(msg.ToolCalls, call)
	}
	return msg, true
}
