package botnotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
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
func (d *testData) Announcements(context.Context) ([]Announcement, error) {
	return []Announcement{{ID: 1, Content: "公告 {now}"}}, d.err
}
func (d *testData) SiteName(context.Context) (string, error) { return "测试站", d.err }

func TestTemplateRendering(t *testing.T) {
	now := time.Date(2026, 10, 17, 9, 5, 0, 0, time.FixedZone("CST", 8*3600))
	d := &testData{}
	text, err := Render(context.Background(), d, "{site_name}\n{channel_status}\n{channel_status_abnormal}\n{announcements}\n{now}\n{unknown}", now)
	require.NoError(t, err)
	require.Equal(t, 1, d.calls)
	require.Contains(t, text, "【10月17日 09:05】分组状态\n\nA · model\n"+strings.Repeat("·", 10)+"\n延迟 42ms｜七天可用性 99.12%")
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
	require.Contains(t, text, "近10次检测中有 3 次出现波动或失败")
	blocks, bad = Blocks([]Point{{Status: "unknown"}})
	require.Equal(t, strings.Repeat("·", 10), blocks)
	require.Zero(t, bad)
	require.NotContains(t, ChannelStatus([]Channel{{GroupName: "healthy", Status: "operational"}}, true, base), "healthy")
}

func TestPassiveBlocksUseWebsiteScoreBands(t *testing.T) {
	tests := []struct {
		score float64
		block string
	}{
		{-5, "🟥"}, {0, "🟥"}, {10, "🟥"}, {20, "🟥"}, {24.99, "🟥"},
		{25, "🟧"}, {30, "🟧"}, {40, "🟧"}, {44.99, "🟧"},
		{45, "🟨"}, {50, "🟨"}, {60, "🟨"}, {64.99, "🟨"},
		{65, "🟩"}, {70, "🟩"}, {80, "🟩"}, {90, "🟩"}, {100, "🟩"}, {105, "🟩"},
		{math.NaN(), "⬛"},
	}
	for _, tt := range tests {
		p := PassiveMetrics{}
		p.Scores[StatusBarSlots-1] = &tt.score
		require.Equal(t, strings.Repeat("⬛", StatusBarSlots-1)+tt.block, p.blocks(), "score %v", tt.score)
	}
	require.Equal(t, strings.Repeat("⬛", StatusBarSlots), (PassiveMetrics{}).blocks())
}

func TestLongReportKeepsGroupSquaresAndRateInOneMessage(t *testing.T) {
	now := time.Date(2026, 11, 27, 14, 19, 0, 0, time.UTC)
	score, rate := 40.0, 90.52
	group := Channel{GroupName: "分组", Passive: &PassiveMetrics{Platform: "Claude", Success90m: &rate}}
	for i := range group.Passive.Scores {
		group.Passive.Scores[i] = &score
	}
	report := ChannelStatus([]Channel{group}, false, now)
	header, row, ok := strings.Cut(report, "\n\n")
	require.True(t, ok)
	require.Equal(t, "分组 · Claude\n"+strings.Repeat("🟧", 10)+"\n近90分钟成功率 90.52%", row)
	// Place the hard transport limit two squares into the status bar.
	prefix := strings.Repeat("字", 2000-len([]rune(header+"\n\n分组 · Claude\n"))-2)
	chunks := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_, _ = io.WriteString(w, `{"access_token":"test","expires_in":7200}`)
			return
		}
		var body struct{ Text, Content string }
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		chunks <- body.Text + body.Content
		_, _ = io.WriteString(w, `{"ok":true,"result":{}}`)
	}))
	t.Cleanup(server.Close)
	qq := NewQQ()
	qq.Client, qq.APIBase, qq.TokenURL = server.Client(), server.URL, server.URL+"/token"
	tg := &Telegram{Client: server.Client(), BaseURL: server.URL}
	for _, tc := range []struct {
		platform      Platform
		token, target string
	}{
		{qq, `{"app_id":"1","client_secret":"test"}`, "group:123"},
		{tg, "test", "-123"},
	} {
		require.NoError(t, tc.platform.SendMessage(t.Context(), tc.token, tc.target, prefix+report))
		require.Len(t, chunks, 2)
		require.Equal(t, prefix+header+"\n\n", <-chunks)
		require.Equal(t, row, <-chunks)
	}
	require.Equal(t, 2000, messageChunkLength([]rune(strings.Repeat("字", 2100))))
	require.Equal(t, 2000, messageChunkLength([]rune("\n\n"+strings.Repeat("字", 2100))))
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
	require.NoError(t, ValidateConfig("telegram", Config{IntervalMinutes: 60, ChatIDs: []string{"-100123", "123"}}))
	require.Error(t, ValidateConfig("telegram", Config{IntervalMinutes: 0}))
	require.Error(t, ValidateConfig("telegram", Config{IntervalMinutes: 60, ChatIDs: []string{"-"}}))
	require.NoError(t, ValidateConfig("qq", Config{IntervalMinutes: 60, ChatIDs: []string{"group:ABC123", "user:XYZ789"}}))
	require.Error(t, ValidateConfig("qq", Config{IntervalMinutes: 60, ChatIDs: []string{"123"}}))
	require.Error(t, ValidateRule(Rule{Keyword: "", MatchType: "exact"}))
	require.Error(t, ValidateRule(Rule{Keyword: "status@other", MatchType: "command"}))
}
