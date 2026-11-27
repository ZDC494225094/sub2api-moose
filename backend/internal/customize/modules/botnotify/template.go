package botnotify

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var ErrMonitorDisabled = errors.New("渠道监控已关闭")

func Blocks(points []Point) (string, int) {
	points = append([]Point(nil), points...)
	sort.SliceStable(points, func(i, j int) bool { return points[i].CheckedAt.Before(points[j].CheckedAt) })
	if len(points) > StatusBarSlots {
		points = points[len(points)-StatusBarSlots:]
	}
	var out strings.Builder
	out.WriteString(strings.Repeat("·", StatusBarSlots-len(points)))
	bad := 0
	for _, p := range points {
		switch p.Status {
		case "operational":
			out.WriteString("🟩")
		case "degraded":
			out.WriteString("🟨")
			bad++
		case "failed", "error":
			out.WriteString("🟥")
			bad++
		default:
			out.WriteString("·")
		}
	}
	return out.String(), bad
}

func ChannelStatus(channels []Channel, abnormal bool, now time.Time) string {
	var out strings.Builder
	out.WriteString(now.Format("【01月02日 15:04】分组状态"))
	if len(channels) == 0 {
		return out.String() + "\n暂无可播报的分组监控数据"
	}
	shown := 0
	for _, c := range channels {
		if c.Passive != nil {
			if abnormal && c.Status != "degraded" && c.Status != "failed" {
				continue
			}
			shown++
			fmt.Fprintf(&out, "\n\n%s · %s", c.GroupName, c.Passive.Platform)
			if len(c.Timeline) > 0 {
				blocks, _ := Blocks(c.Timeline)
				fmt.Fprintf(&out, "\n%s", blocks)
			}
			fmt.Fprintf(&out, "\n%s", c.Passive.Summary())
			continue
		}
		blocks, bad := Blocks(c.Timeline)
		warn := c.Status != "operational" || bad >= 3
		if abnormal && !warn {
			continue
		}
		shown++
		out.WriteString("\n\n" + c.GroupName + " · " + c.Model)
		if warn {
			out.WriteString(" ⚠️")
		}
		fmt.Fprintf(&out, "\n%s\n延迟 %dms｜七天可用性 %.2f%%", blocks, c.LatencyMS, c.Availability)
		if bad >= 3 {
			fmt.Fprintf(&out, "\n近10次检测中有 %d 次出现波动或失败", bad)
		}
	}
	if abnormal && shown == 0 {
		out.WriteString("\n暂无已确认异常的分组")
	}
	return out.String()
}

func (p PassiveMetrics) Summary() string {
	if p.Success90m == nil {
		return "近90分钟成功率 —"
	}
	return fmt.Sprintf("近90分钟成功率 %.2f%%", *p.Success90m)
}

// FormatAnnouncement renders one announcement as its title plus its body
// flattened from Markdown to plain text (chat platforms get no parse mode).
func FormatAnnouncement(a Announcement) string {
	body := PlainText(a.Content)
	if a.Title == "" {
		return body
	}
	if body == "" {
		return a.Title
	}
	return a.Title + "\n" + body
}

// FormatAnnouncements renders at most limit announcements, newest first.
func FormatAnnouncements(list []Announcement, limit int) string {
	if len(list) > limit {
		list = list[:limit]
	}
	parts := make([]string, 0, len(list))
	for _, a := range list {
		if text := FormatAnnouncement(a); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n")
}

// Render loads each dependency once, only if referenced. Inserted data is never
// interpreted as another template, so announcement text stays literal.
func Render(ctx context.Context, data Data, template string, now time.Time) (string, error) {
	pairs := []string{"{now}", now.Format("2006-01-02 15:04:05")}
	if strings.Contains(template, "{channel_status}") || strings.Contains(template, "{channel_status_abnormal}") {
		channels, err := data.Channels(ctx)
		switch {
		case errors.Is(err, ErrMonitorDisabled):
			pairs = append(pairs, "{channel_status}", ErrMonitorDisabled.Error(), "{channel_status_abnormal}", ErrMonitorDisabled.Error())
		case err != nil:
			return "", err
		default:
			pairs = append(pairs, "{channel_status}", ChannelStatus(channels, false, now), "{channel_status_abnormal}", ChannelStatus(channels, true, now))
		}
	}
	if strings.Contains(template, "{announcements}") {
		list, err := data.Announcements(ctx)
		if err != nil {
			return "", err
		}
		pairs = append(pairs, "{announcements}", FormatAnnouncements(list, 5))
	}
	if strings.Contains(template, "{site_name}") {
		value, err := data.SiteName(ctx)
		if err != nil {
			return "", err
		}
		pairs = append(pairs, "{site_name}", value)
	}
	return strings.NewReplacer(pairs...).Replace(template), nil
}

func Addressed(m Message, bot Bot) bool {
	if m.ChatType == "private" {
		return true
	}
	if m.ChatType != "group" && m.ChatType != "supergroup" {
		return false
	}
	if bot.ID != 0 && m.ReplyToBotID == bot.ID {
		return true
	}
	for _, mention := range m.Mentions {
		if bot.Username != "" && strings.EqualFold(mention, "@"+bot.Username) {
			return true
		}
	}
	return false
}

func Matches(rule Rule, text, username string) bool {
	if !rule.Enabled {
		return false
	}
	switch rule.MatchType {
	case "contains":
		return strings.Contains(text, rule.Keyword)
	case "exact":
		return strings.TrimSpace(text) == rule.Keyword
	case "command":
		fields := strings.Fields(text)
		if len(fields) == 0 {
			return false
		}
		command := strings.SplitN(fields[0], "@", 2)
		if len(command) == 2 && !strings.EqualFold(command[1], username) {
			return false
		}
		return command[0] == "/"+strings.TrimPrefix(rule.Keyword, "/")
	}
	return false
}
