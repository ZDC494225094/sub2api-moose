package iqmonitor

import (
	"math/big"
	"regexp"
	"strings"
	"unicode"
)

var (
	thinkBlock   = regexp.MustCompile(`(?is)<think(?:ing)?>.*?</think(?:ing)?>`)
	answerLine   = regexp.MustCompile(`(?i)^\s*(?:[#>*\-\s]*)(?:\*\*|__)?\s*(?:最终答案|答案|final\s+answer|answer)\s*(?:\*\*|__)?\s*[:：]\s*(.*)$`)
	latexWrapper = regexp.MustCompile(`\\(?:text|textbf|mathrm|mathbf|operatorname)\s*\{([^{}]*)\}`)
	variableLHS  = regexp.MustCompile(`^[a-z]\s*=\s*(.+)$`)
	numberPrefix = regexp.MustCompile(`^-?(?:\d+(?:\.\d*)?|\.\d+)(?:/\d+)?`)
)

// stripThinking removes reasoning blocks some upstreams inline in content.
func stripThinking(s string) string { return thinkBlock.ReplaceAllString(s, "") }

// ExtractAnswers returns the candidates a reply is judged on: the value of the
// last "答案：" / "Answer:" line and the last \boxed{} value.
func ExtractAnswers(reply string) []string {
	text := stripThinking(reply)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var out []string
	for i := len(lines) - 1; i >= 0; i-- {
		m := answerLine.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		value := strings.TrimSpace(m[1])
		// "答案：" alone on its line puts the value on the next line.
		for j := i + 1; value == "" && j < len(lines); j++ {
			value = strings.TrimSpace(lines[j])
		}
		if value != "" {
			out = append(out, value)
		}
		break
	}
	if boxed := lastBoxed(text); boxed != "" {
		out = append(out, boxed)
	}
	return out
}

// lastBoxed returns the content of the last \boxed{...}, honoring nested braces.
func lastBoxed(s string) string {
	idx := strings.LastIndex(s, `\boxed{`)
	if idx < 0 {
		return ""
	}
	start := idx + len(`\boxed{`)
	depth := 1
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return strings.TrimSpace(s[start:i])
			}
		}
	}
	return ""
}

