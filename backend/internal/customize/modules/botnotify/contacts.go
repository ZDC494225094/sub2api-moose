package botnotify

import (
	"regexp"
	"strings"
)

// The after-sales group and contact-link settings hold one entry per line,
// "名称 | 值" or just "值" (the admin page writes this format; older single
// values are one entry). Keep this in sync with the frontend contacts.ts.
var contactLinePattern = regexp.MustCompile(`^(.{1,40}?)\s+\|\s+(.+)$`)

const maxContactEntries = 10

// contactEntries renders each entry as "名称 值" (or just "值").
func contactEntries(raw string) []string {
	var out []string
	for _, line := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		if m := contactLinePattern.FindStringSubmatch(text); m != nil {
			label, value := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
			if value == "" {
				continue
			}
			text = label + " " + value
		}
		out = append(out, text)
		if len(out) >= maxContactEntries {
			break
		}
	}
	return out
}
