package botnotify

import (
	"regexp"
	"strings"
)

var (
	mdImage     = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	mdLink      = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	mdBoldStar  = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdBoldUnder = regexp.MustCompile(`__([^_]+)__`)
	mdItalic    = regexp.MustCompile(`\*([^*]+)\*`)
	mdStrike    = regexp.MustCompile(`~~([^~]+)~~`)
	mdCodeSpan  = regexp.MustCompile("`([^`]+)`")
	mdHeading   = regexp.MustCompile(`(?m)^[ \t]{0,3}#{1,6}[ \t]+`)
	mdQuote     = regexp.MustCompile(`(?m)^[ \t]*>[ \t]?`)
	mdRule      = regexp.MustCompile(`(?m)^[ \t]*(-{3,}|\*{3,}|_{3,})[ \t]*$`)
	mdBullet    = regexp.MustCompile(`(?m)^([ \t]*)[-*+][ \t]+`)
	mdHTMLTag   = regexp.MustCompile(`<[^>]+>`)
	mdBlankRun  = regexp.MustCompile(`\n{3,}`)
	mdTrailWS   = regexp.MustCompile(`(?m)[ \t]+$`)
)

// PlainText converts Markdown (the format announcement bodies are written in)
// into clean plain text for chat platforms. Telegram/QQ messages are sent
// without a parse mode, so raw Markdown markers would show up verbatim;
// structure (links, headings, emphasis) is flattened while the information
// (link targets included) is preserved.
func PlainText(md string) string {
	s := strings.ReplaceAll(md, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	// Fenced code blocks keep their content, lose the fences.
	s = fenceToText(s)
	s = mdImage.ReplaceAllString(s, "$1")
	s = mdLink.ReplaceAllStringFunc(s, func(m string) string {
		parts := mdLink.FindStringSubmatch(m)
		if parts[1] == parts[2] {
			return parts[2]
		}
		return parts[1] + " " + parts[2]
	})
	s = mdBoldStar.ReplaceAllString(s, "$1")
	s = mdBoldUnder.ReplaceAllString(s, "$1")
	s = mdStrike.ReplaceAllString(s, "$1")
	s = mdItalic.ReplaceAllString(s, "$1")
	s = mdCodeSpan.ReplaceAllString(s, "$1")
	s = mdHeading.ReplaceAllString(s, "")
	s = mdQuote.ReplaceAllString(s, "")
	s = mdRule.ReplaceAllString(s, "")
	s = mdBullet.ReplaceAllString(s, "$1• ")
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = mdHTMLTag.ReplaceAllString(s, "")
	s = htmlEntities.Replace(s)
	s = mdTrailWS.ReplaceAllString(s, "")
	s = mdBlankRun.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

var htmlEntities = strings.NewReplacer(
	"&nbsp;", " ",
	"&lt;", "<",
	"&gt;", ">",
	"&quot;", "\"",
	"&#39;", "'",
	"&amp;", "&",
)

func fenceToText(s string) string {
	if !strings.Contains(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		_ = inFence
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
