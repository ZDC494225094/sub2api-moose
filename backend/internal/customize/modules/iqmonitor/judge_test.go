package iqmonitor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJudgeLogicExtractionAndNormalization(t *testing.T) {
	cases := []struct {
		name     string
		reply    string
		accepted []string
		want     bool
	}{
		{"answer line", "推理……\n答案：42", []string{"42"}, true},
		{"full width colon and digits", "答案：４２", []string{"42"}, true},
		{"english answer", "Reasoning...\nAnswer: Paris", []string{"paris"}, true},
		{"bold markdown", "**答案：** 7", []string{"7"}, true},
		{"boxed", "所以结果是 \\boxed{12}", []string{"12"}, true},
		{"nested boxed", "\\boxed{\\frac{1}{2}}", []string{"0.5"}, true},
		{"decimal forms", "答案：0.50", []string{"1/2"}, true},
		{"trailing unit", "答案：15 个", []string{"15"}, true},
		{"accepted carries unit", "答案：15", []string{"15个"}, true},
		{"percent", "答案：30%", []string{"30%"}, true},
		{"thousands separator", "答案：1,000", []string{"1000"}, true},
		{"variable lhs", "答案：x = 3", []string{"3"}, true},
		{"value on next line", "答案：\n9", []string{"9"}, true},
		{"last answer line wins", "答案：3\n更正：\n答案：4", []string{"4"}, true},
		{"think block ignored", "<think>答案：1</think>\n答案：2", []string{"1"}, false},
		{"substring of longer number", "答案：123", []string{"12"}, false},
		{"number inside longer number", "答案：12", []string{"123"}, false},
		{"two numbers not a unit", "答案：12或13", []string{"12"}, false},
		{"wrong", "答案：5", []string{"6"}, false},
		{"no answer line", "我认为是 42", []string{"42"}, false},
		{"punctuation and case", "Answer: \"Blue.\"", []string{"blue"}, true},
		{"one of many accepted", "答案：二", []string{"2", "二"}, true},
		{"different units", "答案：5cm", []string{"5m"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := JudgeLogic(tc.reply, tc.accepted)
			require.Equal(t, tc.want, ok)
		})
	}
}

func TestJudgeDrawing(t *testing.T) {
	good := `<svg viewBox="0 0 100 100"><circle cx="50" cy="50" r="10"><animate attributeName="r" values="10;20;10" dur="2s" repeatCount="indefinite"/></circle></svg>`
	cases := []struct {
		name      string
		reply     string
		truncated bool
		want      bool
	}{
		{"bare svg", good, false, true},
		{"in fenced html with prose", "这是代码：\n```html\n<!DOCTYPE html><html><body>" + good + "</body></html>\n```\n说明文字", false, true},
		{"path", `<svg viewBox="0 0 10 10"><path d="M0 0L10 10"/></svg>`, false, true},
		{"text only", `<svg viewBox="0 0 10 10"><text x="1" y="5">Hi</text></svg>`, false, true},
		{"empty reply", "  ", false, false},
		{"refusal", "抱歉，我无法完成这个请求。", false, false},
		{"unclosed svg", `<svg viewBox="0 0 10 10"><circle r="3"/>`, false, false},
		{"empty svg", `<svg viewBox="0 0 10 10"></svg>`, false, false},
		{"zero size shapes", `<svg viewBox="0 0 10 10"><rect width="0" height="5"/><circle r="0"/></svg>`, false, false},
		{"truncated", good, true, false},
		{"canvas only", `<html><canvas id="c"></canvas></html>`, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ok, reason := JudgeDrawing(tc.reply, tc.truncated)
			require.Equal(t, tc.want, ok, reason)
			if !ok {
				require.NotEmpty(t, reason)
			}
		})
	}
}

func TestPreviewHTML(t *testing.T) {
	require.Equal(t, `<svg viewBox="0 0 1 1"><rect width="1" height="1"/></svg>`,
		PreviewHTML("看这里\n```svg\n<svg viewBox=\"0 0 1 1\"><rect width=\"1\" height=\"1\"/></svg>\n```"))
	require.Equal(t, `<!DOCTYPE html><html><svg></svg></html>`, PreviewHTML("前言 <!DOCTYPE html><html><svg></svg></html> 后记"))
	require.Empty(t, PreviewHTML("没有图"))
}

func TestMaskNameAndParseReview(t *testing.T) {
	require.Equal(t, "h**g", MaskName("hung"))
	require.Equal(t, "h****o", MaskName("helloworldfoo"[:0]+"hellooooo"))
	require.Equal(t, "张*", MaskName("张三"))
	require.Equal(t, "a*", MaskName("a"))
	require.Equal(t, "匿名用户", MaskName(""))

	ok, reason, err := ParseReview("```json\n{\"approved\": false, \"reason\": \"涉及真实人物\"}\n```")
	require.NoError(t, err)
	require.False(t, ok)
	require.Equal(t, "涉及真实人物", reason)
	ok, reason, err = ParseReview(`{"approved": true, "reason": "无"}`)
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, reason)
	_, _, err = ParseReview("可以通过")
	require.Error(t, err, "a reply without a verdict is retried, never approved")
	_, _, err = ParseReview(`{"reason": "x"}`)
	require.Error(t, err)
}

func TestEffortLevels(t *testing.T) {
	require.Equal(t, []string{"low", "medium", "high", "xhigh", "max"}, EffortLevels("anthropic", "claude-opus-5-5"))
	require.Equal(t, []string{"low", "medium", "high"}, EffortLevels("anthropic", "claude-opus-4-5"))
	require.Equal(t, []string{"minimal", "low", "medium", "high"}, EffortLevels("gemini", "gemini-3-pro"))
	require.Equal(t, otherEfforts, EffortLevels("openai", "gpt-6"))
	require.True(t, validEffort("anthropic", "claude-opus-4-5", ""))
	require.False(t, validEffort("anthropic", "claude-opus-4-5", "max"))
}

func TestConfigValidateAndHealth(t *testing.T) {
	cfg := DefaultConfig()
	require.NoError(t, cfg.Validate())
	require.Equal(t, KindLogic, cfg.PrimaryKind())
	cfg.Logic.Enabled = false
	require.Equal(t, KindDrawing, cfg.PrimaryKind())
	require.Equal(t, HealthNormal, cfg.Health(95, 10))
	require.Equal(t, HealthUnstable, cfg.Health(80, 10))
	require.Equal(t, HealthDegraded, cfg.Health(79.9, 10))
	require.Equal(t, HealthNoData, cfg.Health(0, 0))
	bad := DefaultConfig()
	bad.IntervalMinutes = 5
	require.Error(t, bad.Validate())
	bad = DefaultConfig()
	bad.Logic.RetryWrong = 4
	require.Error(t, bad.Validate())
	bad = DefaultConfig()
	bad.Thresholds = ThresholdConfig{Normal: 80, Unstable: 90}
	require.Error(t, bad.Validate())
	bad = DefaultConfig()
	bad.Drawing.PromptTemplate = "no placeholder"
	require.Error(t, bad.Validate())
}
