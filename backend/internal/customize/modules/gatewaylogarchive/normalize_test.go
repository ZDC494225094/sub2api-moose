package gatewaylogarchive

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func anthropicToolStream() string {
	events := []string{
		`{"type":"message_start","message":{"id":"msg-1","role":"assistant","model":"claude","content":[]}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"reasoning text"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"13800138000"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"tool-1","name":"lookup","input":{}}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"n\":9007199"}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"254740993}"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":12}}`,
		`{"type":"message_stop"}`,
	}
	return "data: " + strings.Join(events, "\r\n\r\ndata: ") + "\r\n\r\n"
}

func analyzeFixture(t *testing.T, request string, response any, stream bool) exchangeAnalysis {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"status": 200, "request": json.RawMessage(request), "response": response, "stream": stream})
	require.NoError(t, err)
	return analyzeExchange(raw)
}

func TestAnalysisReassemblesAnthropicThinkingSignatureAndToolArguments(t *testing.T) {
	a := analyzeFixture(t, `{"messages":[{"role":"user","content":"run"}]}`, anthropicToolStream(), true)
	require.Equal(t, "anthropic", a.Protocol)
	require.True(t, a.StreamComplete)
	require.Equal(t, 1, a.AssistantTurns)
	require.Equal(t, 1, a.Signatures)
	require.Equal(t, 1, a.ThinkingBlocks)
	require.Equal(t, []toolCallInfo{{ID: "tool-1", Name: "lookup", Valid: true}}, a.ToolCalls)
	require.Equal(t, "sig-13800138000", gjson.GetBytes(a.Normalized, "content.0.signature").String())
	require.Equal(t, "9007199254740993", gjson.GetBytes(a.Normalized, "content.1.input.n").Raw)
	require.Empty(t, a.Warnings)
}

func TestAnalysisDoesNotInventToolCallsOrSignaturesForBrokenStreams(t *testing.T) {
	broken := strings.ReplaceAll(anthropicToolStream(), `"partial_json":"254740993}"`, `"partial_json":"BROKEN"`)
	broken = strings.ReplaceAll(broken, `data: {"type":"message_stop"}`, "")
	a := analyzeFixture(t, `{}`, broken, true)
	require.False(t, a.StreamComplete)
	require.Contains(t, a.Warnings, "invalid_tool_arguments")
	require.Contains(t, a.Warnings, "stream_terminal_missing")
	require.False(t, a.ToolCalls[0].Valid, "invalid partial JSON must not be replaced by an empty valid object")
	a = analyzeFixture(t, `{}`, json.RawMessage(`{"role":"assistant","content":[{"type":"thinking","thinking":"known text"},{"type":"text","text":"answer"}]}`), false)
	require.Zero(t, a.Signatures)
	require.Empty(t, a.ToolCalls)
	require.Contains(t, a.Warnings, "thinking_without_signature")
}

func TestAnalysisChatCompletionsHandlesParallelCallsAndLongSSELines(t *testing.T) {
	text := strings.Repeat("text", 20000)
	stream := "data: " + string(rawJSON(map[string]any{"id": "chat-1", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": text, "reasoning_content": "reason"}}}})) + "\n\n" +
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"c1","type":"function","function":{"name":"first","arguments":"{\"x\":"}},{"index":1,"id":"c2","type":"function","function":{"name":"second","arguments":"{\"b\":true}"}}]}}]}` + "\n\n" +
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]},"finish_reason":"tool_calls"}]}` + "\n\ndata: [DONE]\n\n"
	a := analyzeFixture(t, `{}`, stream, true)
	require.Equal(t, "chat_completions", a.Protocol)
	require.Len(t, a.ToolCalls, 2)
	require.True(t, a.ToolCalls[0].Valid)
	require.True(t, a.ToolCalls[1].Valid)
	require.Equal(t, text, gjson.GetBytes(a.Normalized, "choices.0.message.content").String())
	require.Equal(t, 1, a.AssistantTurns)
	require.True(t, a.StreamComplete)
	require.Zero(t, a.Signatures, "OpenAI reasoning_content is not an Anthropic signed thinking block")
}

