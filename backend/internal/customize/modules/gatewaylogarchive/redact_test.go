package gatewaylogarchive

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func redactForTest(t *testing.T, input string) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, redactJSON(context.Background(), &out, strings.NewReader(input), false, nil))
	require.True(t, json.Valid(out.Bytes()))
	return out.String()
}

func TestRedactionOnlyChangesPersonalValues(t *testing.T) {
	input := `{"request_id":"req-13800138000","model":"gpt-6","name":"lookup_weather","address":"https://api.example.com/v1","n":9007199254740993,"n":1.2300e+05,"escaped":"\u4f60\u597d <>&","code":"const phone = 1; return value + 3;","profile":{"name":"张三","address":"上海市某路1号","email":"alice@example.com","phone":13800138000,"date_of_birth":"1990-01-01"},"tools":[{"name":"phone","input_schema":{"properties":{"email":{"type":"string"},"phone":{"type":"number"}},"required":["email"]}}]}`
	want := strings.NewReplacer(`"张三"`, `"[REDACTED]"`, `"上海市某路1号"`, `"[REDACTED]"`, `"alice@example.com"`, `"[REDACTED]"`, `"phone":13800138000`, `"phone":0`, `"1990-01-01"`, `"[REDACTED]"`).Replace(input)
	require.Equal(t, want, redactForTest(t, input))
}

func TestRedactionTextSSEEmbeddedJSONAndWebSocket(t *testing.T) {
	text := "联系 alice@example.com，手机 13800138000，身份证 11010519491231002X；姓名：张三；银行卡号：6222021234567890123；家庭住址：上海市某路1号；保留 HTTP 200、模型 gpt-6、订单 order-13800138000。"
	sse := "event: message\r\ndata: {\"delta\":{\"text\":\"alice@example.com\"},\"profile\":{\"name\":\"张三\"},\"tokens\":12}\r\n\r\ndata: [DONE]\r\n\r\n"
	input, err := json.Marshal(map[string]any{
		"content":   text,
		"response":  sse,
		"events":    []any{map[string]any{"text": "mobile: +1 (415) 555-2671; unchanged"}},
		"arguments": `{"full_name":"Alice Smith","bank_card_number":"6222021234567890123","city":"Shanghai","amount":42}`,
	})
	require.NoError(t, err)
	result := redactForTest(t, string(input))
	for _, secret := range []string{"alice@example.com", "11010519491231002X", "张三", "6222021234567890123", "上海市某路1号", "Alice Smith", "415"} {
		require.NotContains(t, result, secret)
	}
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &decoded))
	require.Equal(t, strings.NewReplacer("alice@example.com", piiMask, "张三", piiMask).Replace(sse), decoded["response"])
	require.Contains(t, decoded["content"], "保留 HTTP 200、模型 gpt-6、订单 order-13800138000。")
	require.Contains(t, decoded["arguments"], `"city":"Shanghai","amount":42`)
}

func TestRedactionPreservesNonPIIAndMalformedInputFailsClosed(t *testing.T) {
	for _, input := range []string{
		` { "id": "req-123", "count": 13800138000, "name": "tool_name", "format": "json", "timestamp": 1731234567890, "data": null, "ok": true } `,
		`{"content":"\u4f60\u597d: x+y; version 1.2.3; full_name: string; phone = getPhone(); https://example.com", "image":"data:image/png;base64,aGVsbG8="}`,
		`{"code":"npm install package@1.2.3; const phone = getPhone(); type Person = { email: string };"}`,
		`{"content":"email format: user@example.com"}`,
	} {
		if strings.Contains(input, "user@example.com") {
			require.Contains(t, redactForTest(t, input), piiMask)
		} else {
			require.Equal(t, input, redactForTest(t, input))
		}
	}
	for _, input := range []string{`{"email":"alice@example.com"`, `{"n":1,}`, `{"n":oops}`, `{} {}`} {
		var out bytes.Buffer
		require.Error(t, redactJSON(context.Background(), &out, strings.NewReader(input), false, nil))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	require.ErrorIs(t, redactJSON(ctx, &out, strings.NewReader(`{}`), false, nil), context.Canceled)
}

func TestRedactionProtectsOpaqueSignaturesAndToolReferences(t *testing.T) {
	input := `{"signature":"13800138000","thoughtSignature":"alice@example.com","encrypted_content":"13800138000","tool_use_id":"13800138000","call_id":"alice@example.com","content":"联系 alice@example.com 或 13800138000","profile":{"phone":"13800138000"}}`
	want := `{"signature":"13800138000","thoughtSignature":"alice@example.com","encrypted_content":"13800138000","tool_use_id":"13800138000","call_id":"alice@example.com","content":"联系 [REDACTED] 或 [REDACTED]","profile":{"phone":"[REDACTED]"}}`
	require.Equal(t, want, redactForTest(t, input))
	sse := "data: " + input + "\n\n"
	encoded, _ := json.Marshal(map[string]string{"response": sse})
	result := redactForTest(t, string(encoded))
	var decoded map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &decoded))
	require.Equal(t, "data: "+want+"\n\n", decoded["response"])
}
