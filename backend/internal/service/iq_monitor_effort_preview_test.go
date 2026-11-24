package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreviewChatReasoningEffort(t *testing.T) {
	anthropic := &Group{Platform: PlatformAnthropic}
	require.Equal(t, "max", PreviewChatReasoningEffort(anthropic, "claude-opus-4-6", "xhigh").Sent, "Chat Completions xhigh is forwarded to Claude as max")
	require.Equal(t, "high", PreviewChatReasoningEffort(anthropic, "claude-opus-4-6", "high").Sent)
	require.Empty(t, PreviewChatReasoningEffort(anthropic, "claude-opus-4-6", "").Sent)

	capped := &Group{Platform: PlatformOpenAI, MaxReasoningEffort: "medium"}
	require.Equal(t, "medium", PreviewChatReasoningEffort(capped, "gpt-5.2", "high").Sent, "group ceiling downgrades")
	denied := &Group{Platform: PlatformOpenAI, MaxReasoningEffort: "medium", MaxReasoningEffortOverLimit: ReasoningEffortOverLimitDeny}
	require.True(t, PreviewChatReasoningEffort(denied, "gpt-5.2", "high").Denied)
	mapped := &Group{Platform: PlatformOpenAI, ReasoningEffortMappings: []ReasoningEffortMapping{{From: "low", To: "high"}}}
	require.Equal(t, "high", PreviewChatReasoningEffort(mapped, "gpt-5.2", "low").Sent)

	gemini := &Group{Platform: PlatformGemini}
	require.Equal(t, "high", PreviewChatReasoningEffort(gemini, "gemini-3-pro", "medium").Sent, "Gemini 3 Pro has no medium level")
}
