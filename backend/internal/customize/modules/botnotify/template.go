package botnotify

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

func Blocks(points []Point) (string, int) {
	points = append([]Point(nil), points...)
	sort.SliceStable(points, func(i, j int) bool { return points[i].CheckedAt.Before(points[j].CheckedAt) })
	if len(points) > 10 {
		points = points[len(points)-10:]
	}
	var out strings.Builder
	for i := len(points); i < 10; i++ {
		out.WriteString("⬜")
	}
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
			out.WriteString("⬜")
		}
	}
	return out.String(), bad
}

func ChannelStatus(channels []Channel, abnormal bool, now time.Time) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%d月%d日%s 分组状态", now.Month(), now.Day(), now.Format("15:04"))
	for _, c := range channels {
		blocks, bad := Blocks(c.Timeline)
		warn := c.Status != "operational" || bad >= 3
		if abnormal && !warn {
			continue
		}
		out.WriteString("\n\n" + c.GroupName + " · " + c.Model)
		if warn {
			out.WriteString(" ⚠️")
		}
		fmt.Fprintf(&out, "\n%s\n延迟 %dms｜七天可用性 %.2f%%", blocks, c.LatencyMS, c.Availability)
		if bad >= 3 {
			fmt.Fprintf(&out, "\n近10次检测 %d 次降级/失败", bad)
		}
	}
	return out.String()
}

// Render loads each dependency once, only if referenced. Inserted data is never
// interpreted as another template, so announcement text stays literal.
func Render(ctx context.Context, data Data, template string, now time.Time) (string, error) {
	pairs := []string{"{now}", now.Format("2006-01-02 15:04:05")}
	if strings.Contains(template, "{channel_status}") || strings.Contains(template, "{channel_status_abnormal}") {
		channels, err := data.Channels(ctx)
		if err != nil {
			return "", err
		}
		pairs = append(pairs, "{channel_status}", ChannelStatus(channels, false, now), "{channel_status_abnormal}", ChannelStatus(channels, true, now))
	}
	if strings.Contains(template, "{announcements}") {
		value, err := data.Announcements(ctx)
		if err != nil {
			return "", err
		}
		pairs = append(pairs, "{announcements}", value)
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
