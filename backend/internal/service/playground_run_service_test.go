package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestPlaygroundRunServicePersistsImageRunAcrossInstances(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	completedAt := time.Now()
	writer := ProvidePlaygroundRunService(redisClient)
	run := &PlaygroundRun{
		ID:          "persisted-image",
		UserID:      7,
		Mode:        "image",
		Status:      PlaygroundRunSucceeded,
		Model:       "gpt-image-2",
		CreatedAt:   completedAt.Add(-time.Second),
		UpdatedAt:   completedAt,
		CompletedAt: &completedAt,
		Images: []PlaygroundRunImage{{
			MimeType: "image/png",
			data:     []byte("image-bytes"),
		}},
	}
	if err := writer.persistRun(run); err != nil {
		t.Fatalf("persist playground run: %v", err)
	}

	reader := ProvidePlaygroundRunService(redisClient)
	stored, found := reader.Get(7, run.ID)
	if !found {
		t.Fatal("persisted playground run was not found")
	}
	if stored.Status != PlaygroundRunSucceeded {
		t.Fatalf("status = %q, want %q", stored.Status, PlaygroundRunSucceeded)
	}
	if len(stored.Images) != 1 || stored.Images[0].AssetIndex == nil {
		t.Fatalf("stored image metadata = %#v, want one image asset", stored.Images)
	}

	asset, found, err := reader.GetImage(7, run.ID, 0)
	if err != nil {
		t.Fatalf("get persisted image: %v", err)
	}
	if !found {
		t.Fatal("persisted playground image was not found")
	}
	if got := string(asset.Data); got != "image-bytes" {
		t.Fatalf("asset data = %q, want %q", got, "image-bytes")
	}
	if asset.ContentType != "image/png" {
		t.Fatalf("asset content type = %q, want image/png", asset.ContentType)
	}
}

