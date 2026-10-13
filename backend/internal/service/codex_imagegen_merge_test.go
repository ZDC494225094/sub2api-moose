//go:build unit

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCodexImagegenNamespaceDoesNotReceiveHostedBridge(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"responses", `{"model":"gpt-6.1-sol","instructions":"existing instructions","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]}`},
		{"additional_tools", `{"model":"gpt-6.1-sol","instructions":"existing instructions","input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}]}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body map[string]any
			require.NoError(t, json.Unmarshal([]byte(tc.raw), &body))
			require.False(t, ensureOpenAIResponsesImageGenerationTool(body))
			require.False(t, ensureOpenAIResponsesImageGenerationToolChoiceAuto(body))
			require.False(t, applyCodexImageGenerationBridgeInstructions(body))
			require.Equal(t, "existing instructions", body["instructions"])
			require.NotContains(t, body, "tool_choice")
		})
	}
}

func TestCodexImagegenCachedGroupForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, route := range []struct {
		name          string
		policy        string
		primaryActive bool
		wantGroupID   int64
	}{
		{name: "native", policy: multigroupbilling.NativeRouting, primaryActive: true, wantGroupID: 4242},
		{name: "multigroup_cached_primary", policy: multigroupbilling.LegacyRouting, primaryActive: true, wantGroupID: 4242},
		{name: "multigroup_loaded_fallback", policy: multigroupbilling.LegacyRouting, wantGroupID: 4243},
	} {
		for _, lite := range []bool{false, true} {
			for _, passthrough := range []bool{false, true} {
				for _, strip := range []bool{false, true} {
					name := route.name
					if lite {
						name += "/lite"
					} else {
						name += "/responses"
					}
					if passthrough {
						name += "/passthrough"
					} else {
						name += "/managed"
					}
					if strip {
						name += "/strip"
					} else {
						name += "/allow"
					}
					t.Run(name, func(t *testing.T) {
						primaryID := int64(4242)
						primary := &Group{ID: primaryID, Platform: PlatformOpenAI, Status: StatusActive,
							AllowImageGeneration: route.primaryActive, RateMultiplier: 1, ImageRateMultiplier: 1}
						if !route.primaryActive {
							primary.Status = StatusDisabled
						}
						fallback := &Group{ID: 4243, Platform: PlatformOpenAI, Status: StatusActive,
							AllowImageGeneration: true, RateMultiplier: 1, ImageRateMultiplier: 1}
						repo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{4243: fallback}}
						keys := &APIKeyService{groupRepo: repo}
						key := &APIKey{ID: 2424, UserID: 1, Platform: PlatformOpenAI,
							GroupID: &primaryID, Group: primary, GroupIDs: []int64{4242, 4243},
							RoutingPolicy: route.policy, User: &User{ID: 1, Balance: 10}}

						// Exercise the serialized auth cache, not just an in-memory group copy.
						rawSnapshot, err := json.Marshal(keys.snapshotFromAPIKey(context.Background(), key))
						require.NoError(t, err)
						var snapshot APIKeyAuthSnapshot
						require.NoError(t, json.Unmarshal(rawSnapshot, &snapshot))
						cached, hit, err := keys.applyAuthCacheEntry("test-key", &APIKeyAuthCacheEntry{Snapshot: &snapshot})
						require.NoError(t, err)
						require.True(t, hit)
						require.Equal(t, primary.AllowImageGeneration, cached.Group.AllowImageGeneration)
						_, err = keys.SelectUsableGroupForAPIKey(context.Background(), cached, nil)
						require.NoError(t, err)
						require.Equal(t, route.wantGroupID, *cached.GroupID)
						require.True(t, cached.Group.AllowImageGeneration)
						require.Equal(t, []int64{4242, 4243}, cached.GroupIDs)
						require.Equal(t, primaryID, *key.GroupID, "selection must not mutate the original auth key")

						upstream := &httpUpstreamRecorder{resp: &http.Response{
							StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}},
							Body: io.NopCloser(strings.NewReader(`{"id":"resp_imagegen","model":"gpt-6.1-sol","output":[{"type":"function_call","call_id":"call_image","namespace":"image_gen","name":"imagegen","arguments":"{}"}],"usage":{"input_tokens":1,"output_tokens":1}}`)),
						}}
						svc := newOpenAIImageGenerationControlTestService(upstream)
						svc.cfg.Gateway.CodexImageGenerationBridgeEnabled = true
						c, recorder := newOpenAIImageGenerationControlTestContext(true, "Codex Desktop/0.162.0-alpha.17.2")
						c.Request.Header.Set("originator", "Codex Desktop")
						if lite {
							c.Request.Header.Set(responsesLiteHeader, "true")
						}
						c.Set("api_key", cached)
						account := newOpenAIImageGenerationControlTestAccount()
						account.Extra = map[string]any{"openai_passthrough": passthrough}
						if strip {
							account.Extra[featureKeyCodexImageGenerationExplicitToolPolicy] = codexImageGenerationExplicitToolPolicyStrip
						}
						body := []byte(`{"model":"gpt-6.1-sol","stream":false,"tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen","parameters":{"type":"object"}}]},{"type":"function","name":"shell","parameters":{"type":"object"}}],"input":[{"role":"user","content":"draw a giraffe"}],"tool_choice":"auto"}`)
						result, err := svc.Forward(context.Background(), c, account, body)
						require.NoError(t, err)
						require.NotNil(t, result)
						require.Equal(t, http.StatusOK, recorder.Code)
						require.NotNil(t, upstream.lastReq)
						var forwarded map[string]any
						require.NoError(t, json.Unmarshal(upstream.lastBody, &forwarded))
						require.Equal(t, !strip, hasOpenAIImageGenerationTool(forwarded))
						require.False(t, gjson.GetBytes(upstream.lastBody, `tools.#(type=="image_generation")`).Exists(), "a client imagegen declaration must not acquire a duplicate hosted tool")
						require.NotContains(t, gjson.GetBytes(upstream.lastBody, "instructions").String(), codexImageGenerationBridgeMarker)
						if !strip {
							require.Equal(t, "image_gen", gjson.GetBytes(recorder.Body.Bytes(), "output.0.namespace").String())
							require.Equal(t, "imagegen", gjson.GetBytes(recorder.Body.Bytes(), "output.0.name").String())
						}
					})
				}
			}
		}
	}
}

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
