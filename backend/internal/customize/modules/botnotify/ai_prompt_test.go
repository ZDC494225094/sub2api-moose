package botnotify

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSystemPromptCarriesKnowledgeGuideAndContacts(t *testing.T) {
	cfg := DefaultAIConfig(1)
	req := AIRequest{Config: cfg, Context: PublicDTO{
		SiteName:    "麋鹿云",
		ContactInfo: "QQ 1076623975",
		GuideTitle:  "使用手册",
		Guide:       "## Claude Desktop\n打开 CC Switch 切到路由\n![CC Switch 的 Claude Desktop 页](/guide-images/ccsdesktop.jpg)",
	}}
	prompt := BuildSystemPrompt(req)
	require.Contains(t, prompt, "「麋鹿云」")
	require.Contains(t, prompt, "联系方式：QQ 1076623975")
	require.Contains(t, prompt, "https://moosecloud.cc/keys", "default knowledge base")
	require.Contains(t, prompt, "打开 CC Switch 切到路由", "manual content")
	require.Contains(t, prompt, "（配图：CC Switch 的 Claude Desktop 页）")
	require.NotContains(t, prompt, "/guide-images/")
	for _, tool := range AIToolsFor(cfg) {
		require.Contains(t, prompt, tool)
	}
	// Same input, same bytes: upstream prompt caches depend on it.
	require.Equal(t, prompt, BuildSystemPrompt(req))
}

func TestBuildSystemPromptFollowsSwitches(t *testing.T) {
	cfg := DefaultAIConfig(1)
	cfg.AllowRecharge, cfg.AllowUsageStats, cfg.UseUserGuide = false, false, false
	cfg.KnowledgeBase = "自定义资料：退款请联系人工"
	prompt := BuildSystemPrompt(AIRequest{Config: cfg, Context: PublicDTO{Guide: "手册正文"}})
	require.Contains(t, prompt, "自定义资料：退款请联系人工")
	require.NotContains(t, prompt, "https://moosecloud.cc/keys")
	require.NotContains(t, prompt, "手册正文")
	require.NotContains(t, prompt, ToolPlans)
	require.NotContains(t, prompt, ToolCacheStats)
	require.Contains(t, prompt, "不处理充值、支付")
	require.NotContains(t, AIToolsFor(cfg), ToolRechargeInfo)
}

func TestUserTurnTextDescribesImages(t *testing.T) {
	req := AIRequest{Config: DefaultAIConfig(1), Images: []Image{{MIME: "image/png"}}}
	require.Contains(t, UserTurnText(req), "只发送了图片")
	req = AIRequest{Config: DefaultAIConfig(1), Message: Message{Text: "报错了"}, ImageNote: "1 张图片读取失败"}
	require.Equal(t, "报错了\n（1 张图片读取失败）", UserTurnText(req))
}

func TestKnowledgeBaseDefaultIsStoredEmpty(t *testing.T) {
	require.Empty(t, normalizeKnowledgeBase(DefaultKnowledgeBase+"\n"))
	require.Equal(t, "我的资料", normalizeKnowledgeBase(" 我的资料 \r\n"))
	require.Equal(t, DefaultKnowledgeBase, EffectiveKnowledgeBase(AIConfig{}))
	require.Error(t, ValidateAIConfig(func() AIConfig {
		c := DefaultAIConfig(1)
		c.KnowledgeBase = strings.Repeat("字", MaxKnowledgeBaseRunes+1)
		return c
	}()))
	require.Error(t, ValidateAIConfig(func() AIConfig {
		c := DefaultAIConfig(1)
		c.ReasoningEffort = "max"
		return c
	}()))
}

func TestChatTextFlattensMarkdownButKeepsCommands(t *testing.T) {
	got := ChatText("## 解决步骤\n1. **打开** `CC Switch`\n```bash\nexport ANTHROPIC_AUTH_TOKEN=<你的密钥>\n```\n- 重启 Claude Desktop")
	require.Equal(t, "解决步骤\n1. 打开 CC Switch\nexport ANTHROPIC_AUTH_TOKEN=<你的密钥>\n• 重启 Claude Desktop", got)
	require.Equal(t, "__init__ 和 a * b", ChatText("__init__ 和 a * b"))
}

func TestAIErrorCodeAndAdminClassification(t *testing.T) {
	require.Equal(t, "provider_http_402", AIErrorCode(errorString("ai_provider_http_402")))
	require.Equal(t, "provider_unreachable", AIErrorCode(errorString("ai_provider_unreachable: dial tcp")))
	require.Equal(t, "provider_error", AIErrorCode(errorString("boom")))
	require.True(t, aiErrorNeedsAdmin("provider_http_402"))
	require.True(t, aiErrorNeedsAdmin("key_unavailable"))
	require.False(t, aiErrorNeedsAdmin("provider_http_429"))
}

type errorString string

func (e errorString) Error() string { return string(e) }

func TestFixedRepliesNameConfiguredSupport(t *testing.T) {
	dto := PublicDTO{ContactInfo: "QQ 1076623975"}
	require.Contains(t, BusyReply(dto), "人工客服：QQ 1076623975")
	require.Contains(t, UnavailableReply(PublicDTO{}), "联系客服")
	require.Contains(t, AttachmentReply("voice", true), "语音")
	require.Contains(t, AttachmentReply("image", false), "暂时不支持识别图片")
}