func TestAnalysisResponsesSSEAndWebSocketUseFinalOutput(t *testing.T) {
	final := `{"type":"response.completed","response":{"id":"resp-2","status":"completed","output":[{"type":"reasoning","encrypted_content":"opaque-13800138000"},{"type":"function_call","call_id":"c2","name":"run","arguments":"{\"cmd\":\"ls\"}"}]}}`
	sse := "data: " + final + "\n\n"
	a := analyzeFixture(t, `{"previous_response_id":"resp-1","input":[{"type":"function_call_output","call_id":"c1","output":"ok"}]}`, sse, true)
	require.Equal(t, "responses", a.Protocol)
	require.Equal(t, "resp-1", a.PreviousResponseID)
	require.Equal(t, "resp-2", a.ResponseID)
	require.Equal(t, 1, a.Signatures)
	require.Equal(t, []string{"c1"}, a.ToolResults)
	require.True(t, a.ToolCalls[0].Valid)
	raw := `{"transport":"websocket","request":{},"response":[` + final + `]}`
	ws := analyzeExchange([]byte(raw))
	require.True(t, ws.StreamComplete)
	require.Equal(t, a.Normalized, ws.Normalized)
}

func TestAnalysisGeminiPreservesNativeSignatureAndFunctionCall(t *testing.T) {
	stream := "data: " + `{"candidates":[{"index":0,"content":{"parts":[{"text":"think","thought":true,"thoughtSignature":"sig"},{"functionCall":{"name":"lookup","args":{"city":"Paris"}}}]},"finishReason":"STOP"}]}` + "\n\n"
	a := analyzeFixture(t, `{}`, stream, true)
	require.Equal(t, "gemini", a.Protocol)
	require.Equal(t, 1, a.Signatures)
	require.True(t, a.ToolCalls[0].Valid, "Gemini does not require Anthropic-style tool_use IDs")
	require.True(t, a.StreamComplete)
	require.Equal(t, "sig", gjson.GetBytes(a.Normalized, "candidates.0.content.parts.0.thoughtSignature").String())
}

func TestAnalysisReportsCaptureAndModelTruncationSeparately(t *testing.T) {
	raw := `{"request_truncated":true,"request":"partial","response":{"choices":[{"message":{"role":"assistant","content":"partial"},"finish_reason":"length"}]}}`
	a := analyzeExchange([]byte(raw))
	require.True(t, a.CaptureTruncated)
	require.Contains(t, a.Warnings, "capture_truncated")
	require.Contains(t, a.Warnings, "output_token_limit")
	require.Contains(t, a.Warnings, "request_not_structured")
	require.False(t, a.StreamComplete)
}

func TestResponsesReconstructsDeltasWithoutFinalSnapshot(t *testing.T) {
	events := []string{
		`{"type":"response.created","response":{"id":"r","output":[]}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"type":"reasoning","summary":[],"encrypted_content":"opaque"}}`,
		`{"type":"response.reasoning_summary_part.added","output_index":0,"summary_index":0,"part":{"type":"summary_text","text":""}}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"first "}`,
		`{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"second"}`,
		`{"type":"response.output_item.added","output_index":1,"item":{"type":"function_call","call_id":"c","name":"run","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"x\":"}`,
		`{"type":"response.function_call_arguments.delta","output_index":1,"delta":"1}"}`,
		`{"type":"response.output_item.added","output_index":2,"item":{"type":"message","role":"assistant","content":[]}}`,
		`{"type":"response.content_part.added","output_index":2,"content_index":0,"part":{"type":"output_text","text":""}}`,
		`{"type":"response.output_text.delta","output_index":2,"content_index":0,"delta":"hello "}`,
		`{"type":"response.output_text.delta","output_index":2,"content_index":0,"delta":"world"}`,
		`{"type":"response.completed","response":{"id":"r","status":"completed"}}`,
	}
	a := analyzeFixture(t, `{}`, "data: "+strings.Join(events, "\n\ndata: ")+"\n\n", true)
	require.True(t, a.StreamComplete)
	require.Equal(t, "first second", gjson.GetBytes(a.Normalized, "output.0.summary.0.text").String())
	require.Equal(t, "{\"x\":1}", gjson.GetBytes(a.Normalized, "output.1.arguments").String())
	require.Equal(t, "hello world", gjson.GetBytes(a.Normalized, "output.2.content.0.text").String())
	require.True(t, a.ToolCalls[0].Valid)
	require.Equal(t, 1, a.Signatures)
}
