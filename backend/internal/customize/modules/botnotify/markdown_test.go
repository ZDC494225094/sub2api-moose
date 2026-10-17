package botnotify

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPlainTextFlattensMarkdown(t *testing.T) {
	in := "## 当前 各 gpt 分组已上线 GPT-6.1-Sol 模型\n\n**重点**：价格 *下调*，详见 [价格表](https://example.com/prices)。\n\n- 第一项\n- 第二项\n\n> 引用一行\n\n```\ncode_line\n```\n\n![示意图](https://example.com/a.png)\n\n---\n\n结束。"
	want := "当前 各 gpt 分组已上线 GPT-6.1-Sol 模型\n\n重点：价格 下调，详见 价格表 https://example.com/prices。\n\n• 第一项\n• 第二项\n\n引用一行\n\ncode_line\n\n示意图\n\n结束。"
	require.Equal(t, want, PlainText(in))
}

func TestPlainTextKeepsPlainTextUntouched(t *testing.T) {
	require.Equal(t, "普通公告\n第二行", PlainText("普通公告\n第二行"))
	require.Equal(t, "3 * 5 = 15", PlainText("3 * 5 = 15"))
	require.Equal(t, "", PlainText("  \n\n"))
}

func TestFormatAnnouncement(t *testing.T) {
	require.Equal(t, "标题\n正文", FormatAnnouncement(Announcement{Title: "标题", Content: "**正文**"}))
	require.Equal(t, "只有正文", FormatAnnouncement(Announcement{Content: "只有正文"}))
	require.Equal(t, "只有标题", FormatAnnouncement(Announcement{Title: "只有标题"}))
}
