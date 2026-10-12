//go:build unit

package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCodexImagegenCrossProtocolRoundTrip(t *testing.T) {
	for _, lite := range []bool{false, true} {
		name := "responses"
		if lite {
			name = "responses_lite"
		}
		t.Run(name, func(t *testing.T) {
			tool := map[string]any{"type": "namespace", "name": "image_gen", "tools": []any{
				map[string]any{"type": "function", "name": "imagegen", "parameters": map[string]any{"type": "object"}},
			}}
			input := []any{map[string]any{"type": "function_call", "call_id": "call_image", "namespace": "image_gen", "name": "imagegen", "arguments": "{}"}}
			body := map[string]any{"model": "claude-fable-5", "input": input}
			if lite {
				body["input"] = append([]any{map[string]any{"type": "additional_tools", "tools": []any{tool}}}, input...)
			} else {
				body["tools"] = []any{tool}
			}
			raw, err := json.Marshal(body)
			require.NoError(t, err)
			adapted, mapping, err := adaptResponsesClientToolsForAnthropic(raw)
			require.NoError(t, err)
			require.Equal(t, "image_gen__imagegen", gjson.GetBytes(adapted, "tools.0.name").String())
			require.Equal(t, "image_gen__imagegen", gjson.GetBytes(adapted, "input.0.name").String())
			for _, response := range []string{
				`{"type":"response.output_item.done","item":{"type":"function_call","call_id":"call_image","name":"image_gen__imagegen","arguments":"{}"}}`,
				`{"output":[{"type":"function_call","call_id":"call_image","name":"image_gen__imagegen","arguments":"{}"}]}`,
			} {
				restored, changed, err := apicompat.RestoreResponsesNamespaceCalls([]byte(response), mapping.NamespaceTools)
				require.NoError(t, err)
				require.True(t, changed)
				path := "item"
				if gjson.Get(response, "output").Exists() {
					path = "output.0"
				}
				require.Equal(t, "image_gen", gjson.GetBytes(restored, path+".namespace").String())
				require.Equal(t, "imagegen", gjson.GetBytes(restored, path+".name").String())
				require.Equal(t, "call_image", gjson.GetBytes(restored, path+".call_id").String())
			}
		})
	}
}
