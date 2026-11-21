package service

import (
	"errors"
	"strings"
	"testing"
)

func TestResolveAuditServerNode(t *testing.T) {
	host := func() (string, error) { return "container-7f3a", nil }
	failing := func() (string, error) { return "", errors.New("no hostname") }

	if got := resolveAuditServerNode("  hk-1 ", host); got != "hk-1" {
		t.Fatalf("configured node = %q, want hk-1", got)
	}
	if got := resolveAuditServerNode("", host); got != "container-7f3a" {
		t.Fatalf("hostname fallback = %q", got)
	}
	if got := resolveAuditServerNode("", failing); got != "" {
		t.Fatalf("failing hostname = %q, want empty", got)
	}
	long := strings.Repeat("节", 200)
	if got := resolveAuditServerNode(long, host); len([]rune(got)) != auditServerNodeMaxLen {
		t.Fatalf("node not truncated to %d runes: %d", auditServerNodeMaxLen, len([]rune(got)))
	}
}

func TestSanitizeAuditRequestHost(t *testing.T) {
	cases := map[string]string{
		"API.Example.com":    "api.example.com",
		" 1.2.3.4:8080 ":     "1.2.3.4:8080",
		"evil\r\nhost\t.com": "evilhost.com",
		"[2001:db8::1]:443":  "[2001:db8::1]:443",
		"":                   "",
	}
	for in, want := range cases {
		if got := SanitizeAuditRequestHost(in); got != want {
			t.Errorf("SanitizeAuditRequestHost(%q) = %q, want %q", in, got, want)
		}
	}
}