// toHalfWidth folds full-width ASCII and the ideographic space.
func toHalfWidth(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == 0x3000:
			r = ' '
		case r >= 0xFF01 && r <= 0xFF5E:
			r -= 0xFEE0
		case r == '。':
			r = '.'
		case r == '－' || r == '−' || r == '–' || r == '—':
			r = '-'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// dropped punctuation; '.', '-' and '/' survive for decimals, signs and fractions.
const answerPunct = ",，、;；:：!！?？\"'“”‘’「」『』()（）[]【】{}<>《》*`~_$|"

// NormalizeAnswer canonicalizes an answer: width, case, LaTeX wrappers,
// punctuation, whitespace and number formatting.
func NormalizeAnswer(s string) string {
	s = toHalfWidth(strings.TrimSpace(s))
	s = strings.ToLower(s)
	for i := 0; i < 3; i++ {
		s = latexWrapper.ReplaceAllString(s, "$1")
	}
	s = strings.NewReplacer(`\%`, "%", `\,`, "", `\;`, "", `\!`, "", `\ `, "", `\dfrac`, `\frac`).Replace(s)
	s = fracToSlash(s)
	// Thousands separators: "1,000" -> "1000" before punctuation removal
	// would also do this, but only between digit groups is intended.
	var b strings.Builder
	for _, r := range s {
		if unicode.IsSpace(r) || strings.ContainsRune(answerPunct, r) {
			continue
		}
		b.WriteRune(r)
	}
	s = strings.Trim(b.String(), ".")
	if m := variableLHS.FindStringSubmatch(s); m != nil {
		s = m[1]
	}
	if num, ok := canonicalNumber(s); ok {
		return num
	}
	return s
}

// fracToSlash rewrites \frac{a}{b} as a/b.
var latexFrac = regexp.MustCompile(`\\frac\s*\{([^{}]*)\}\s*\{([^{}]*)\}`)

func fracToSlash(s string) string {
	return latexFrac.ReplaceAllString(s, "$1/$2")
}

// canonicalNumber reduces a numeric string to a canonical rational form.
// The whole string must be numeric: "123" never matches "12".
func canonicalNumber(s string) (string, bool) {
	if s == "" || !numberPrefix.MatchString(s) || numberPrefix.FindString(s) != s {
		return "", false
	}
	r, ok := parseRat(s)
	if !ok {
		return "", false
	}
	if r.IsInt() {
		return r.Num().String(), true
	}
	return r.RatString(), true
}

func parseRat(s string) (*big.Rat, bool) {
	if strings.HasSuffix(s, ".") {
		s += "0"
	}
	if strings.HasPrefix(s, ".") {
		s = "0" + s
	}
	if strings.HasPrefix(s, "-.") {
		s = "-0" + s[1:]
	}
	r, ok := new(big.Rat).SetString(s)
	return r, ok
}

// splitUnit separates a leading number from a trailing unit ("42个", "5cm",
// "30%"). The unit must contain no digits, so "12或13" is not "12".
func splitUnit(s string) (string, bool) {
	prefix := numberPrefix.FindString(s)
	if prefix == "" || prefix == s {
		return "", false
	}
	unit := s[len(prefix):]
	for _, r := range unit {
		if unicode.IsDigit(r) || r == '.' || r == '/' || r == '-' {
			return "", false
		}
	}
	return prefix, true
}

// AnswerMatches reports whether a candidate equals an accepted answer after
// normalization. Units on either side are ignored when the other is a bare
// number.
func AnswerMatches(candidate, accepted string) bool {
	c, a := NormalizeAnswer(candidate), NormalizeAnswer(accepted)
	if c == "" || a == "" {
		return false
	}
	if c == a {
		return true
	}
	cn, cUnit := splitUnit(c)
	an, aUnit := splitUnit(a)
	switch {
	case cUnit && aUnit:
		// Both carry units: numbers must match and units must match.
		cv, _ := canonicalNumber(cn)
		av, _ := canonicalNumber(an)
		return cv == av && strings.TrimPrefix(c, cn) == strings.TrimPrefix(a, an)
	case cUnit:
		cv, _ := canonicalNumber(cn)
		return cv != "" && cv == a
	case aUnit:
		av, _ := canonicalNumber(an)
		return av != "" && av == c
	}
	return false
}

// JudgeLogic returns the extracted answer shown to users and whether any
// candidate matches any accepted answer.
func JudgeLogic(reply string, accepted []string) (string, bool) {
	candidates := ExtractAnswers(reply)
	shown := strings.Join(candidates, " / ")
	if len([]rune(shown)) > 200 {
		shown = string([]rune(shown)[:200])
	}
	for _, c := range candidates {
		for _, a := range accepted {
			if AnswerMatches(c, a) {
				return shown, true
			}
		}
	}
	return shown, false
}

var (
	svgBlock   = regexp.MustCompile(`(?is)<svg\b[^>]*>(.*?)</svg\s*>`)
	svgOpen    = regexp.MustCompile(`(?i)<svg\b`)
	fenceBlock = regexp.MustCompile("(?s)```[a-zA-Z0-9_-]*\\s*\\n(.*?)```")
	shapeTag   = regexp.MustCompile(`(?is)<(path|rect|circle|ellipse|line|polyline|polygon|text|use|image|g)\b([^>]*)>`)
	attrValue  = regexp.MustCompile(`(?is)\b([a-z:-]+)\s*=\s*("([^"]*)"|'([^']*)')`)
	textBody   = regexp.MustCompile(`(?is)<text\b[^>]*>(.*?)</text>`)
	tagStrip   = regexp.MustCompile(`(?s)<[^>]*>`)
)

func attrs(raw string) map[string]string {
	out := map[string]string{}
	for _, m := range attrValue.FindAllStringSubmatch(raw, -1) {
		v := m[3]
		if v == "" {
			v = m[4]
		}
		out[strings.ToLower(m[1])] = strings.TrimSpace(v)
	}
	return out
}

func nonZero(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	if r, ok := parseRat(strings.TrimSuffix(strings.TrimSuffix(v, "px"), "%")); ok {
		return r.Sign() != 0
	}
	return true
}

// svgDraws reports whether an SVG body contains at least one element that
// paints something.
func svgDraws(body string) bool {
	for _, m := range shapeTag.FindAllStringSubmatch(body, -1) {
		a := attrs(m[2])
		switch strings.ToLower(m[1]) {
		case "path":
			if strings.TrimSpace(a["d"]) != "" {
				return true
			}
		case "rect":
			if nonZero(a["width"]) && nonZero(a["height"]) {
				return true
			}
		case "circle":
			if nonZero(a["r"]) {
				return true
			}
		case "ellipse":
			if nonZero(a["rx"]) || nonZero(a["ry"]) {
				return true
			}
		case "line":
			if a["x1"] != a["x2"] || a["y1"] != a["y2"] {
				return true
			}
		case "polyline", "polygon":
			if strings.TrimSpace(a["points"]) != "" {
				return true
			}
		case "use", "image":
			if a["href"] != "" || a["xlink:href"] != "" {
				return true
			}
		}
	}
	for _, m := range textBody.FindAllStringSubmatch(body, -1) {
		if strings.TrimSpace(tagStrip.ReplaceAllString(m[1], "")) != "" {
			return true
		}
	}
	return false
}

// JudgeDrawing passes a reply that contains one complete SVG that draws
// something, wherever it sits (code fences and surrounding prose allowed).
// Truncated replies fail even when an earlier SVG closed.
func JudgeDrawing(reply string, truncated bool) (bool, string) {
	text := strings.TrimSpace(stripThinking(reply))
	if text == "" {
		return false, "空回复"
	}
	if truncated {
		return false, "回复达到输出上限被截断"
	}
	blocks := svgBlock.FindAllStringSubmatch(text, -1)
	if len(blocks) == 0 {
		if svgOpen.MatchString(text) {
			return false, "SVG 不完整（缺少 </svg>）"
		}
		return false, "回复中没有 SVG（可能拒答）"
	}
	for _, b := range blocks {
		if svgDraws(b[1]) {
			return true, ""
		}
	}
	return false, "SVG 中没有画出任何图形"
}

// PreviewHTML extracts the renderable document from a drawing reply: the
// first fenced block that holds an SVG, else the span from the first
// document/SVG tag to the last closing tag.
func PreviewHTML(reply string) string {
	text := stripThinking(reply)
	for _, m := range fenceBlock.FindAllStringSubmatch(text, -1) {
		if svgOpen.MatchString(m[1]) {
			return strings.TrimSpace(m[1])
		}
	}
	lower := strings.ToLower(text)
	start := -1
	for _, tag := range []string{"<!doctype", "<html", "<svg"} {
		if i := strings.Index(lower, tag); i >= 0 && (start < 0 || i < start) {
			start = i
		}
	}
	if start < 0 || !strings.Contains(lower, "<svg") {
		return ""
	}
	end := len(text)
	for _, tag := range []string{"</html>", "</svg>"} {
		if i := strings.LastIndex(lower, tag); i >= 0 {
			end = i + len(tag)
			break
		}
	}
	if end <= start {
		return ""
	}
	return strings.TrimSpace(text[start:end])
}
