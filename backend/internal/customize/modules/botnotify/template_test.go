package botnotify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testData struct {
	calls int
	err   error
}

func (d *testData) Channels(context.Context) ([]Channel, error) {
	d.calls++
	return []Channel{{GroupName: "A", Model: "model", Status: "operational", LatencyMS: 42, Availability: 99.123}}, d.err
}
func (d *testData) Announcements(context.Context) (string, error) { return "公告 {now}", d.err }
func (d *testData) SiteName(context.Context) (string, error)      { return "测试站", d.err }

func TestTemplateRendering(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 5, 0, 0, time.FixedZone("CST", 8*3600))
	d := &testData{}
	text, err := Render(context.Background(), d, "{site_name}\n{channel_status}\n{channel_status_abnormal}\n{announcements}\n{now}\n{unknown}", now)
	require.NoError(t, err)
	require.Equal(t, 1, d.calls)
	require.Contains(t, text, "10月17日09:05 分组状态\n\nA · model\n"+strings.Repeat("⬜", 10)+"\n延迟 42ms｜七天可用性 99.12%")
	require.Contains(t, text, "公告 {now}\n2026-10-17 09:05:00\n{unknown}")
	d.err = errors.New("unavailable")
	text, err = Render(context.Background(), d, "纯文本", now)
	require.NoError(t, err)
	require.Equal(t, "纯文本", text)
	_, err = Render(context.Background(), d, "{channel_status}", now)
	require.Error(t, err)
}

func TestBlocksChronologyAndLastTen(t *testing.T) {
	base := time.Now()
	points := []Point{}
	for i := 11; i >= 0; i-- {
		status := "operational"
		if i >= 9 {
			status = []string{"degraded", "failed", "error"}[i-9]
		}
		points = append(points, Point{Status: status, CheckedAt: base.Add(time.Duration(i) * time.Second)})
	}
	blocks, bad := Blocks(points)
	require.Equal(t, strings.Repeat("🟩", 7)+"🟨🟥🟥", blocks)
	require.Equal(t, 3, bad)
	text := ChannelStatus([]Channel{{GroupName: "G", Model: "M", Status: "operational", Timeline: points}}, true, base)
	require.Contains(t, text, "G · M ⚠️")
	require.Contains(t, text, "近10次检测 3 次降级/失败")
	blocks, bad = Blocks([]Point{{Status: "unknown"}})
	require.Equal(t, strings.Repeat("⬜", 10), blocks)
	require.Zero(t, bad)
	require.NotContains(t, ChannelStatus([]Channel{{GroupName: "healthy", Status: "operational"}}, true, base), "healthy")
}

func TestRuleMatching(t *testing.T) {
	for _, tc := range []struct {
		kind, keyword, text string
		enabled, want       bool
	}{
		{"contains", "渠道状态", "请查渠道状态", true, true},
		{"exact", "渠道状态", "渠道状态 其他", true, false},
		{"exact", "渠道状态", " 渠道状态 ", true, true},
		{"command", "/status", "/status@MyBot args", true, true},
		{"command", "/status", "/status@OtherBot", true, false},
		{"command", "/status", "/status_more", true, false},
		{"contains", "渠道状态", "渠道状态", false, false},
	} {
		require.Equal(t, tc.want, Matches(Rule{Keyword: tc.keyword, MatchType: tc.kind, Enabled: tc.enabled}, tc.text, "mybot"), tc.text)
	}
}

func TestGroupAddressing(t *testing.T) {
	bot := Bot{ID: 12, Username: "MyBot"}
	require.True(t, Addressed(Message{ChatType: "private"}, bot))
	require.False(t, Addressed(Message{ChatType: "group", Text: "渠道状态 @MyBotSuffix"}, bot))
	require.True(t, Addressed(Message{ChatType: "supergroup", Mentions: []string{"@mybot"}}, bot))
	require.True(t, Addressed(Message{ChatType: "group", ReplyToBotID: 12}, bot))
	require.False(t, Addressed(Message{ChatType: "group", ReplyToBotID: 13}, bot))
	require.False(t, Addressed(Message{ChatType: "channel", Mentions: []string{"@MyBot"}}, bot))
}

func TestValidationAndMasking(t *testing.T) {
	require.Equal(t, "****abcd", MaskToken("123456:abcd"))
	require.Equal(t, "****", MaskToken("abc"))
	require.NoError(t, ValidateConfig(Config{IntervalMinutes: 60, ChatIDs: []string{"-100123", "123"}}))
	require.Error(t, ValidateConfig(Config{IntervalMinutes: 0}))
	require.Error(t, ValidateConfig(Config{IntervalMinutes: 60, ChatIDs: []string{"-"}}))
	require.Error(t, ValidateRule(Rule{Keyword: "", MatchType: "exact"}))
	require.Error(t, ValidateRule(Rule{Keyword: "status@other", MatchType: "command"}))
}