func TestPlaygroundRunServiceStartIsIdempotentAcrossInstances(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	var upstreamCalls atomic.Int32
	firstRequest := make(chan struct{})
	releaseFirstRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if upstreamCalls.Add(1) == 1 {
			close(firstRequest)
			<-releaseFirstRequest
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	t.Cleanup(server.Close)

	request := PlaygroundRunRequest{
		ID:           "idempotent-image",
		Mode:         "image",
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		N:            1,
	}
	writer := ProvidePlaygroundRunService(redisClient)
	if _, err := writer.Start(7, request, server.URL); err != nil {
		t.Fatalf("start initial run: %v", err)
	}
	select {
	case <-firstRequest:
	case <-time.After(time.Second):
		t.Fatal("initial run did not reach upstream")
	}

	reader := ProvidePlaygroundRunService(redisClient)
	duplicate, err := reader.Start(7, request, server.URL)
	if err != nil {
		t.Fatalf("start duplicate run: %v", err)
	}
	if duplicate.ID != request.ID {
		t.Fatalf("duplicate run ID = %q, want %q", duplicate.ID, request.ID)
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("upstream calls before initial completion = %d, want 1", got)
	}

	close(releaseFirstRequest)
	deadline := time.Now().Add(time.Second)
	for {
		run, found := writer.Get(7, request.ID)
		if found && run.Status == PlaygroundRunSucceeded {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("initial run did not complete: %#v", run)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := upstreamCalls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want 1", got)
	}
}

func TestPlaygroundRunServiceExecuteImageUsesGenerationsWithoutUploads(t *testing.T) {
	var gotPath string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.runs["1:run"] = &PlaygroundRun{ID: "run", UserID: 1, Mode: "image", Status: PlaygroundRunRunning}
	raw, err := svc.executeImage(context.Background(), "1:run", PlaygroundRunRequest{
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		Size:         "2048x1152",
		N:            1,
		OutputFormat: "png",
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute image: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("raw response length = %d, want 0", len(raw))
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("path = %q, want /v1/images/generations", gotPath)
	}
	if gotPayload["response_format"] != "b64_json" {
		t.Fatalf("response_format = %v, want b64_json", gotPayload["response_format"])
	}
	if gotPayload["n"] != float64(1) {
		t.Fatalf("n = %v, want 1", gotPayload["n"])
	}
	if gotPayload["size"] != "2048x1152" {
		t.Fatalf("size = %v, want 2048x1152", gotPayload["size"])
	}
}

func TestPlaygroundImageRequestSizePassesGPTImage2DimensionsThrough(t *testing.T) {
	tests := []struct {
		model string
		size  string
		want  string
	}{
		{model: "gpt-image-2", size: "2048x1152", want: "2048x1152"},
		{model: "gpt-image-2-2026-04-21", size: "4096x2304", want: "4096x2304"},
		{model: "gpt-image-2", size: "4096x3072", want: "4096x3072"},
		{model: "gpt-image-2", size: "256x256", want: "256x256"},
		{model: "gpt-image-2", size: "4096x256", want: "4096x256"},
		{model: "gpt-image-1.5", size: "4096x4096", want: "1024x1024"},
		{model: "gpt-image-2", size: "invalid", want: "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.model+"/"+tt.size, func(t *testing.T) {
			if got := playgroundImageRequestSize(tt.model, tt.size); got != tt.want {
				t.Fatalf("playgroundImageRequestSize(%q, %q) = %q, want %q", tt.model, tt.size, got, tt.want)
			}
		})
	}
}

func TestPlaygroundRunServiceExecuteGiteeZImageUsesProviderPayload(t *testing.T) {
	var gotPath string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.runs["1:gitee"] = &PlaygroundRun{ID: "gitee", UserID: 1, Mode: "image", Status: PlaygroundRunRunning}
	_, err := svc.executeImage(context.Background(), "1:gitee", PlaygroundRunRequest{
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "z-image-turbo",
		Prompt:       "draw a city portrait",
		Size:         "4096x4096",
		N:            1,
		Quality:      "high",
		Background:   "transparent",
		OutputFormat: "webp",
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute Gitee Z-Image: %v", err)
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("path = %q, want /v1/images/generations", gotPath)
	}
	for key, want := range map[string]any{
		"model":                 "z-image-turbo",
		"prompt":                "draw a city portrait",
		"num_images_per_prompt": float64(1),
		"negative_prompt":       "blurry ugly bad",
		"num_inference_steps":   float64(9),
		"seed":                  float64(0),
		"guidance_scale":        float64(1),
	} {
		if gotPayload[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, gotPayload[key], want)
		}
	}
	for _, key := range []string{"size", "n", "response_format", "quality", "background", "output_format"} {
		if _, exists := gotPayload[key]; exists {
			t.Fatalf("Gitee Z-Image payload must not contain %q: %#v", key, gotPayload)
		}
	}
}

func TestPlaygroundRunServiceExecuteGiteeZImageUsesControlImage(t *testing.T) {
	var gotPath string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/generated.png"}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.runs["1:gitee-control"] = &PlaygroundRun{ID: "gitee-control", UserID: 1, Mode: "image", Status: PlaygroundRunRunning}
	_, err := svc.executeImage(context.Background(), "1:gitee-control", PlaygroundRunRequest{
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "z-image-turbo",
		Prompt:       "follow the edges",
		N:            1,
		OutputFormat: "png",
		Images: []PlaygroundRunImageInput{{
			Name:    "control.png",
			Type:    "image/png",
			DataURL: "data:image/png;base64,cG5nLWJ5dGVz",
		}},
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute Gitee controlled Z-Image: %v", err)
	}
	if gotPath != "/v1/images/generations" {
		t.Fatalf("path = %q, want /v1/images/generations", gotPath)
	}
	for key, want := range map[string]any{
		"control_image":         "cG5nLWJ5dGVz",
		"control_mode":          "HED",
		"control_context_scale": 0.75,
		"image_scale":           float64(1),
	} {
		if gotPayload[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, gotPayload[key], want)
		}
	}
}

func TestPlaygroundRunServiceExecuteGeminiNativeImage(t *testing.T) {
	var gotPath string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"done"},{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.runs["1:gemini"] = &PlaygroundRun{ID: "gemini", UserID: 1, Mode: "image", Status: PlaygroundRunRunning}
	_, err := svc.executeImage(context.Background(), "1:gemini", PlaygroundRunRequest{
		APIKey:   "sk-test",
		Platform: "gemini",
		Model:    "gemini-3.1-flash-image-preview",
		Prompt:   "draw a cat",
		Size:     "1024x1024",
		N:        1,
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute Gemini image: %v", err)
	}
	svc.update("1:gemini", func(run *PlaygroundRun) { run.Status = PlaygroundRunSucceeded })
	if gotPath != "/v1beta/models/gemini-3.1-flash-image-preview:generateContent" {
		t.Fatalf("path = %q", gotPath)
	}
	config, ok := gotPayload["generationConfig"].(map[string]any)
	if !ok || config["responseModalities"] == nil {
		t.Fatalf("generation config = %#v", gotPayload["generationConfig"])
	}
	run, ok := svc.Get(1, "gemini")
	if !ok || len(run.Images) != 1 || run.Images[0].AssetIndex == nil {
		t.Fatalf("run images = %+v", run.Images)
	}
	asset, found, err := svc.GetImage(1, "gemini", 0)
	if err != nil || !found || string(asset.Data) != "image" {
		t.Fatalf("image asset = found:%v err:%v data:%q", found, err, asset.Data)
	}
}

func TestPlaygroundRunServiceExecuteOpenAIProtocolGeminiImageUsesImagesAPI(t *testing.T) {
	var gotPath string
	var gotPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.runs["1:openai-gemini"] = &PlaygroundRun{ID: "openai-gemini", UserID: 1, Mode: "image", Status: PlaygroundRunRunning}
	_, err := svc.executeImage(context.Background(), "1:openai-gemini", PlaygroundRunRequest{
		APIKey:   "sk-test",
		Platform: "openai",
		Model:    "gemini-3.1-flash-image-preview",
		Prompt:   "draw a cat",
		Size:     "1024x1024",
		N:        1,
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute OpenAI protocol Gemini image: %v", err)
	}
	svc.update("1:openai-gemini", func(run *PlaygroundRun) { run.Status = PlaygroundRunSucceeded })
	if gotPath != "/v1/images/generations" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotPayload["model"] != "gemini-3.1-flash-image-preview" {
		t.Fatalf("model = %#v", gotPayload["model"])
	}
	run, ok := svc.Get(1, "openai-gemini")
	if !ok || len(run.Images) != 1 || run.Images[0].AssetIndex == nil {
		t.Fatalf("run images = %+v", run.Images)
	}
	asset, found, err := svc.GetImage(1, "openai-gemini", 0)
	if err != nil || !found || string(asset.Data) != "image" {
		t.Fatalf("image asset = found:%v err:%v data:%q", found, err, asset.Data)
	}
}

func TestPlaygroundRunServiceExecuteImageFansOutConcurrentSingleImageRequests(t *testing.T) {
	const imageCount = 4
	var requestCount atomic.Int32
	var activeRequests atomic.Int32
	var maxActiveRequests atomic.Int32
	allStarted := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseAll := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseAll()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
			return
		}
		if payload["n"] != float64(1) {
			t.Errorf("upstream n = %v, want 1", payload["n"])
		}

		active := activeRequests.Add(1)
		defer activeRequests.Add(-1)
		for {
			currentMax := maxActiveRequests.Load()
			if active <= currentMax || maxActiveRequests.CompareAndSwap(currentMax, active) {
				break
			}
		}
		if requestCount.Add(1) == imageCount {
			close(allStarted)
		}
		<-release
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	defer server.Close()

	type executeResult struct{ err error }
	resultCh := make(chan executeResult, 1)
	svc := NewPlaygroundRunService()
	svc.runs["1:run"] = &PlaygroundRun{ID: "run", UserID: 1, Mode: "image", Status: PlaygroundRunRunning}
	go func() {
		_, err := svc.executeImage(context.Background(), "1:run", PlaygroundRunRequest{
			APIKey:       "sk-test",
			EndpointBase: "/v1",
			Model:        "gpt-image-2",
			Prompt:       "draw four cats",
			Size:         "1024x1024",
			N:            imageCount,
			OutputFormat: "png",
		}, server.URL, time.Now())
		resultCh <- executeResult{err: err}
	}()

	select {
	case <-allStarted:
		releaseAll()
	case <-time.After(2 * time.Second):
		releaseAll()
		result := <-resultCh
		t.Fatalf("requests did not run concurrently: count=%d max_active=%d err=%v", requestCount.Load(), maxActiveRequests.Load(), result.err)
	}

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("execute image: %v", result.err)
	}
	if requestCount.Load() != imageCount {
		t.Fatalf("request count = %d, want %d", requestCount.Load(), imageCount)
	}
	if maxActiveRequests.Load() != imageCount {
		t.Fatalf("max active requests = %d, want %d", maxActiveRequests.Load(), imageCount)
	}
	run, ok := svc.Get(1, "run")
	if !ok {
		t.Fatal("expected playground run")
	}
	if len(run.Images) != imageCount {
		t.Fatalf("image result count = %d, want %d", len(run.Images), imageCount)
	}
	for index, image := range run.Images {
		if image.URL != "" || image.AssetIndex == nil || *image.AssetIndex != index {
			t.Fatalf("image %d metadata = %+v, want asset index without inline URL", index, image)
		}
	}
	svc.update("1:run", func(run *PlaygroundRun) {
		run.Status = PlaygroundRunSucceeded
	})
	asset, found, err := svc.GetImage(1, "run", 0)
	if err != nil || !found {
		t.Fatalf("get image asset: found=%v err=%v", found, err)
	}
	if string(asset.Data) != "image" || asset.ContentType != "image/png" {
		t.Fatalf("asset = type:%q data:%q", asset.ContentType, asset.Data)
	}
}

func TestPlaygroundRunServiceStartCompletesImageAndServesAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"aW1hZ2U="}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	started, err := svc.Start(1, PlaygroundRunRequest{
		ID:           "lifecycle",
		Mode:         "image",
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "gpt-image-2",
		Prompt:       "draw a cat",
		N:            2,
		OutputFormat: "png",
	}, server.URL)
	if err != nil {
		t.Fatalf("start image run: %v", err)
	}
	if started.Status != PlaygroundRunQueued {
		t.Fatalf("initial status = %q, want queued", started.Status)
	}

	deadline := time.Now().Add(2 * time.Second)
	var completed *PlaygroundRun
	for time.Now().Before(deadline) {
		completed, _ = svc.Get(1, "lifecycle")
		if completed != nil && isTerminalPlaygroundRunStatus(completed.Status) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if completed == nil || completed.Status != PlaygroundRunSucceeded {
		t.Fatalf("completed run = %+v, want succeeded", completed)
	}
	if len(completed.Images) != 2 {
		t.Fatalf("image count = %d, want 2", len(completed.Images))
	}
	asset, found, err := svc.GetImage(1, "lifecycle", 1)
	if err != nil || !found || string(asset.Data) != "image" {
		t.Fatalf("asset = found:%v err:%v data:%q", found, err, asset.Data)
	}
}

func TestPlaygroundRunServiceImageStatusOmitsInlineDataAndRaw(t *testing.T) {
	svc := NewPlaygroundRunService()
	svc.runs["1:run"] = &PlaygroundRun{
		ID:     "run",
		UserID: 1,
		Mode:   "image",
		Status: PlaygroundRunSucceeded,
		Images: []PlaygroundRunImage{{
			RevisedPrompt: "refined prompt",
			MimeType:      "image/png",
			data:          []byte("large-image-bytes"),
		}},
		Raw: json.RawMessage(`{"data":[{"b64_json":"duplicate-image-data"}]}`),
	}

	run, ok := svc.Get(1, "run")
	if !ok {
		t.Fatal("expected playground run")
	}
	if run.Raw != nil {
		t.Fatal("image run summary must omit raw response")
	}
	if len(run.Images) != 1 || run.Images[0].AssetIndex == nil || *run.Images[0].AssetIndex != 0 {
		t.Fatalf("image metadata = %+v", run.Images)
	}
	encoded, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal run summary: %v", err)
	}
	if strings.Contains(string(encoded), "large-image-bytes") || strings.Contains(string(encoded), "duplicate-image-data") {
		t.Fatalf("run summary leaked inline image data: %s", encoded)
	}

	asset, found, err := svc.GetImage(1, "run", 0)
	if err != nil || !found {
		t.Fatalf("get image asset: found=%v err=%v", found, err)
	}
	if string(asset.Data) != "large-image-bytes" || asset.ContentType != "image/png" {
		t.Fatalf("asset = type:%q data:%q", asset.ContentType, asset.Data)
	}
}

