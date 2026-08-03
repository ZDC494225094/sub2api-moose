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
	writer := ProvidePlaygroundRunService(redisClient, nil)
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

	reader := ProvidePlaygroundRunService(redisClient, nil)
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
	writer := ProvidePlaygroundRunService(redisClient, nil)
	if _, err := writer.Start(7, request, server.URL); err != nil {
		t.Fatalf("start initial run: %v", err)
	}
	select {
	case <-firstRequest:
	case <-time.After(time.Second):
		t.Fatal("initial run did not reach upstream")
	}

	reader := ProvidePlaygroundRunService(redisClient, nil)
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
		if r.URL.Path == "/asset.png" {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("generated-image"))
			return
		}
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"url":"/asset.png"}]}`))
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

func TestPlaygroundRunServiceStoresAbsoluteImageURLAsProtectedAsset(t *testing.T) {
	const imageBytes = "remote-image-bytes"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if got := r.Header.Get("Authorization"); got != "Bearer sk-image" {
				t.Errorf("generation authorization = %q", got)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"data":[{"url":%q}]}`, server.URL+"/cdn/generated.png?token=signed")
		case "/cdn/generated.png":
			if got := r.Header.Get("Authorization"); got != "" {
				t.Errorf("remote image download leaked authorization header %q", got)
			}
			if r.URL.RawQuery != "token=signed" {
				t.Errorf("image query = %q, want signed token", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte(imageBytes))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	_, err := svc.Start(1, PlaygroundRunRequest{
		ID:           "absolute-image",
		Mode:         "image",
		APIKey:       "sk-image",
		EndpointBase: "/v1",
		Model:        "gpt-image-2",
		Prompt:       "draw a protected image",
		N:            1,
		OutputFormat: "png",
	}, server.URL)
	if err != nil {
		t.Fatalf("start image run: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var completed *PlaygroundRun
	for time.Now().Before(deadline) {
		completed, _ = svc.Get(1, "absolute-image")
		if completed != nil && isTerminalPlaygroundRunStatus(completed.Status) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if completed == nil || completed.Status != PlaygroundRunSucceeded {
		t.Fatalf("completed run = %+v, want succeeded", completed)
	}
	if len(completed.Images) != 1 || completed.Images[0].URL != "" || completed.Images[0].AssetIndex == nil {
		t.Fatalf("image metadata leaked remote URL: %+v", completed.Images)
	}
	encoded, err := json.Marshal(completed)
	if err != nil {
		t.Fatalf("marshal completed run: %v", err)
	}
	if strings.Contains(string(encoded), server.URL) {
		t.Fatalf("completed run leaked upstream image URL: %s", encoded)
	}
	asset, found, err := svc.GetImage(1, "absolute-image", 0)
	if err != nil || !found || string(asset.Data) != imageBytes || asset.ContentType != "image/png" {
		t.Fatalf("image asset = found:%v err:%v type:%q data:%q", found, err, asset.ContentType, asset.Data)
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
		_, err := svc.executeParallelImageRequests(context.Background(), "1:run", PlaygroundRunRequest{}, "", time.Now(), imageCount, func(ctx context.Context) ([]PlaygroundRunImage, error) {
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

func TestPlaygroundVideoGenerationPayloadMapsConfigByProvider(t *testing.T) {
	request := PlaygroundRunRequest{
		Model:       "video-model",
		Prompt:      "city at night",
		N:           1,
		Duration:    8,
		Resolution:  "1080p",
		AspectRatio: "9:16",
	}

	t.Run("Gemini", func(t *testing.T) {
		payload, err := playgroundVideoGenerationPayload(request, true)
		if err != nil {
			t.Fatal(err)
		}
		parameters, ok := payload["parameters"].(map[string]any)
		if !ok {
			t.Fatalf("parameters = %#v, want map", payload["parameters"])
		}
		if got := parameters["durationSeconds"]; got != 8 {
			t.Fatalf("durationSeconds = %#v, want 8", got)
		}
		if got := parameters["resolution"]; got != "1080p" {
			t.Fatalf("resolution = %#v, want 1080p", got)
		}
		if got := parameters["aspectRatio"]; got != "9:16" {
			t.Fatalf("aspectRatio = %#v, want 9:16", got)
		}
	})

	t.Run("OpenAI compatible", func(t *testing.T) {
		payload, err := playgroundVideoGenerationPayload(request, false)
		if err != nil {
			t.Fatal(err)
		}
		if got := payload["duration"]; got != 8 {
			t.Fatalf("duration = %#v, want 8", got)
		}
		if got := payload["resolution"]; got != "1080p" {
			t.Fatalf("resolution = %#v, want 1080p", got)
		}
		if got := payload["aspect_ratio"]; got != "9:16" {
			t.Fatalf("aspect_ratio = %#v, want 9:16", got)
		}
	})

	t.Run("Gemini image to video", func(t *testing.T) {
		request.Images = []PlaygroundRunImageInput{{
			Type:    "image/png",
			DataURL: "data:image/png;base64,QUJD",
		}}
		payload, err := playgroundVideoGenerationPayload(request, true)
		if err != nil {
			t.Fatal(err)
		}
		instances := payload["instances"].([]map[string]any)
		image := instances[0]["image"].(map[string]any)
		if got := image["bytesBase64Encoded"]; got != "QUJD" {
			t.Fatalf("Gemini image bytes = %#v", got)
		}
		if got := image["mimeType"]; got != "image/png" {
			t.Fatalf("Gemini image mime type = %#v", got)
		}
	})

	t.Run("Grok compatible image to video", func(t *testing.T) {
		request.Images = []PlaygroundRunImageInput{{
			Type:    "image/png",
			DataURL: "data:image/png;base64,QUJD",
		}}
		payload, err := playgroundVideoGenerationPayload(request, false)
		if err != nil {
			t.Fatal(err)
		}
		if got := payload["image"]; got != "data:image/png;base64,QUJD" {
			t.Fatalf("Grok image URL = %#v", got)
		}
	})

	t.Run("Grok compatible multi-reference video", func(t *testing.T) {
		request.Images = []PlaygroundRunImageInput{
			{Type: "image/png", DataURL: "data:image/png;base64,QUJD"},
			{Type: "image/jpeg", DataURL: "data:image/jpeg;base64,REVG"},
		}
		payload, err := playgroundVideoGenerationPayload(request, false)
		if err != nil {
			t.Fatal(err)
		}
		references, ok := payload["reference_images"].([]string)
		if !ok || len(references) != 2 {
			t.Fatalf("reference_images = %#v, want two strings", payload["reference_images"])
		}
		if _, exists := payload["image"]; exists {
			t.Fatalf("multi-reference payload unexpectedly contains image: %#v", payload)
		}
	})

	t.Run("frame pair video", func(t *testing.T) {
		request.Images = []PlaygroundRunImageInput{
			{Type: "image/png", DataURL: "data:image/png;base64,QUJD", Frame: "first"},
			{Type: "image/png", DataURL: "data:image/png;base64,REVG", Frame: "last"},
		}
		payload, err := playgroundVideoGenerationPayload(request, false)
		if err != nil {
			t.Fatal(err)
		}
		if got := payload["image"]; got != "data:image/png;base64,QUJD" {
			t.Fatalf("first frame = %#v", got)
		}
		if got := payload["image_tail"]; got != "data:image/png;base64,REVG" {
			t.Fatalf("last frame = %#v", got)
		}
	})

	t.Run("Gemini Omni reference images and video", func(t *testing.T) {
		request.Model = "gemini-omni-flash"
		request.ReferenceVideo = &PlaygroundRunVideoInput{
			Type:            "video/mp4",
			DataURL:         "data:video/mp4;base64,QUJD",
			DurationSeconds: 8,
		}
		payload, err := playgroundVideoGenerationPayload(request, true)
		if err != nil {
			t.Fatal(err)
		}
		instances := payload["instances"].([]map[string]any)
		references, ok := instances[0]["referenceImages"].([]map[string]any)
		if !ok || len(references) != 2 {
			t.Fatalf("Gemini referenceImages = %#v", instances[0]["referenceImages"])
		}
		video, ok := instances[0]["video"].(map[string]any)
		if !ok || video["bytesBase64Encoded"] != "QUJD" || video["mimeType"] != "video/mp4" {
			t.Fatalf("Gemini reference video = %#v", instances[0]["video"])
		}
	})
}

func TestNormalizePlaygroundVideoRequestAppliesNamedModelRules(t *testing.T) {
	t.Run("grok-video-10 downgrades multi-reference duration", func(t *testing.T) {
		request := PlaygroundRunRequest{
			Model:      "grok-video-10",
			Duration:   16,
			Resolution: "720p",
			Images: []PlaygroundRunImageInput{
				{DataURL: "data:image/png;base64,QUJD"},
				{DataURL: "data:image/png;base64,REVG"},
			},
		}
		if err := normalizePlaygroundVideoRequest(&request); err != nil {
			t.Fatal(err)
		}
		if request.Duration != 10 {
			t.Fatalf("duration = %d, want 10", request.Duration)
		}
	})

	t.Run("grok-video-r rejects more than seven images", func(t *testing.T) {
		request := PlaygroundRunRequest{Model: "grok-video-r", Duration: 6}
		for range 8 {
			request.Images = append(request.Images, PlaygroundRunImageInput{DataURL: "data:image/png;base64,QUJD"})
		}
		if err := normalizePlaygroundVideoRequest(&request); err == nil {
			t.Fatal("expected reference image limit error")
		}
	})

	t.Run("kling accepts five, ten, or fifteen seconds", func(t *testing.T) {
		request := PlaygroundRunRequest{Model: "kling-v1", Duration: 8}
		if err := normalizePlaygroundVideoRequest(&request); err == nil {
			t.Fatal("expected Kling duration validation error")
		}
		request.Duration = 15
		if err := normalizePlaygroundVideoRequest(&request); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Seedance supports a two to twelve second frame pair", func(t *testing.T) {
		request := PlaygroundRunRequest{Model: "doubao-seedance-1-5-pro", Duration: 12, Images: []PlaygroundRunImageInput{
			{DataURL: "data:image/png;base64,QUJD", Frame: "first"},
			{DataURL: "data:image/png;base64,REVG", Frame: "last"},
		}}
		if err := normalizePlaygroundVideoRequest(&request); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Seedance 2.0 and 2.5 use their versioned duration limits", func(t *testing.T) {
		request := PlaygroundRunRequest{Model: "doubao-seedance-2.0", Duration: 15}
		if err := normalizePlaygroundVideoRequest(&request); err != nil {
			t.Fatal(err)
		}
		request.Duration = 16
		if err := normalizePlaygroundVideoRequest(&request); err == nil {
			t.Fatal("expected Seedance 2.0 duration validation error")
		}
		request.Model = "doubao-seedance-2.5"
		request.Duration = 30
		if err := normalizePlaygroundVideoRequest(&request); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Gemini Omni rejects forbidden prompt and long reference video", func(t *testing.T) {
		request := PlaygroundRunRequest{Model: "gemini-omni-flash", Duration: 10, Prompt: "生成 16:9 的分镜视频"}
		if err := normalizePlaygroundVideoRequest(&request); err == nil || !strings.Contains(err.Error(), "aspect ratios") {
			t.Fatalf("prompt validation error = %v", err)
		}
		request.Prompt = "人物走入街道"
		request.ReferenceVideo = &PlaygroundRunVideoInput{DataURL: "data:video/mp4;base64,QUJD", DurationSeconds: 10.5}
		if err := normalizePlaygroundVideoRequest(&request); err == nil || !strings.Contains(err.Error(), "reference video") {
			t.Fatalf("reference video validation error = %v", err)
		}
	})
}

func TestPlaygroundRunServiceExecuteVideoPollsGrokTaskAndStoresProtectedContent(t *testing.T) {
	var statusCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-grok" {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos/generations":
			_, _ = w.Write([]byte(`{"request_id":"req-grok-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/req-grok-1":
			if statusCalls.Add(1) == 1 {
				_, _ = w.Write([]byte(`{"status":"pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"done","video":{"url":"/v1/videos/req-grok-1/content"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/req-grok-1/content":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("grok-video-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.videoPollInterval = time.Millisecond
	key := "7:grok-video"
	svc.runs[key] = &PlaygroundRun{ID: "grok-video", UserID: 7, Mode: "video", Status: PlaygroundRunRunning}
	_, err := svc.executeVideo(context.Background(), key, PlaygroundRunRequest{
		APIKey:   "sk-grok",
		Platform: PlatformGrok,
		Model:    "grok-imagine-video",
		Prompt:   "waves",
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute video: %v", err)
	}
	if statusCalls.Load() != 2 {
		t.Fatalf("status calls = %d, want 2", statusCalls.Load())
	}
	video := svc.runs[key].Videos[0]
	if got := string(video.data); got != "grok-video-bytes" {
		t.Fatalf("video data = %q", got)
	}
	if video.MimeType != "video/mp4" {
		t.Fatalf("mime type = %q", video.MimeType)
	}
}

func TestPlaygroundRunServiceExecuteVideoUsesGeminiLongRunningOperation(t *testing.T) {
	operationName := "models/veo-3.1-generate-preview/operations/op-123"
	encodedOperation := "bW9kZWxzL3Zlby0zLjEtZ2VuZXJhdGUtcHJldmlldy9vcGVyYXRpb25zL29wLTEyMw"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-gemini" {
			t.Errorf("authorization = %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1beta/models/veo-3.1-generate-preview:predictLongRunning":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode submit body: %v", err)
			}
			if _, ok := body["instances"]; !ok {
				t.Errorf("submit body missing instances: %#v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"name":%q}`, operationName)
		case r.Method == http.MethodGet && r.URL.Path == "/v1beta/video-operations/"+encodedOperation:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"done":true,"response":{"generateVideoResponse":{"generatedSamples":[{"video":{"uri":"https://generativelanguage.googleapis.com/v1beta/files/video-1:download"}}]}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1beta/video-operations/"+encodedOperation+"/content":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("veo-video-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.videoPollInterval = time.Millisecond
	key := "8:veo-video"
	svc.runs[key] = &PlaygroundRun{ID: "veo-video", UserID: 8, Mode: "video", Status: PlaygroundRunRunning}
	_, err := svc.executeVideo(context.Background(), key, PlaygroundRunRequest{
		APIKey:   "sk-gemini",
		Platform: PlatformGemini,
		Model:    "veo-3.1-generate-preview",
		Prompt:   "sunrise",
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute Gemini video: %v", err)
	}
	if got := string(svc.runs[key].Videos[0].data); got != "veo-video-bytes" {
		t.Fatalf("video data = %q", got)
	}
}

func TestPlaygroundRunServiceExecuteVideoFallsBackToOpenAIContentEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-openai" {
			t.Errorf("authorization = %q", got)
		}
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/videos/generations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"video-openai-1","status":"queued"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/video-openai-1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"video-openai-1","status":"completed"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1/videos/video-openai-1/content":
			w.Header().Set("Content-Type", "video/mp4")
			_, _ = w.Write([]byte("openai-video-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	svc.videoPollInterval = time.Millisecond
	key := "10:openai-video"
	svc.runs[key] = &PlaygroundRun{ID: "openai-video", UserID: 10, Mode: "video", Status: PlaygroundRunRunning}
	_, err := svc.executeVideo(context.Background(), key, PlaygroundRunRequest{
		APIKey:   "sk-openai",
		Platform: PlatformOpenAI,
		Model:    "sora-2",
		Prompt:   "ocean waves",
	}, server.URL, time.Now())
	if err != nil {
		t.Fatalf("execute OpenAI video: %v", err)
	}
	if got := string(svc.runs[key].Videos[0].data); got != "openai-video-bytes" {
		t.Fatalf("video data = %q", got)
	}
}

func TestPlaygroundVideoTaskStateSupportsSeedanceArkResponse(t *testing.T) {
	var payload any
	decoder := json.NewDecoder(strings.NewReader(`{"id":"task-1","status":"completed","video":{"url":"https://example.test/video.mp4"}}`))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		t.Fatal(err)
	}
	state, message := playgroundVideoTaskState(payload, false)
	if state != playgroundVideoTaskSucceeded || message != "" {
		t.Fatalf("state = %v, message = %q", state, message)
	}
	videos := extractPlaygroundVideosFromAny(payload)
	if len(videos) != 1 || videos[0].URL != "https://example.test/video.mp4" {
		t.Fatalf("videos = %#v", videos)
	}
}

func TestPlaygroundRunServiceStoresAbsoluteVideoURLAsProtectedAsset(t *testing.T) {
	assetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("absolute video download leaked authorization header %q", got)
		}
		if r.URL.RawQuery != "token=signed" {
			t.Errorf("video query = %q, want signed token", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("seedance-video-bytes"))
	}))
	defer assetServer.Close()

	svc := NewPlaygroundRunService()
	key := "12:seedance-video"
	svc.runs[key] = &PlaygroundRun{ID: "seedance-video", UserID: 12, Mode: "video", Status: PlaygroundRunRunning}
	err := svc.commitPlaygroundVideos(context.Background(), key, PlaygroundRunRequest{APIKey: "sk-user-key"}, assetServer.URL, time.Now(), []PlaygroundRunVideo{{
		URL:          assetServer.URL + "/video.mp4?token=signed",
		ThumbnailURL: "https://provider.example/cover.jpg",
	}})
	if err != nil {
		t.Fatalf("commit absolute video: %v", err)
	}
	svc.update(key, func(run *PlaygroundRun) { run.Status = PlaygroundRunSucceeded })

	run, found := svc.Get(12, "seedance-video")
	if !found || len(run.Videos) != 1 {
		t.Fatalf("stored run = %#v, found=%v", run, found)
	}
	video := run.Videos[0]
	if video.URL != "" || video.ThumbnailURL != "" || video.AssetIndex == nil || *video.AssetIndex != 0 {
		t.Fatalf("client video metadata leaked provider URL: %#v", video)
	}
	asset, found, err := svc.GetVideo(12, "seedance-video", 0)
	if err != nil || !found || string(asset.Data) != "seedance-video-bytes" || asset.ContentType != "video/mp4" {
		t.Fatalf("stored video asset = %#v, found=%v, err=%v", asset, found, err)
	}
}

func TestPlaygroundRunServiceRejectsJSONVideoAsset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error":"expired signed URL"}`))
	}))
	defer server.Close()

	svc := NewPlaygroundRunService()
	_, _, err := svc.downloadPlaygroundVideo(context.Background(), server.URL, "")
	if err == nil || !strings.Contains(err.Error(), "supported video") {
		t.Fatalf("download video error = %v, want unsupported video error", err)
	}
}

func TestPlaygroundRunServicePersistsVideoAssetsAcrossInstances(t *testing.T) {
	mini := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })

	now := time.Now()
	writer := ProvidePlaygroundRunService(redisClient, nil)
	run := &PlaygroundRun{
		ID: "persisted-video", UserID: 9, Mode: "video", Status: PlaygroundRunSucceeded,
		CreatedAt: now, UpdatedAt: now, CompletedAt: &now,
		Videos: []PlaygroundRunVideo{{MimeType: "video/mp4", data: []byte("video-bytes")}},
	}
	if err := writer.persistRun(run); err != nil {
		t.Fatalf("persist video run: %v", err)
	}
	reader := ProvidePlaygroundRunService(redisClient, nil)
	stored, found := reader.Get(9, run.ID)
	if !found || len(stored.Videos) != 1 || stored.Videos[0].AssetIndex == nil {
		t.Fatalf("stored video metadata = %#v", stored)
	}
	asset, found, err := reader.GetVideo(9, run.ID, 0)
	if err != nil || !found || string(asset.Data) != "video-bytes" || asset.ContentType != "video/mp4" {
		t.Fatalf("video asset = %#v, found=%v, err=%v", asset, found, err)
	}
}

type playgroundVideoAssetRepositoryStub struct {
	asset   *PlaygroundVideoAssetMetadata
	upserts []PlaygroundVideoAssetMetadata
}

func (r *playgroundVideoAssetRepositoryStub) Upsert(_ context.Context, asset PlaygroundVideoAssetMetadata) error {
	r.upserts = append(r.upserts, asset)
	r.asset = &asset
	return nil
}

func (r *playgroundVideoAssetRepositoryStub) Get(_ context.Context, userID int64, runID string, assetIndex int) (*PlaygroundVideoAssetMetadata, error) {
	if r.asset == nil || r.asset.UserID != userID || r.asset.RunID != runID || r.asset.AssetIndex != assetIndex {
		return nil, nil
	}
	copy := *r.asset
	return &copy, nil
}

func TestPlaygroundRunServiceRecoversVideoFromPersistedProviderURL(t *testing.T) {
	assetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("persisted provider URL received authorization %q", got)
		}
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("persisted-provider-video"))
	}))
	defer assetServer.Close()

	repo := &playgroundVideoAssetRepositoryStub{asset: &PlaygroundVideoAssetMetadata{
		UserID: 21, RunID: "expired-run", AssetIndex: 0,
		SourceURL: assetServer.URL + "/result.mp4", MimeType: "video/mp4",
	}}
	svc := newPlaygroundRunService(nil, repo)
	asset, found, err := svc.GetVideoContext(context.Background(), 21, "expired-run", 0)
	if err != nil || !found {
		t.Fatalf("GetVideoContext found=%v err=%v", found, err)
	}
	if got := string(asset.Data); got != "persisted-provider-video" {
		t.Fatalf("video data = %q", got)
	}
}

func TestSanitizePlaygroundUpstreamErrorMessageRemovesGroupPrefix(t *testing.T) {
	got := sanitizePlaygroundUpstreamErrorMessage("分组 default 下模型 grok-video-r 的可用渠道不存在（retry）")
	want := "grok-video-r 的可用渠道不存在（retry）"
	if got != want {
		t.Fatalf("sanitized message = %q, want %q", got, want)
	}

	plain := "No eligible Grok media accounts"
	if got := sanitizePlaygroundUpstreamErrorMessage(plain); got != plain {
		t.Fatalf("plain message changed to %q", got)
	}
}
