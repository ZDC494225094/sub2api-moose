package wiring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAIStreamBodyAssemblesContent(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"你好\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"，充值\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{}}]}\n\n" +
		"data: [DONE]\n")
	msg, ok := parseAIStreamBody(body)
	require.True(t, ok)
	require.Equal(t, "你好，充值", msg.Content)
	require.Empty(t, msg.ToolCalls)
}

func TestParseAIStreamBodyMergesFragmentedToolCalls(t *testing.T) {
	body := []byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"function\":{\"name\":\"query_groups\",\"arguments\":\"\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"group\\\":\"}}]}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"codex\\\"}\"}}]}}]}\n\n" +
		"data: [DONE]\n")
	msg, ok := parseAIStreamBody(body)
	require.True(t, ok)
	require.Len(t, msg.ToolCalls, 1)
	require.Equal(t, "call_1", msg.ToolCalls[0].ID)
	require.Equal(t, "query_groups", msg.ToolCalls[0].Function.Name)
	require.Equal(t, `{"group":"codex"}`, msg.ToolCalls[0].Function.Arguments)
}

func TestParseAIStreamBodyRejectsPlainJSON(t *testing.T) {
	_, ok := parseAIStreamBody([]byte(`{"choices":[{"message":{"content":"hi"}}]}`))
	require.False(t, ok)
}
