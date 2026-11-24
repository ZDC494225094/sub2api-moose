package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
)

// new-api (QuantumNous/new-api) relays Gemini channels in several dialects.
// These fixtures mirror its serialized usage objects so a format drift that
// silently drops Gemini implicit-cache hits shows up as a test failure.

// Chat Completions, as produced by new-api's Gemini → OpenAI chat conversion:
// cached tokens live in prompt_tokens_details, input_tokens_details is null.
const newAPIGeminiChatUsage = `{"prompt_tokens":12000,"completion_tokens":300,"total_tokens":12300,` +
	`"prompt_tokens_details":{"cached_tokens":10500,"text_tokens":12000,"audio_tokens":0,"image_tokens":0},` +
	`"completion_tokens_details":{"text_tokens":0,"audio_tokens":0,"reasoning_tokens":120,"image_tokens":0},` +
	`"input_tokens":0,"output_tokens":0,"input_tokens_details":null,` +
	`"claude_cache_creation_5_m_tokens":0,"claude_cache_creation_1_h_tokens":0}`

func TestNewAPIGeminiChatCompletionsUsageKeepsCachedTokens(t *testing.T) {
	body := []byte(`{"id":"chatcmpl-1","object":"chat.completion","choices":[],"usage":` + newAPIGeminiChatUsage + `}`)
	usage, ok := extractOpenAIUsageFromJSONBytes(body)
	require.True(t, ok)
	require.Equal(t, 12000, usage.InputTokens)
	require.Equal(t, 10500, usage.CacheReadInputTokens)

	stream := extractCCStreamUsage(`{"id":"chatcmpl-1","object":"chat.completion.chunk","choices":[],"usage":` + newAPIGeminiChatUsage + `}`)
	require.NotNil(t, stream)
	require.Equal(t, 10500, stream.CacheReadInputTokens)
}

// Responses, as produced by new-api's chat → Responses conversion: both detail
// objects are populated from the chat prompt details.
func TestNewAPIGeminiResponsesUsageKeepsCachedTokens(t *testing.T) {
	raw := `{"prompt_tokens":12000,"completion_tokens":300,"total_tokens":12300,` +
		`"prompt_tokens_details":{"cached_tokens":10500,"text_tokens":12000,"audio_tokens":0,"image_tokens":0},` +
		`"completion_tokens_details":{"text_tokens":0,"audio_tokens":0,"reasoning_tokens":0,"image_tokens":0},` +
		`"input_tokens":12000,"output_tokens":300,` +
		`"input_tokens_details":{"cached_tokens":10500,"text_tokens":12000,"audio_tokens":0,"image_tokens":0}}`
	var usage apicompat.ResponsesUsage
	require.NoError(t, json.Unmarshal([]byte(raw), &usage))
	got := copyOpenAIUsageFromResponsesUsage(&usage)
	require.Equal(t, 12000, got.InputTokens)
	require.Equal(t, 10500, got.CacheReadInputTokens)

	body := []byte(`{"type":"response.completed","response":{"id":"resp_1","usage":` + raw + `}}`)
	parsed, ok := extractOpenAIUsageFromJSONBytes(body)
	require.True(t, ok)
	require.Equal(t, 10500, parsed.CacheReadInputTokens)
}

// Native Gemini: new-api passes the upstream body through unchanged.
func TestNewAPIGeminiNativeUsageKeepsCachedTokens(t *testing.T) {
	body := []byte(`{"candidates":[],"usageMetadata":{"promptTokenCount":12000,"candidatesTokenCount":180,` +
		`"totalTokenCount":12300,"cachedContentTokenCount":10500,"thoughtsTokenCount":120,` +
		`"promptTokensDetails":[{"modality":"TEXT","tokenCount":12000}],` +
		`"cacheTokensDetails":[{"modality":"TEXT","tokenCount":10500}]}}`)
	usage := extractGeminiUsage(body)
	require.NotNil(t, usage)
	require.Equal(t, 1500, usage.InputTokens)
	require.Equal(t, 10500, usage.CacheReadInputTokens)
	require.Equal(t, 300, usage.OutputTokens)
}

// Anthropic Messages, as produced by new-api's OpenAI → Claude usage bridge:
// input_tokens excludes cache reads, which are reported separately.
func TestNewAPIGeminiAnthropicUsageKeepsCachedTokens(t *testing.T) {
	svc := &GatewayService{}
	usage := &ClaudeUsage{}
	svc.parseSSEUsage(`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","usage":{"input_tokens":0,"output_tokens":0}}}`, usage)
	svc.parseSSEUsage(`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"input_tokens":1500,"output_tokens":300,"cache_creation_input_tokens":0,"cache_read_input_tokens":10500}}`, usage)
	require.Equal(t, 1500, usage.InputTokens)
	require.Equal(t, 300, usage.OutputTokens)
	require.Equal(t, 10500, usage.CacheReadInputTokens)
}
