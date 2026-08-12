package service

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

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
	usage := openAICompatibleVideoUsageFromResponse(GrokMediaEndpointVideoStatus, GrokMediaRequestInfo{}, normalized)
	if usage.ResponseID != "task-1" || usage.VideoCount != 1 {
		t.Fatalf("normalized Ark status must be billable, response_id=%q video_count=%d", usage.ResponseID, usage.VideoCount)
	}
}

func TestOpenAICompatibleVideoContentResultIsBillable(t *testing.T) {
	headers := http.Header{"Content-Type": []string{"video/mp4"}}
	result := openAICompatibleVideoContentResult("seedance-task", "upstream-request", headers, 5*time.Minute)
	if result.ResponseID != "seedance-task" || result.RequestID != "upstream-request" {
		t.Fatalf("unexpected ids: response=%q request=%q", result.ResponseID, result.RequestID)
	}
	if result.VideoCount != 1 {
		t.Fatalf("video count = %d, want 1", result.VideoCount)
	}
	if result.Duration != 5*time.Minute {
		t.Fatalf("duration = %s", result.Duration)
	}
	if result.ResponseHeaders.Get("Content-Type") != "video/mp4" {
		t.Fatalf("content type = %q", result.ResponseHeaders.Get("Content-Type"))
	}
}

func TestNormalizeArkVideoGenerationAddsFirstFrameImage(t *testing.T) {
	body, err := normalizeArkVideoGenerationBody([]byte(`{
		"model":"doubao-seedance-1-5-pro-250528",
		"prompt":"animate",
		"image":"data:image/png;base64,QUJD"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := gjson.GetBytes(body, "content.1.type").String(); got != "image_url" {
		t.Fatalf("image content type = %q", got)
	}
	if got := gjson.GetBytes(body, "content.1.role").String(); got != "first_frame" {
		t.Fatalf("image role = %q", got)
	}
	if got := gjson.GetBytes(body, "content.1.image_url.url").String(); got != "data:image/png;base64,QUJD" {
		t.Fatalf("image url = %q", got)
	}
}

func TestNormalizeArkVideoGenerationAddsLastFrameImage(t *testing.T) {
	body, err := normalizeArkVideoGenerationBody([]byte(`{
		"model":"doubao-seedance-1-5-pro-250528",
		"prompt":"animate",
		"image":"data:image/png;base64,QUJD",
		"image_tail":"data:image/png;base64,REVG"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := gjson.GetBytes(body, "content.2.role").String(); got != "last_frame" {
		t.Fatalf("last image role = %q", got)
	}
	if got := gjson.GetBytes(body, "content.2.image_url.url").String(); got != "data:image/png;base64,REVG" {
		t.Fatalf("last image url = %q", got)
	}
}
