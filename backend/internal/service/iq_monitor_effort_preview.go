package service

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/tidwall/gjson"
)

// EffortPreview is what the gateway forwards for a Chat Completions request
// carrying reasoning_effort, as used by the iq-monitor extension's target
// editor. Sent is the upstream value ("" = the field is dropped and the
// model default applies); Denied is set when the group policy rejects it.
type EffortPreview struct {
	Sent   string `json:"sent"`
	Denied bool   `json:"denied"`
	Note   string `json:"note,omitempty"`
}

// PreviewChatReasoningEffort runs a requested effort through the same steps a
// /v1/chat/completions request takes: the group's reasoning-effort policy,
// then the platform's protocol conversion. It reuses the real conversion
// functions so the preview follows gateway changes.
func PreviewChatReasoningEffort(group *Group, model, effort string) EffortPreview {
	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "" {
		return EffortPreview{}
	}
	platform := ""
	if group != nil {
		platform = group.Platform
		body, _ := json.Marshal(map[string]any{"model": model, "reasoning_effort": effort})
		policy := *group
		sanitizeGroupReasoningEffortPolicy(&policy)
		mapped, _, err := ApplyReasoningEffortPolicy(body, policy.MaxReasoningEffort, policy.ReasoningEffortMappings, policy.MaxReasoningEffortOverLimit)
		if err != nil {
			return EffortPreview{Denied: true, Note: "分组思考深度策略会拒绝该挡位"}
		}
		effort = gjson.GetBytes(mapped, "reasoning_effort").String()
	}
	switch {
	case platform == PlatformAnthropic || platform == PlatformGemini || platform == PlatformAntigravity:
		anthropic, err := previewAnthropicBody(model, effort)
		if err != nil {
			return EffortPreview{Sent: effort, Note: "转换失败：" + err.Error()}
		}
		if platform != PlatformGemini {
			return EffortPreview{Sent: gjson.GetBytes(anthropic, "output_config.effort").String()}
		}
		intent := parseClaudeThinkingIntent(anthropic)
		switch geminiThinkingParamForModel(model) {
		case geminiThinkingParamLevel:
			if geminiModelNameCarriesThinkingTier(model) {
				return EffortPreview{Note: "模型名已带挡位，不再发送思考参数"}
			}
			return EffortPreview{Sent: geminiThinkingLevelFor(model, intent)}
		case geminiThinkingParamBudget:
			if budget, ok := geminiThinkingBudgetFor(model, intent); ok {
				return EffortPreview{Sent: "budget " + strconv.Itoa(budget)}
			}
			return EffortPreview{Note: "交给模型动态预算"}
		default:
			return EffortPreview{Note: "该模型不接受思考参数"}
		}
	case domain.UsesOpenAIGateway(platform):
		req := &apicompat.ChatCompletionsRequest{Model: model, Messages: previewMessages(), ReasoningEffort: effort}
		converted, err := apicompat.ChatCompletionsToResponses(req)
		if err != nil {
			return EffortPreview{Sent: effort, Note: "转换失败：" + err.Error()}
		}
		if converted.Reasoning == nil {
			return EffortPreview{}
		}
		return EffortPreview{Sent: converted.Reasoning.Effort}
	}
	return EffortPreview{Sent: effort}
}

func previewMessages() []apicompat.ChatMessage {
	content, _ := json.Marshal("x")
	return []apicompat.ChatMessage{{Role: "user", Content: content}}
}

func previewAnthropicBody(model, effort string) ([]byte, error) {
	req := &apicompat.ChatCompletionsRequest{Model: model, Messages: previewMessages(), ReasoningEffort: effort}
	responses, err := apicompat.ChatCompletionsToResponses(req)
	if err != nil {
		return nil, err
	}
	anthropic, err := apicompat.ResponsesToAnthropicRequest(responses)
	if err != nil {
		return nil, err
	}
	return json.Marshal(anthropic)
}
