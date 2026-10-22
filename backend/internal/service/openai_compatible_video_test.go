package service

import (
	"encoding/json"
	"net/http"
	"reflect"
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

func TestNormalizeArkVideoStatusPreservesBillingMetadata(t *testing.T) {
	normalized := normalizeArkVideoStatusResponse([]byte(`{
		"id":"task-720p","status":"succeeded","model":"doubao-seedance-1-5-pro",
		"resolution":"720p","duration":12,"content":{"video_url":"https://example.test/v.mp4"}
	}`))
	usage := openAICompatibleVideoUsageFromResponse(GrokMediaEndpointVideoStatus, GrokMediaRequestInfo{}, normalized)
	if usage.VideoResolution != VideoBillingResolution720P || usage.VideoDurationSeconds != 12 || usage.Model != "doubao-seedance-1-5-pro" {
		t.Fatalf("billing metadata lost: %+v; body=%s", usage, normalized)
	}
}

func TestNormalizeArkVideoGenerationMapsOpenAIVideoParameters(t *testing.T) {
	body, err := normalizeArkVideoGenerationBody([]byte(`{"model":"vendor-video","prompt":"waves","size":"720x1280","seconds":"12"}`))
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(body, "resolution").String() != "720p" || gjson.GetBytes(body, "duration").Int() != 12 {
		t.Fatalf("video parameters lost: %s", body)
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

func TestPlaygroundSeedanceVideoPreservesReferenceImagesThroughArkAdapter(t *testing.T) {
	for _, withFrames := range []bool{false, true} {
		name := "references_only"
		if withFrames {
			name = "frame_pair_and_references"
		}
		t.Run(name, func(t *testing.T) {
			references := []string{
				"https://example.test/reference-1.png",
				"https://example.test/reference-2.png",
				"https://example.test/reference-3.png",
			}
			request := PlaygroundRunRequest{Model: "seedance-2.0-fast-M", Prompt: "animate"}
			if withFrames {
				request.Images = append(request.Images,
					PlaygroundRunImageInput{DataURL: "https://example.test/first.png", Frame: "first"},
					PlaygroundRunImageInput{DataURL: "https://example.test/last.png", Frame: "last"},
				)
			}
			for _, reference := range references {
				request.Images = append(request.Images, PlaygroundRunImageInput{DataURL: reference})
			}
			payload, err := playgroundVideoGenerationPayload(request, false)
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if got := gjson.GetBytes(body, "reference_images.#").Int(); got != int64(len(references)) {
				t.Fatalf("reference image count = %d, want %d", got, len(references))
			}
			if got := gjson.GetBytes(body, "content.#").Int(); got != int64(len(request.Images)+1) {
				t.Fatalf("content count = %d, want %d", got, len(request.Images)+1)
			}
			body, err = normalizeArkVideoGenerationBody(body)
			if err != nil {
				t.Fatal(err)
			}
			content := gjson.GetBytes(body, "content").Array()
			if len(content) != len(request.Images)+1 {
				t.Fatalf("Ark content count = %d, want %d: %s", len(content), len(request.Images)+1, body)
			}
			offset := 1
			if withFrames {
				if content[1].Get("role").String() != "first_frame" || content[2].Get("role").String() != "last_frame" {
					t.Fatalf("frame roles were not preserved: %s", body)
				}
				offset += 2
			}
			for index, reference := range references {
				item := content[offset+index]
				if item.Get("type").String() != "image_url" || item.Get("role").String() != "reference_image" || item.Get("image_url.url").String() != reference {
					t.Fatalf("Ark reference %d was not preserved: %s", index, item.Raw)
				}
			}
		})
	}
}

func TestPlaygroundSeedanceVideoPreservesSingleFirstFrame(t *testing.T) {
	for _, model := range []string{"doubao-seedance-1-5-pro", "seedance-2.0-fast-M"} {
		t.Run(model, func(t *testing.T) {
			payload, err := playgroundVideoGenerationPayload(PlaygroundRunRequest{
				Model:  model,
				Prompt: "animate",
				Images: []PlaygroundRunImageInput{{DataURL: "https://example.test/first.png"}},
			}, false)
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			body, err = normalizeArkVideoGenerationBody(body)
			if err != nil {
				t.Fatal(err)
			}
			if gjson.GetBytes(body, "content.#").Int() != 2 || gjson.GetBytes(body, "content.1.role").String() != "first_frame" || gjson.GetBytes(body, "content.1.image_url.url").String() != "https://example.test/first.png" {
				t.Fatalf("single first frame was changed or duplicated: %s", body)
			}
		})
	}
}

func TestNormalizeArkVideoGenerationPreservesMultimodalContent(t *testing.T) {
	input := []byte(`{
		"model":"seedance-2.0-fast-M",
		"prompt":"animate",
		"image":"https://example.test/first.png",
		"reference_images":["https://example.test/reference.png"],
		"content":[
			{"type":"text","text":"animate with references"},
			{"type":"image_url","image_url":{"url":"https://example.test/first.png"},"role":"first_frame"},
			{"type":"image_url","image_url":{"url":"https://example.test/reference.png"},"role":"reference_image"},
			{"type":"video_url","video_url":{"url":"https://example.test/video.mp4"},"role":"reference_video"},
			{"type":"video_url","video_url":{"url":"https://example.test/depth.webm"},"role":"reference_video"},
			{"type":"audio_url","audio_url":{"url":"https://example.test/audio.mp3"},"role":"reference_audio"}
		]
	}`)
	body, err := normalizeArkVideoGenerationBody(input)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal([]byte(gjson.GetBytes(input, "content").Raw), &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(gjson.GetBytes(body, "content").Raw), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Ark content was changed or duplicated: %s", body)
	}
}

func TestNormalizeArkVideoGenerationAddsReferenceImagesWithoutContent(t *testing.T) {
	body, err := normalizeArkVideoGenerationBody([]byte(`{
		"model":"seedance-2.0-fast-M",
		"prompt":"animate",
		"image":"https://example.test/first.png",
		"image_tail":"https://example.test/last.png",
		"reference_images":["https://example.test/a.png","https://example.test/b.png","https://example.test/c.png"]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	content := gjson.GetBytes(body, "content").Array()
	if len(content) != 6 {
		t.Fatalf("Ark content count = %d, want 6: %s", len(content), body)
	}
	for index, reference := range []string{"https://example.test/a.png", "https://example.test/b.png", "https://example.test/c.png"} {
		item := content[index+3]
		if item.Get("role").String() != "reference_image" || item.Get("image_url.url").String() != reference {
			t.Fatalf("Ark reference %d was not preserved: %s", index, item.Raw)
		}
	}
}

func TestNormalizeArkVideoGenerationAddsVideoAndAudioReferencesWithoutContent(t *testing.T) {
	for _, content := range []string{"", `,"content":[]`} {
		body, err := normalizeArkVideoGenerationBody([]byte(`{
			"model":"doubao-seedance-2-0-fast-260128",
			"prompt":"replace the actor",
			"image":"https://example.test/actor.png?signature=image",
			"video":"https://example.test/source.mp4?signature=video",
			"reference_videos":["https://example.test/source.mp4?signature=video","https://example.test/depth.webm?signature=depth"],
			"reference_audios":["https://example.test/audio.mp3?signature=audio"]` + content + `}`))
		if err != nil {
			t.Fatal(err)
		}
		parts := gjson.GetBytes(body, "content").Array()
		if len(parts) != 5 {
			t.Fatalf("references lost or duplicated: %s", body)
		}
		for i, want := range []struct{ kind, role, url string }{
			{"image", "reference_image", "https://example.test/actor.png?signature=image"},
			{"video", "reference_video", "https://example.test/source.mp4?signature=video"},
			{"video", "reference_video", "https://example.test/depth.webm?signature=depth"},
			{"audio", "reference_audio", "https://example.test/audio.mp3?signature=audio"},
		} {
			part := parts[i+1]
			if part.Get("type").String() != want.kind+"_url" || part.Get("role").String() != want.role || part.Get(want.kind+"_url.url").String() != want.url {
				t.Fatalf("reference %d changed: %s", i, part.Raw)
			}
		}
	}
}

func TestPlaygroundSeedanceImageWithVideoUsesReferenceRole(t *testing.T) {
	payload, err := playgroundVideoGenerationPayload(PlaygroundRunRequest{
		Model:           "doubao-seedance-2-0-fast-260128",
		Images:          []PlaygroundRunImageInput{{DataURL: "https://example.test/actor.png"}},
		ReferenceVideos: []PlaygroundRunVideoInput{{DataURL: "https://example.test/source.mp4"}, {DataURL: "https://example.test/depth.webm"}},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	parts := payload["content"].([]map[string]any)
	if len(parts) != 4 || parts[1]["role"] != "reference_image" {
		t.Fatalf("image/video references misclassified: %#v", parts)
	}
}

func TestNormalizeArkVideoGenerationPreservesExplicitFirstFrameWithVideo(t *testing.T) {
	body, err := normalizeArkVideoGenerationBody([]byte(`{"model":"doubao-seedance-2-0-fast-260128","first_frame_image":"https://example.test/first.png","video":"https://example.test/source.mp4"}`))
	if err != nil {
		t.Fatal(err)
	}
	if gjson.GetBytes(body, "content.1.role").String() != "first_frame" || gjson.GetBytes(body, "content.2.role").String() != "reference_video" {
		t.Fatalf("explicit frame role lost: %s", body)
	}
}
