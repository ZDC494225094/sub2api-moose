package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlaygroundNonJSONErrorSnippet(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty", "  ", ""},
		{"s3 xml", `<?xml version="1.0"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`, "AccessDenied - Access Denied"},
		{"oss xml code only", `<Error><Code>AccessDenied</Code></Error>`, "AccessDenied"},
		{"html title", `<html><head><title>403 Forbidden</title></head><body><h1>blocked</h1></body></html>`, "403 Forbidden"},
		{"plain text", "Request blocked by WAF\n\n ray id 123", "Request blocked by WAF ray id 123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := playgroundNonJSONErrorSnippet([]byte(tc.body)); got != tc.want {
				t.Fatalf("snippet = %q, want %q", got, tc.want)
			}
		})
	}

	long := strings.Repeat("x", 500)
	if got := playgroundNonJSONErrorSnippet([]byte(long)); len([]rune(got)) != playgroundErrorSnippetMaxRunes+1 {
		t.Fatalf("long snippet not truncated: %d runes", len([]rune(got)))
	}
}

func TestParsePlaygroundUpstreamErrorKeepsNonJSONReason(t *testing.T) {
	resp := &http.Response{
		Status:     "403 Forbidden",
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(`<Error><Code>AccessDenied</Code><Message>Request has expired</Message></Error>`)),
	}
	err := parsePlaygroundUpstreamError(resp)
	want := "403 Forbidden: AccessDenied - Request has expired (HTTP 403)"
	if err == nil || err.Error() != want {
		t.Fatalf("error = %v, want %q", err, want)
	}
}

func TestParsePlaygroundUpstreamErrorPrefersJSONMessage(t *testing.T) {
	resp := &http.Response{
		Status:     "403 Forbidden",
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"no image permission"}}`)),
	}
	err := parsePlaygroundUpstreamError(resp)
	if err == nil || err.Error() != "no image permission (HTTP 403)" {
		t.Fatalf("error = %v", err)
	}
}

func TestDownloadPlaygroundAssetSendsBrowserHeaders(t *testing.T) {
	var gotUA, gotAccept, gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		gotAuth = r.Header.Get("Authorization")
		if strings.HasPrefix(gotUA, "Go-http-client") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake"))
	}))
	defer server.Close()

	svc := newPlaygroundRunService(nil, nil)
	data, mimeType, err := svc.downloadPlaygroundImage(context.Background(), server.URL+"/a.png", "")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if len(data) == 0 || mimeType != "image/png" {
		t.Fatalf("unexpected result: %d bytes, %q", len(data), mimeType)
	}
	if gotUA != playgroundAssetUserAgent {
		t.Fatalf("User-Agent = %q", gotUA)
	}
	if !strings.HasPrefix(gotAccept, "image/") {
		t.Fatalf("Accept = %q", gotAccept)
	}
	if gotAuth != "" {
		t.Fatalf("remote asset download must not send credentials, got %q", gotAuth)
	}
}

func TestPlaygroundRemoteAssetErrorAddsStageAndHost(t *testing.T) {
	if playgroundRemoteAssetError("image", "https://cdn.example.com/a.png", nil) != nil {
		t.Fatal("nil error should stay nil")
	}
	base := errors.New("403 Forbidden (HTTP 403)")
	err := playgroundRemoteAssetError("image", "https://cdn.example.com/a.png?sig=1", base)
	if err == nil || !strings.Contains(err.Error(), "cdn.example.com") || !strings.Contains(err.Error(), "图片") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "sig=1") {
		t.Fatalf("query string leaked into error: %v", err)
	}
	if !errors.Is(err, base) {
		t.Fatal("wrapped error should unwrap to the original")
	}
}
