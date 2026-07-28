package service

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

func TestOpenAICompatibleVideoURLSupportsStandardAndNativeArkAccounts(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	tests := []struct {
		name       string
		baseURL    string
		endpoint   GrokMediaEndpoint
		requestID  string
		wantURL    string
		wantNative bool
	}{
		{
			name: "standard root", baseURL: "https://relay.example.test", endpoint: GrokMediaEndpointVideosGenerations,
			wantURL: "https://relay.example.test/v1/videos/generations",
		},
		{
			name: "standard v1 prefix", baseURL: "https://relay.example.test/openai/v1", endpoint: GrokMediaEndpointVideoStatus,
			requestID: "task 1", wantURL: "https://relay.example.test/openai/v1/videos/task%201",
		},
		{
			name: "ark native", baseURL: "https://ark.cn-beijing.volces.com/api/v3", endpoint: GrokMediaEndpointVideoStatus,
			requestID: "task-1", wantURL: "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/task-1", wantNative: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": tt.baseURL}}
			gotURL, native, err := svc.openAICompatibleVideoURL(account, tt.endpoint, tt.requestID)
			if err != nil {
				t.Fatal(err)
			}
			if gotURL != tt.wantURL || native != tt.wantNative {
				t.Fatalf("url = %q native=%v, want %q native=%v", gotURL, native, tt.wantURL, tt.wantNative)
			}
		})
	}
}

func TestNormalizeArkVideoGenerationAndStatus(t *testing.T) {
	body, err := normalizeArkVideoGenerationBody([]byte(`{
		"model":"doubao-seedance-1-5-pro-250528",
		"prompt":"waves",
		"duration":8,
		"fps":24,
		"aspect_ratio":"16:9",
		"resolution":"1080p"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := gjson.GetBytes(body, "content.0.text").String(); got != "waves" {
		t.Fatalf("content text = %q", got)
	}
	if got := gjson.GetBytes(body, "ratio").String(); got != "16:9" {
		t.Fatalf("ratio = %q", got)
	}
	if got := gjson.GetBytes(body, "framespersecond").Int(); got != 24 {
		t.Fatalf("framespersecond = %d", got)
	}

	normalized := normalizeArkVideoStatusResponse([]byte(`{
		"id":"task-1",
		"status":"succeeded",
		"content":{"video_url":"https://cdn.example.test/video.mp4"}
	}`))
	if !json.Valid(normalized) {
		t.Fatalf("invalid normalized JSON: %s", normalized)
	}
	if got := gjson.GetBytes(normalized, "status").String(); got != "completed" {
		t.Fatalf("status = %q", got)
	}
	if got := gjson.GetBytes(normalized, "video.url").String(); got != "https://cdn.example.test/video.mp4" {
		t.Fatalf("video URL = %q", got)
	}
}
