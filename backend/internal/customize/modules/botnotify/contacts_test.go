package botnotify

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContactEntriesSupportSeveralNamedEntries(t *testing.T) {
	require.Equal(t, []string{"QQ群 987654"}, contactEntries("QQ群 987654"), "legacy single value stays one entry")
	require.Equal(t, []string{"一群 123", "二群 456", "789"}, contactEntries("一群 | 123\r\n\n二群 | 456\n789\n"))
	require.Equal(t, []string{"https://x.example/a|b"}, contactEntries("https://x.example/a|b"), "a bare pipe inside a URL is not a separator")
	many := strings.Repeat("v\n", 20)
	require.Len(t, contactEntries(many), maxContactEntries)
}

func TestSupportTextsListEveryContactEntry(t *testing.T) {
	dto := PublicDTO{AfterSalesGroup: "一群 | 123\n二群 | 456", CustomerServiceLink: "企业微信 | https://work.example/kf\nhttps://t.me/moose"}
	lines := contactLines(dto)
	require.Contains(t, lines, "- 售后群：一群 123；二群 456")
	require.Contains(t, lines, "- 在线客服：企业微信 https://work.example/kf；https://t.me/moose")
	hint := supportHint(dto)
	require.Contains(t, hint, "售后群 一群 123；售后群 二群 456")
	require.Contains(t, hint, "https://t.me/moose")
}
