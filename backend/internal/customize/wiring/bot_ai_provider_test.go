package wiring

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/stretchr/testify/require"
)

// fakeGateway records chat completion payloads and answers with a scripted
// function of the request.
type fakeGateway struct {
	mu       sync.Mutex
	payloads []map[string]any
	raws     []string
	respond  func(payload map[string]any, raw string) (int, string)
}

func (g *fakeGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var payload map[string]any
	_ = json.Unmarshal(raw, &payload)
	g.mu.Lock()
	g.payloads = append(g.payloads, payload)
	g.raws = append(g.raws, string(raw))
	g.mu.Unlock()
	status, body := g.respond(payload, string(raw))
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func sseText(text string, prompt, completion int) string {
	chunk, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"delta": map[string]any{"content": text}}}})
	usage, _ := json.Marshal(map[string]any{"choices": []any{}, "usage": map[string]any{"prompt_tokens": prompt, "completion_tokens": completion}})
	return "data: " + string(chunk) + "\n\ndata: " + string(usage) + "\n\ndata: [DONE]\n"
}

func newTestProvider(t *testing.T, gateway *fakeGateway) (*botAIProvider, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	server := httptest.NewServer(gateway)
	t.Cleanup(server.Close)
	p := newBotAIProvider(db, &botAITools{db: db, data: &stubBotData{}})
	p.gatewayURL = server.URL
	return p, mock
}

func expectKey(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT key FROM api_keys").WithArgs(int64(5)).WillReturnRows(sqlmock.NewRows([]string{"key"}).AddRow("sk-bot"))
}

func testAIRequest() botnotify.AIRequest {
	cfg := botnotify.DefaultAIConfig(1)
	keyID := int64(5)
	cfg.APIKeyID, cfg.Model, cfg.Enabled = &keyID, "gpt-test", true
	return botnotify.AIRequest{Config: cfg, Message: botnotify.Message{ChatType: "private", Text: "这个报错怎么办"}}
}

func TestProviderDropsUnsupportedFeaturesOneAtATime(t *testing.T) {
	gateway := &fakeGateway{respond: func(payload map[string]any, raw string) (int, string) {
		if _, ok := payload["reasoning_effort"]; ok {
			return http.StatusBadRequest, `{"error":"unsupported reasoning_effort"}`
		}
		if strings.Contains(raw, `"image_url"`) {
			return http.StatusBadRequest, `{"error":"image input not supported"}`
		}
		return http.StatusOK, sseText("请把报错文字发给我", 120, 30)
	}}
	p, mock := newTestProvider(t, gateway)
	expectKey(mock)
	req := testAIRequest()
	req.Images = []botnotify.Image{{MIME: "image/png", Data: []byte("png")}}
	result, err := p.Complete(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "请把报错文字发给我", result.Text)
	require.True(t, result.ImagesDropped)
	require.Equal(t, 120, result.InputTokens)
	require.Equal(t, 30, result.OutputTokens)
	require.Len(t, gateway.payloads, 3)
	last := gateway.payloads[2]
	messages := last["messages"].([]any)
	user := messages[len(messages)-1].(map[string]any)
	require.Contains(t, user["content"], "当前模型无法识别图片")
	require.True(t, p.capsFor("gpt-test").noEffort, "rejected effort is remembered")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProviderRunsToolsThenAnswers(t *testing.T) {
	calls := 0
	gateway := &fakeGateway{respond: func(payload map[string]any, raw string) (int, string) {
		calls++
		if calls == 1 {
			require.Equal(t, "low", payload["reasoning_effort"])
			require.NotEmpty(t, payload["tools"])
			return http.StatusOK, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","function":{"name":"query_groups","arguments":"{}"}}]}}]}` + "\n\ndata: [DONE]\n"
		}
		require.Contains(t, raw, "Codex-Pro")
		return http.StatusOK, sseText("推荐 Codex-Pro，倍率 0.35", 200, 20)
	}}
	p, mock := newTestProvider(t, gateway)
	expectKey(mock)
	mock.ExpectQuery("SELECT name, platform, subscription_type, rate_multiplier").WillReturnRows(
		sqlmock.NewRows([]string{"name", "platform", "subscription_type", "rate_multiplier", "description"}).
			AddRow("Codex-Pro", "openai", "standard", 0.35, "日常主力"))
	result, err := p.Complete(context.Background(), testAIRequest())
	require.NoError(t, err)
	require.Equal(t, "推荐 Codex-Pro，倍率 0.35", result.Text)
	require.Equal(t, []string{"query_groups"}, result.Tools)
	require.Contains(t, result.ToolOutputs[0], "Codex-Pro｜OpenAI/GPT｜按量（扣余额）｜倍率 0.35｜日常主力")
	require.Len(t, gateway.payloads, 2)
	require.Equal(t, "low", gateway.payloads[0]["reasoning_effort"])
	require.NotEmpty(t, gateway.payloads[0]["tools"])
	require.Contains(t, gateway.raws[1], "Codex-Pro", "tool result is sent back to the model")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestProviderReportsKeyAndHTTPErrorsAsCodes(t *testing.T) {
	gateway := &fakeGateway{respond: func(map[string]any, string) (int, string) { return http.StatusPaymentRequired, `{}` }}
	p, mock := newTestProvider(t, gateway)
	mock.ExpectQuery("SELECT key FROM api_keys").WillReturnError(io.EOF)
	_, err := p.Complete(context.Background(), testAIRequest())
	require.Equal(t, "key_unavailable", botnotify.AIErrorCode(err))
	expectKey(mock)
	_, err = p.Complete(context.Background(), testAIRequest())
	require.Equal(t, "provider_http_402", botnotify.AIErrorCode(err))
	require.Len(t, gateway.payloads, 1, "402 is not a request-shape error and is not retried")
}

func TestParseAIStreamBodyKeepsUsage(t *testing.T) {
	msg, ok := parseAIStreamBody([]byte(sseText("好", 10, 2)))
	require.True(t, ok)
	require.Equal(t, "好", msg.Content)
	require.Equal(t, aiUsage{PromptTokens: 10, CompletionTokens: 2}, msg.Usage)
}