func TestPlaygroundRunServiceCancelClearsImageAssets(t *testing.T) {
	svc := NewPlaygroundRunService()
	svc.runs["1:run"] = &PlaygroundRun{
		ID:     "run",
		UserID: 1,
		Mode:   "image",
		Status: PlaygroundRunRunning,
		Images: []PlaygroundRunImage{{data: []byte("large-image-bytes")}},
		Raw:    json.RawMessage(`{"data":"duplicate-image-data"}`),
	}

	run, found := svc.Cancel(1, "run")
	if !found || run.Status != PlaygroundRunCanceled {
		t.Fatalf("cancel result = found:%v run:%+v", found, run)
	}
	if len(run.Images) != 0 || run.Raw != nil {
		t.Fatalf("canceled run retained image data: %+v", run)
	}
	if _, found, err := svc.GetImage(1, "run", 0); err != nil || found {
		t.Fatalf("canceled image asset = found:%v err:%v", found, err)
	}
}

func TestExtractPlaygroundImagesConvertsInlineURLAndDetectsMimeType(t *testing.T) {
	var payload playgroundImageUpstreamResponse
	if err := json.Unmarshal([]byte(`{"data":[{"url":"data:image/webp;base64,iVBORw0KGgo=","revised_prompt":"prompt"}]}`), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	images := extractPlaygroundImages(payload, "webp")
	if len(images) != 1 {
		t.Fatalf("image count = %d, want 1", len(images))
	}
	if images[0].URL != "" || images[0].MimeType != "image/png" || len(images[0].data) == 0 {
		t.Fatalf("image = %+v, data_len=%d", images[0], len(images[0].data))
	}
}

func TestExtractPlaygroundImagesReportsActualDimensions(t *testing.T) {
	encoded := encodeOpenAIImageTestPNG(t, 1086, 1448)
	var payload playgroundImageUpstreamResponse
	if err := json.Unmarshal([]byte(fmt.Sprintf(`{"data":[{"b64_json":%q}]}`, encoded)), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	images := extractPlaygroundImages(payload, "png")
	if len(images) != 1 {
		t.Fatalf("image count = %d, want 1", len(images))
	}
	if images[0].Width != 1086 || images[0].Height != 1448 {
		t.Fatalf("actual dimensions = %dx%d, want 1086x1448", images[0].Width, images[0].Height)
	}
}

func TestPlaygroundRunServiceExecuteImageUsesEditsWithUploads(t *testing.T) {
	var gotPath string
	fields := map[string]string{}
	var gotUploadName string
	var gotUploadType string
	var gotUploadBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatalf("parse content type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Fatalf("content type = %q, want multipart/form-data", mediaType)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := reader.NextPart()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatalf("read part: %v", err)
			}
			name := part.FormName()
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("read part body: %v", err)
			}
			if part.FileName() != "" {
				gotUploadName = part.FileName()
				gotUploadType = part.Header.Get("Content-Type")
				gotUploadBody = string(data)
			} else {
				fields[name] = string(data)
			}
			_ = part.Close()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"ZWRpdGVk"}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.runs["1:run"] = &PlaygroundRun{ID: "run", UserID: 1, Mode: "image", Status: PlaygroundRunRunning}
	raw, err := svc.executeImage(context.Background(), "1:run", PlaygroundRunRequest{
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "gpt-image-2",
		Prompt:       "replace background",
		Size:         "1536x1024",
		N:            1,
		OutputFormat: "png",
		Images: []PlaygroundRunImageInput{{
			Name:    "source.png",
			Type:    "image/png",
			DataURL: "data:image/png;base64,cG5nLWJ5dGVz",
		}},
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute image edit: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("raw response length = %d, want 0", len(raw))
	}
	if gotPath != "/v1/images/edits" {
		t.Fatalf("path = %q, want /v1/images/edits", gotPath)
	}
	if fields["prompt"] != "replace background" {
		t.Fatalf("prompt = %q", fields["prompt"])
	}
	if fields["response_format"] != "b64_json" {
		t.Fatalf("response_format = %q, want b64_json", fields["response_format"])
	}
	if fields["n"] != "1" {
		t.Fatalf("n = %q, want 1", fields["n"])
	}
	if gotUploadName != "source.png" || gotUploadType != "image/png" || !strings.Contains(gotUploadBody, "png-bytes") {
		t.Fatalf("upload = name:%q type:%q body:%q", gotUploadName, gotUploadType, gotUploadBody)
	}
}

func TestPlaygroundRunServiceExecuteImageEditFansOutSingleImageRequests(t *testing.T) {
	const imageCount = 3
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse multipart form: %v", err)
			return
		}
		if got := r.FormValue("n"); got != "1" {
			t.Errorf("upstream n = %q, want 1", got)
		}
		file, _, err := r.FormFile("image")
		if err != nil {
			t.Errorf("read image upload: %v", err)
			return
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil || string(data) != "png-bytes" {
			t.Errorf("upload body = %q, err=%v", data, err)
		}
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"b64_json":"ZWRpdGVk"}]}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.runs["1:edit"] = &PlaygroundRun{ID: "edit", UserID: 1, Mode: "image", Status: PlaygroundRunRunning}
	_, err := svc.executeImage(context.Background(), "1:edit", PlaygroundRunRequest{
		APIKey:       "sk-test",
		EndpointBase: "/v1",
		Model:        "gpt-image-2",
		Prompt:       "replace background",
		Size:         "1536x1024",
		N:            imageCount,
		OutputFormat: "png",
		Images: []PlaygroundRunImageInput{{
			Name:    "source.png",
			Type:    "image/png",
			DataURL: "data:image/png;base64,cG5nLWJ5dGVz",
		}},
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute image edit: %v", err)
	}
	if requestCount.Load() != imageCount {
		t.Fatalf("request count = %d, want %d", requestCount.Load(), imageCount)
	}
	run, ok := svc.Get(1, "edit")
	if !ok || len(run.Images) != imageCount {
		t.Fatalf("image result count = %d, want %d", len(run.Images), imageCount)
	}
}

func TestPlaygroundRunServiceParallelImageRequestsCancelSiblingsOnFailure(t *testing.T) {
	const imageCount = 4
	upstreamErr := errors.New("upstream 502")
	var callCount atomic.Int32
	var canceledCount atomic.Int32
	started := make(chan struct{}, imageCount)
	release := make(chan struct{})
	resultCh := make(chan error, 1)

	svc := NewPlaygroundRunService()
	go func() {
		_, err := svc.executeParallelImageRequests(context.Background(), "1:run", time.Now(), imageCount, func(ctx context.Context) ([]PlaygroundRunImage, error) {
			index := callCount.Add(1)
			started <- struct{}{}
			<-release
			if index == 1 {
				return nil, upstreamErr
			}
			<-ctx.Done()
			canceledCount.Add(1)
			return nil, ctx.Err()
		})
		resultCh <- err
	}()

	for range imageCount {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatal("parallel image requests did not all start")
		}
	}
	close(release)
	err := <-resultCh
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("error = %v, want upstream error", err)
	}
	if canceledCount.Load() != imageCount-1 {
		t.Fatalf("canceled sibling count = %d, want %d", canceledCount.Load(), imageCount-1)
	}
}
