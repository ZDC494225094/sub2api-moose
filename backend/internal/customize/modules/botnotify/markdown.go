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
	// Tables become per-row key/value blocks before inline cleanup runs, so
	// cell text still gets its emphasis/links flattened afterwards.
	s = tablesToText(s)
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

// tablesToText rewrites Markdown tables as readable chat text: each data row
// becomes a block of "column：value" lines, rows separated by a blank line.
// Chat clients render no tables, so the raw pipe layout is pure noise.
func tablesToText(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if isTableRow(lines[i]) && i+1 < len(lines) && isTableSeparator(lines[i+1]) {
			header := splitTableRow(lines[i])
			i += 2
			for i < len(lines) && isTableRow(lines[i]) {
				cells := splitTableRow(lines[i])
				var block []string
				for c, cell := range cells {
					if cell == "" {
						continue
					}
					name := ""
					if c < len(header) {
						name = header[c]
					}
					if name != "" && name != cell {
						block = append(block, name+"："+cell)
					} else {
						block = append(block, cell)
					}
				}
				if len(block) > 0 {
					out = append(out, strings.Join(block, "\n"), "")
				}
				i++
			}
			continue
		}
		out = append(out, lines[i])
		i++
	}
	return strings.Join(out, "\n")
}

func isTableRow(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "|") && strings.Count(t, "|") >= 2
}

func isTableSeparator(line string) bool {
	if !isTableRow(line) {
		return false
	}
	cells := splitTableRow(line)
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		if cell == "" {
			return false
		}
		for _, r := range cell {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

func splitTableRow(line string) []string {
	t := strings.TrimSpace(line)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

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
