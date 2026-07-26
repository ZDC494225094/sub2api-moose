package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	playgroundRunRedisKeyPrefix = "playground:run:"
	playgroundRunRedisImageKey  = ":image:"
	playgroundRunRedisTimeout   = 3 * time.Second
)

type PlaygroundRunStatus string

const (
	PlaygroundRunQueued    PlaygroundRunStatus = "queued"
	PlaygroundRunRunning   PlaygroundRunStatus = "running"
	PlaygroundRunSucceeded PlaygroundRunStatus = "succeeded"
	PlaygroundRunFailed    PlaygroundRunStatus = "failed"
	PlaygroundRunCanceled  PlaygroundRunStatus = "canceled"
)

type PlaygroundRunRequest struct {
	ID               string                     `json:"id"`
	Mode             string                     `json:"mode"`
	APIKey           string                     `json:"apiKey"`
	Platform         string                     `json:"platform"`
	EndpointBase     string                     `json:"endpointBase"`
	DisplayEndpoint  string                     `json:"displayEndpoint"`
	Model            string                     `json:"model"`
	Messages         []PlaygroundRunChatMessage `json:"messages"`
	Temperature      *float64                   `json:"temperature"`
	TopP             *float64                   `json:"topP"`
	MaxTokens        *int                       `json:"maxTokens"`
	PresencePenalty  *float64                   `json:"presencePenalty"`
	FrequencyPenalty *float64                   `json:"frequencyPenalty"`
	Prompt           string                     `json:"prompt"`
	Size             string                     `json:"size"`
	N                int                        `json:"n"`
	Quality          string                     `json:"quality"`
	Background       string                     `json:"background"`
	OutputFormat     string                     `json:"outputFormat"`
	Images           []PlaygroundRunImageInput  `json:"images"`
}

type PlaygroundRunChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type PlaygroundRunImageInput struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	DataURL string `json:"dataUrl"`
}

type PlaygroundRunImage struct {
	URL           string `json:"url,omitempty"`
	RevisedPrompt string `json:"revisedPrompt,omitempty"`
	AssetIndex    *int   `json:"assetIndex,omitempty"`
	MimeType      string `json:"mimeType,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`

	data []byte
}

type PlaygroundRunImageAsset struct {
	Data        []byte
	ContentType string
}

type playgroundImageUpstreamResponse struct {
	Data []struct {
		B64JSON       []byte `json:"b64_json"`
		URL           string `json:"url"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
}

type PlaygroundRun struct {
	ID          string               `json:"id"`
	UserID      int64                `json:"-"`
	Mode        string               `json:"mode"`
	Status      PlaygroundRunStatus  `json:"status"`
	Model       string               `json:"model,omitempty"`
	Content     string               `json:"content,omitempty"`
	Images      []PlaygroundRunImage `json:"images,omitempty"`
	Error       string               `json:"error,omitempty"`
	Raw         json.RawMessage      `json:"raw,omitempty"`
	CreatedAt   time.Time            `json:"createdAt"`
	UpdatedAt   time.Time            `json:"updatedAt"`
	CompletedAt *time.Time           `json:"completedAt,omitempty"`
	DurationMs  int64                `json:"durationMs,omitempty"`

	cancel context.CancelFunc
}

type PlaygroundRunService struct {
	mu         sync.RWMutex
	runs       map[string]*PlaygroundRun
	httpClient *http.Client
	rdb        *redis.Client
	ttl        time.Duration
}

func NewPlaygroundRunService() *PlaygroundRunService {
	return newPlaygroundRunService(nil)
}

// ProvidePlaygroundRunService keeps completed Playground tasks available when a
// browser refreshes or a subsequent request lands on a different API instance.
func ProvidePlaygroundRunService(rdb *redis.Client) *PlaygroundRunService {
	return newPlaygroundRunService(rdb)
}

func newPlaygroundRunService(rdb *redis.Client) *PlaygroundRunService {
	return &PlaygroundRunService{
		runs: make(map[string]*PlaygroundRun),
		httpClient: &http.Client{
			Timeout: 0,
		},
		rdb: rdb,
		ttl: 6 * time.Hour,
	}
}

func (s *PlaygroundRunService) Start(userID int64, request PlaygroundRunRequest, baseURL string) (*PlaygroundRun, error) {
	if s == nil {
		return nil, errors.New("playground run service is not available")
	}
	if userID <= 0 {
		return nil, errors.New("user is not authenticated")
	}
	request.ID = strings.TrimSpace(request.ID)
	if request.ID == "" {
		request.ID = "run-" + uuid.NewString()
	}
	if len(request.ID) > 128 {
		return nil, errors.New("run id is too long")
	}
	request.Mode = strings.TrimSpace(request.Mode)
	if request.Mode != "chat" && request.Mode != "image" {
		return nil, errors.New("unsupported playground run mode")
	}
	request.APIKey = strings.TrimSpace(request.APIKey)
	if request.APIKey == "" {
		return nil, errors.New("api key is required")
	}
	request.Model = strings.TrimSpace(request.Model)
	if request.Model == "" {
		return nil, errors.New("model is required")
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, errors.New("request base url is required")
	}

	key := playgroundRunKey(userID, request.ID)
	s.cleanupExpiredLocked(time.Now())
	s.mu.Lock()
	if existing := s.runs[key]; existing != nil {
		out := clonePlaygroundRunForClient(existing)
		s.mu.Unlock()
		return out, nil
	}
	s.mu.Unlock()

	now := time.Now()
	run := &PlaygroundRun{
		ID:        request.ID,
		UserID:    userID,
		Mode:      request.Mode,
		Status:    PlaygroundRunQueued,
		Model:     request.Model,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if existing, claimed, err := s.claimPersistentRun(run); err != nil {
		log.Printf("playground run persistence unavailable for user %d run %s: %v", userID, request.ID, err)
	} else if !claimed {
		return existing, nil
	}

	s.mu.Lock()
	if existing := s.runs[key]; existing != nil {
		out := clonePlaygroundRunForClient(existing)
		s.mu.Unlock()
		return out, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	run.cancel = cancel
	s.runs[key] = run
	out := clonePlaygroundRunForClient(run)
	s.mu.Unlock()

	go s.execute(ctx, key, request, baseURL)

	return out, nil
}

func (s *PlaygroundRunService) Get(userID int64, id string) (*PlaygroundRun, bool) {
	if s == nil || userID <= 0 {
		return nil, false
	}
	id = strings.TrimSpace(id)
	if run, found, err := s.loadPersistentRun(userID, id); err != nil {
		log.Printf("playground run persistence read failed for user %d run %s: %v", userID, id, err)
	} else if found {
		return clonePlaygroundRunForClient(run), true
	}
	key := playgroundRunKey(userID, id)
	s.mu.RLock()
	run := s.runs[key]
	if run == nil {
		s.mu.RUnlock()
		return nil, false
	}
	out := clonePlaygroundRunForClient(run)
	s.mu.RUnlock()
	return out, true
}

func (s *PlaygroundRunService) GetImage(userID int64, id string, index int) (PlaygroundRunImageAsset, bool, error) {
	if s == nil || userID <= 0 || index < 0 {
		return PlaygroundRunImageAsset{}, false, nil
	}
	id = strings.TrimSpace(id)
	key := playgroundRunKey(userID, id)
	s.mu.RLock()
	run := s.runs[key]
	var image PlaygroundRunImage
	found := false
	if run != nil && run.Status == PlaygroundRunSucceeded && index < len(run.Images) {
		image = run.Images[index]
		image.data = append([]byte(nil), image.data...)
		found = true
	}
	s.mu.RUnlock()
	if !found {
		persistedRun, persisted, err := s.loadPersistentRun(userID, id)
		if err != nil {
			return PlaygroundRunImageAsset{}, false, err
		}
		if !persisted || persistedRun.Status != PlaygroundRunSucceeded || index >= len(persistedRun.Images) {
			return PlaygroundRunImageAsset{}, false, nil
		}
		image = persistedRun.Images[index]
	}

	if len(image.data) > 0 {
		return playgroundRunImageAsset(image.data, image.MimeType), true, nil
	}
	if asset, persisted, err := s.loadPersistentImage(userID, id, index, image.MimeType); err != nil || persisted {
		return asset, persisted, err
	}

	if strings.HasPrefix(strings.TrimSpace(image.URL), "data:") {
		data, contentType, err := decodePlaygroundImageDataURL(image.URL)
		if err != nil {
			return PlaygroundRunImageAsset{}, true, err
		}
		return PlaygroundRunImageAsset{Data: data, ContentType: playgroundImageContentType(data, contentType)}, true, nil
	}
	return PlaygroundRunImageAsset{}, true, errors.New("playground image is not available as a local asset")
}

func (s *PlaygroundRunService) Cancel(userID int64, id string) (*PlaygroundRun, bool) {
	if s == nil || userID <= 0 {
		return nil, false
	}
	id = strings.TrimSpace(id)
	key := playgroundRunKey(userID, id)
	s.mu.Lock()
	run := s.runs[key]
	if run != nil {
		if run.cancel != nil && !isTerminalPlaygroundRunStatus(run.Status) {
			run.cancel()
		}
		imageCount := len(run.Images)
		canceled := !isTerminalPlaygroundRunStatus(run.Status)
		cancelPlaygroundRun(run, time.Now())
		snapshot := clonePlaygroundRunForPersistence(run)
		out := clonePlaygroundRunForClient(run)
		s.mu.Unlock()
		if err := s.persistRun(snapshot); err != nil {
			log.Printf("playground run cancellation persistence failed for user %d run %s: %v", userID, id, err)
		}
		if canceled {
			s.deletePersistentImages(userID, id, imageCount)
		}
		return out, true
	}
	s.mu.Unlock()

	persistedRun, found, err := s.loadPersistentRun(userID, id)
	if err != nil || !found {
		return nil, false
	}
	imageCount := len(persistedRun.Images)
	canceled := !isTerminalPlaygroundRunStatus(persistedRun.Status)
	cancelPlaygroundRun(persistedRun, time.Now())
	if err := s.persistRun(persistedRun); err != nil {
		log.Printf("playground run cancellation persistence failed for user %d run %s: %v", userID, id, err)
	}
	if canceled {
		s.deletePersistentImages(userID, id, imageCount)
	}
	return clonePlaygroundRunForClient(persistedRun), true
}

func (s *PlaygroundRunService) execute(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string) {
	started := time.Now()
	s.update(key, func(run *PlaygroundRun) {
		run.Status = PlaygroundRunRunning
		run.UpdatedAt = started
	})

	var raw json.RawMessage
	var err error
	switch request.Mode {
	case "image":
		raw, err = s.executeImage(ctx, key, request, baseURL, started)
	default:
		raw, err = s.executeChat(ctx, key, request, baseURL)
	}

	completed := time.Now()
	if err != nil {
		status := PlaygroundRunFailed
		if errors.Is(err, context.Canceled) {
			status = PlaygroundRunCanceled
		}
		s.update(key, func(run *PlaygroundRun) {
			if isTerminalPlaygroundRunStatus(run.Status) && run.Status == PlaygroundRunCanceled {
				return
			}
			run.Status = status
			run.Error = err.Error()
			run.UpdatedAt = completed
			run.CompletedAt = &completed
			run.DurationMs = completed.Sub(started).Milliseconds()
		})
		return
	}

	s.update(key, func(run *PlaygroundRun) {
		if isTerminalPlaygroundRunStatus(run.Status) {
			return
		}
		run.Status = PlaygroundRunSucceeded
		if request.Mode == "image" {
			run.Raw = nil
		} else {
			run.Raw = raw
		}
		run.UpdatedAt = completed
		run.CompletedAt = &completed
		run.DurationMs = completed.Sub(started).Milliseconds()
	})
}

func (s *PlaygroundRunService) executeChat(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string) (json.RawMessage, error) {
	payload := map[string]any{
		"model":    request.Model,
		"messages": filterPlaygroundChatMessages(request.Messages),
		"stream":   true,
	}
	if request.Temperature != nil {
		payload["temperature"] = *request.Temperature
	}
	if request.TopP != nil {
		payload["top_p"] = *request.TopP
	}
	if request.MaxTokens != nil && *request.MaxTokens > 0 {
		payload["max_tokens"] = *request.MaxTokens
	}
	if request.PresencePenalty != nil {
		payload["presence_penalty"] = *request.PresencePenalty
	}
	if request.FrequencyPenalty != nil {
		payload["frequency_penalty"] = *request.FrequencyPenalty
	}

	endpointURL, err := buildPlaygroundRunEndpointURL(baseURL, request.EndpointBase, "/v1/chat/completions")
	if err != nil {
		return nil, err
	}
	responseBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(responseBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+request.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parsePlaygroundUpstreamError(resp)
	}

	var events []json.RawMessage
	var content strings.Builder
	if err := readPlaygroundChatStream(resp.Body, func(raw json.RawMessage, delta string) {
		if len(raw) > 0 {
			events = append(events, append(json.RawMessage(nil), raw...))
		}
		if delta == "" {
			return
		}
		content.WriteString(delta)
		s.update(key, func(run *PlaygroundRun) {
			run.Content += delta
			run.UpdatedAt = time.Now()
		})
	}); err != nil {
		return nil, err
	}

	if content.Len() == 0 {
		rawBody, err := json.Marshal(events)
		if err == nil {
			if text := extractPlaygroundChatTextFromRaw(rawBody); text != "" {
				s.update(key, func(run *PlaygroundRun) {
					run.Content = text
					run.UpdatedAt = time.Now()
				})
			}
		}
	}

	raw, err := json.Marshal(events)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (s *PlaygroundRunService) executeImage(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string, started time.Time) (json.RawMessage, error) {
	imageCount := request.N
	if imageCount <= 0 {
		imageCount = 1
	}
	if imageCount > 4 {
		imageCount = 4
	}
	outputFormat := strings.TrimSpace(request.OutputFormat)
	if outputFormat == "" {
		outputFormat = "png"
	}
	imageInputs := filterPlaygroundImageInputs(request.Images)
	if isGiteeImageGenerationModel(request.Model) {
		return s.executeGiteeZImage(ctx, key, request, baseURL, started, imageInputs, outputFormat, imageCount)
	}
	if isGeminiImageGenerationModel(request.Model) && playgroundImageUsesGeminiNativeAPI(request) {
		return s.executeGeminiNativeImage(ctx, key, request, baseURL, started, imageInputs, outputFormat, imageCount)
	}
	if len(imageInputs) > 0 {
		return s.executeImageEdit(ctx, key, request, baseURL, started, imageInputs, outputFormat, imageCount)
	}
	payload := map[string]any{
		"model":           request.Model,
		"prompt":          request.Prompt,
		"size":            playgroundImageRequestSize(request.Model, request.Size),
		"n":               1,
		"response_format": "b64_json",
	}
	if request.Quality != "" && request.Quality != "auto" {
		payload["quality"] = request.Quality
	}
	if request.Background != "" && request.Background != "auto" {
		payload["background"] = request.Background
	}
	if outputFormat != "" {
		payload["output_format"] = outputFormat
	}

	endpointURL, err := buildPlaygroundRunEndpointURL(baseURL, request.EndpointBase, "/v1/images/generations")
	if err != nil {
		return nil, err
	}
	responseBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return s.executeParallelImageRequests(ctx, key, started, imageCount, func(requestCtx context.Context) ([]PlaygroundRunImage, error) {
		return s.executeImageRequest(requestCtx, endpointURL, request.APIKey, "application/json", bytes.NewReader(responseBody), outputFormat)
	})
}

func (s *PlaygroundRunService) executeGiteeZImage(
	ctx context.Context,
	key string,
	request PlaygroundRunRequest,
	baseURL string,
	started time.Time,
	images []PlaygroundRunImageInput,
	outputFormat string,
	imageCount int,
) (json.RawMessage, error) {
	payload := map[string]any{
		"model":                 request.Model,
		"prompt":                request.Prompt,
		"num_images_per_prompt": 1,
		"negative_prompt":       "blurry ugly bad",
		"num_inference_steps":   9,
		"seed":                  0,
		"guidance_scale":        1,
	}
	if len(images) > 0 {
		data, _, err := decodePlaygroundImageDataURL(images[0].DataURL)
		if err != nil {
			return nil, err
		}
		payload["control_image"] = base64.StdEncoding.EncodeToString(data)
		payload["control_mode"] = "HED"
		payload["control_context_scale"] = 0.75
		payload["image_scale"] = 1
	}

	endpointURL, err := buildPlaygroundRunEndpointURL(baseURL, request.EndpointBase, "/v1/images/generations")
	if err != nil {
		return nil, err
	}
	responseBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return s.executeParallelImageRequests(ctx, key, started, imageCount, func(requestCtx context.Context) ([]PlaygroundRunImage, error) {
		return s.executeImageRequest(requestCtx, endpointURL, request.APIKey, "application/json", bytes.NewReader(responseBody), outputFormat)
	})
}

func (s *PlaygroundRunService) executeGeminiNativeImage(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string, started time.Time, images []PlaygroundRunImageInput, outputFormat string, imageCount int) (json.RawMessage, error) {
	parts := make([]any, 0, 1+len(images))
	if prompt := strings.TrimSpace(request.Prompt); prompt != "" {
		parts = append(parts, map[string]any{"text": prompt})
	}
	for _, image := range images {
		data, contentType, err := decodePlaygroundImageDataURL(image.DataURL)
		if err != nil {
			return nil, err
		}
		if requestContentType := strings.TrimSpace(image.Type); strings.HasPrefix(strings.ToLower(requestContentType), "image/") {
			contentType = requestContentType
		}
		parts = append(parts, map[string]any{
			"inlineData": map[string]any{
				"mimeType": contentType,
				"data":     base64.StdEncoding.EncodeToString(data),
			},
		})
	}
	if len(parts) == 0 {
		parts = append(parts, map[string]any{"text": ""})
	}
	generationConfig := map[string]any{"responseModalities": []string{"TEXT", "IMAGE"}}
	if aspectRatio := playgroundImageAspectRatio(request.Size); aspectRatio != "" {
		generationConfig["imageConfig"] = map[string]any{"aspectRatio": aspectRatio}
	}
	payload := map[string]any{
		"contents":         []any{map[string]any{"role": "user", "parts": parts}},
		"generationConfig": generationConfig,
	}
	model := strings.TrimSpace(request.Model)
	if strings.ContainsAny(model, "/\\") {
		return nil, errors.New("invalid Gemini image model")
	}
	endpoint := "/v1beta/models/" + model + ":generateContent"
	endpointURL, err := buildPlaygroundRunEndpointURL(baseURL, playgroundImageEndpointBase(request), endpoint)
	if err != nil {
		return nil, err
	}
	responseBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return s.executeParallelImageRequests(ctx, key, started, imageCount, func(requestCtx context.Context) ([]PlaygroundRunImage, error) {
		return s.executeGeminiImageRequest(requestCtx, endpointURL, request.APIKey, responseBody, outputFormat)
	})
}

func (s *PlaygroundRunService) executeGeminiImageRequest(ctx context.Context, endpointURL, apiKey string, body []byte, outputFormat string) ([]PlaygroundRunImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parsePlaygroundUpstreamError(resp)
	}
	limitedBody := &io.LimitedReader{R: resp.Body, N: 128<<20 + 1}
	var payload any
	if err := json.NewDecoder(limitedBody).Decode(&payload); err != nil {
		return nil, err
	}
	if limitedBody.N <= 0 {
		return nil, errors.New("playground image response exceeds 128 MiB")
	}
	images := extractPlaygroundImagesFromAny(payload, outputFormat)
	if len(images) == 0 {
		return nil, errors.New("no images returned by image model")
	}
	return images, nil
}

func (s *PlaygroundRunService) executeImageEdit(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string, started time.Time, images []PlaygroundRunImageInput, outputFormat string, imageCount int) (json.RawMessage, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("model", request.Model); err != nil {
		return nil, err
	}
	if err := writer.WriteField("prompt", request.Prompt); err != nil {
		return nil, err
	}
	if err := writer.WriteField("size", playgroundImageRequestSize(request.Model, request.Size)); err != nil {
		return nil, err
	}
	if err := writer.WriteField("n", "1"); err != nil {
		return nil, err
	}
	if err := writer.WriteField("response_format", "b64_json"); err != nil {
		return nil, err
	}
	if request.Quality != "" && request.Quality != "auto" {
		if err := writer.WriteField("quality", request.Quality); err != nil {
			return nil, err
		}
	}
	if request.Background != "" && request.Background != "auto" {
		if err := writer.WriteField("background", request.Background); err != nil {
			return nil, err
		}
	}
	if outputFormat != "" {
		if err := writer.WriteField("output_format", outputFormat); err != nil {
			return nil, err
		}
	}

	for _, image := range images {
		data, contentType, err := decodePlaygroundImageDataURL(image.DataURL)
		if err != nil {
			return nil, err
		}
		if requestContentType := strings.TrimSpace(image.Type); strings.HasPrefix(strings.ToLower(requestContentType), "image/") {
			contentType = requestContentType
		}
		fileName := sanitizePlaygroundUploadFileName(image.Name, contentType)
		part, err := writer.CreatePart(playgroundImageMultipartHeader("image", fileName, contentType))
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(data); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	contentType := writer.FormDataContentType()

	endpointURL, err := buildPlaygroundRunEndpointURL(baseURL, request.EndpointBase, "/v1/images/edits")
	if err != nil {
		return nil, err
	}
	bodyBytes := append([]byte(nil), body.Bytes()...)
	return s.executeParallelImageRequests(ctx, key, started, imageCount, func(requestCtx context.Context) ([]PlaygroundRunImage, error) {
		return s.executeImageRequest(requestCtx, endpointURL, request.APIKey, contentType, bytes.NewReader(bodyBytes), outputFormat)
	})
}

type playgroundImageRequestResult struct {
	images []PlaygroundRunImage
}

func (s *PlaygroundRunService) executeParallelImageRequests(
	ctx context.Context,
	key string,
	started time.Time,
	imageCount int,
	execute func(context.Context) ([]PlaygroundRunImage, error),
) (json.RawMessage, error) {
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]playgroundImageRequestResult, imageCount)
	errorsCh := make(chan error, imageCount)
	var wg sync.WaitGroup
	for index := 0; index < imageCount; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			images, err := execute(requestCtx)
			if err != nil {
				errorsCh <- err
				cancel()
				return
			}
			results[index] = playgroundImageRequestResult{images: images}
		}()
	}
	wg.Wait()
	close(errorsCh)
	if err := <-errorsCh; err != nil {
		return nil, err
	}
	if err := requestCtx.Err(); err != nil {
		return nil, err
	}

	allImages := make([]PlaygroundRunImage, 0, imageCount)
	for _, result := range results {
		allImages = append(allImages, result.images...)
	}

	updated := false
	s.update(key, func(run *PlaygroundRun) {
		if isTerminalPlaygroundRunStatus(run.Status) {
			return
		}
		run.Images = allImages
		run.Content = ""
		run.DurationMs = time.Since(started).Milliseconds()
		run.UpdatedAt = time.Now()
		updated = true
	})
	if !updated {
		return nil, context.Canceled
	}
	return nil, nil
}

func (s *PlaygroundRunService) executeImageRequest(
	ctx context.Context,
	endpointURL string,
	apiKey string,
	contentType string,
	body io.Reader,
	outputFormat string,
) ([]PlaygroundRunImage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", contentType)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parsePlaygroundUpstreamError(resp)
	}
	limitedBody := &io.LimitedReader{R: resp.Body, N: 128<<20 + 1}
	var payload playgroundImageUpstreamResponse
	if err := json.NewDecoder(limitedBody).Decode(&payload); err != nil {
		return nil, err
	}
	if limitedBody.N <= 0 {
		return nil, errors.New("playground image response exceeds 128 MiB")
	}
	return extractPlaygroundImages(payload, outputFormat), nil
}

func (s *PlaygroundRunService) update(key string, fn func(*PlaygroundRun)) {
	s.mu.Lock()
	var snapshot *PlaygroundRun
	if run := s.runs[key]; run != nil {
		previousStatus := run.Status
		fn(run)
		if run.Mode == "image" || previousStatus != run.Status || isTerminalPlaygroundRunStatus(run.Status) {
			snapshot = clonePlaygroundRunForPersistence(run)
		}
	}
	s.mu.Unlock()
	if snapshot != nil {
		if err := s.persistRun(snapshot); err != nil {
			log.Printf("playground run persistence update failed for user %d run %s: %v", snapshot.UserID, snapshot.ID, err)
		}
	}
}

func (s *PlaygroundRunService) claimPersistentRun(run *PlaygroundRun) (*PlaygroundRun, bool, error) {
	if s.rdb == nil {
		return nil, true, nil
	}
	payload, err := json.Marshal(clonePlaygroundRunForClient(run))
	if err != nil {
		return nil, false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	claimed, err := s.rdb.SetNX(ctx, playgroundRunRedisKey(run.UserID, run.ID), payload, s.ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if claimed {
		return nil, true, nil
	}
	existing, found, err := s.loadPersistentRun(run.UserID, run.ID)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, errors.New("playground run disappeared while being claimed")
	}
	return clonePlaygroundRunForClient(existing), false, nil
}

func (s *PlaygroundRunService) persistRun(run *PlaygroundRun) error {
	if s.rdb == nil || run == nil || run.UserID <= 0 || strings.TrimSpace(run.ID) == "" {
		return nil
	}
	payload, err := json.Marshal(clonePlaygroundRunForClient(run))
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, playgroundRunRedisKey(run.UserID, run.ID), payload, s.ttl)
	for index, image := range run.Images {
		if len(image.data) == 0 {
			continue
		}
		pipe.Set(ctx, playgroundRunRedisImageKeyFor(run.UserID, run.ID, index), image.data, s.ttl)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *PlaygroundRunService) loadPersistentRun(userID int64, id string) (*PlaygroundRun, bool, error) {
	if s.rdb == nil || userID <= 0 || strings.TrimSpace(id) == "" {
		return nil, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	payload, err := s.rdb.Get(ctx, playgroundRunRedisKey(userID, id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var run PlaygroundRun
	if err := json.Unmarshal(payload, &run); err != nil {
		return nil, false, err
	}
	run.UserID = userID
	return &run, true, nil
}

func (s *PlaygroundRunService) loadPersistentImage(userID int64, id string, index int, mimeType string) (PlaygroundRunImageAsset, bool, error) {
	if s.rdb == nil || userID <= 0 || index < 0 {
		return PlaygroundRunImageAsset{}, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	data, err := s.rdb.Get(ctx, playgroundRunRedisImageKeyFor(userID, id, index)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return PlaygroundRunImageAsset{}, false, nil
		}
		return PlaygroundRunImageAsset{}, false, err
	}
	return playgroundRunImageAsset(data, mimeType), true, nil
}

func (s *PlaygroundRunService) deletePersistentImages(userID int64, id string, imageCount int) {
	if s.rdb == nil || imageCount <= 0 {
		return
	}
	keys := make([]string, 0, imageCount)
	for index := 0; index < imageCount; index++ {
		keys = append(keys, playgroundRunRedisImageKeyFor(userID, id, index))
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("playground run image cleanup failed for user %d run %s: %v", userID, id, err)
	}
}

func (s *PlaygroundRunService) cleanupExpiredLocked(now time.Time) {
	s.mu.Lock()
	for key, run := range s.runs {
		if run == nil {
			delete(s.runs, key)
			continue
		}
		if isTerminalPlaygroundRunStatus(run.Status) && now.Sub(run.UpdatedAt) > s.ttl {
			delete(s.runs, key)
		}
	}
	s.mu.Unlock()
}

func playgroundRunKey(userID int64, id string) string {
	return fmt.Sprintf("%d:%s", userID, id)
}

func playgroundRunRedisKey(userID int64, id string) string {
	return playgroundRunRedisKeyPrefix + playgroundRunKey(userID, id)
}

func playgroundRunRedisImageKeyFor(userID int64, id string, index int) string {
	return fmt.Sprintf("%s%s%d", playgroundRunRedisKey(userID, id), playgroundRunRedisImageKey, index)
}

func clonePlaygroundRunForClient(run *PlaygroundRun) *PlaygroundRun {
	if run == nil {
		return nil
	}
	out := *run
	out.cancel = nil
	out.Images = append([]PlaygroundRunImage(nil), run.Images...)
	for index := range out.Images {
		if len(run.Images[index].data) > 0 {
			assetIndex := index
			out.Images[index].AssetIndex = &assetIndex
			out.Images[index].URL = ""
		}
		out.Images[index].data = nil
	}
	if run.Mode == "image" {
		out.Raw = nil
	} else if run.Raw != nil {
		out.Raw = append(json.RawMessage(nil), run.Raw...)
	}
	return &out
}

func clonePlaygroundRunForPersistence(run *PlaygroundRun) *PlaygroundRun {
	if run == nil {
		return nil
	}
	out := *run
	out.cancel = nil
	out.Images = append([]PlaygroundRunImage(nil), run.Images...)
	for index := range out.Images {
		out.Images[index].data = append([]byte(nil), run.Images[index].data...)
	}
	if run.Raw != nil {
		out.Raw = append(json.RawMessage(nil), run.Raw...)
	}
	return &out
}

func playgroundRunImageAsset(data []byte, mimeType string) PlaygroundRunImageAsset {
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return PlaygroundRunImageAsset{Data: append([]byte(nil), data...), ContentType: mimeType}
}

func cancelPlaygroundRun(run *PlaygroundRun, now time.Time) {
	if run == nil || isTerminalPlaygroundRunStatus(run.Status) {
		return
	}
	run.Status = PlaygroundRunCanceled
	run.Error = "request canceled"
	run.Images = nil
	run.Raw = nil
	run.UpdatedAt = now
	run.CompletedAt = &now
}

func isTerminalPlaygroundRunStatus(status PlaygroundRunStatus) bool {
	return status == PlaygroundRunSucceeded || status == PlaygroundRunFailed || status == PlaygroundRunCanceled
}

func filterPlaygroundImageInputs(images []PlaygroundRunImageInput) []PlaygroundRunImageInput {
	out := make([]PlaygroundRunImageInput, 0, len(images))
	for _, image := range images {
		if strings.TrimSpace(image.DataURL) == "" {
			continue
		}
		out = append(out, image)
	}
	return out
}

func decodePlaygroundImageDataURL(dataURL string) ([]byte, string, error) {
	value := strings.TrimSpace(dataURL)
	lower := strings.ToLower(value)
	const prefix = "data:"
	const marker = ";base64,"
	comma := strings.Index(lower, marker)
	if !strings.HasPrefix(lower, prefix+"image/") || comma < 0 {
		return nil, "", errors.New("uploaded image must be a base64 image data URL")
	}
	mediaType := strings.TrimSpace(value[len(prefix):comma])
	if semicolon := strings.Index(mediaType, ";"); semicolon >= 0 {
		mediaType = strings.TrimSpace(mediaType[:semicolon])
	}
	if mediaType == "" {
		mediaType = "image/png"
	}
	data, err := base64.StdEncoding.DecodeString(value[comma+len(marker):])
	if err != nil {
		return nil, "", fmt.Errorf("decode uploaded image: %w", err)
	}
	if len(data) == 0 {
		return nil, "", errors.New("uploaded image is empty")
	}
	return data, mediaType, nil
}

func sanitizePlaygroundUploadFileName(name, contentType string) string {
	cleaned := filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	cleaned = strings.Trim(cleaned, ". ")
	if cleaned != "" && cleaned != "." && cleaned != string(filepath.Separator) {
		return cleaned
	}
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return "image.jpg"
	case "image/webp":
		return "image.webp"
	default:
		return "image.png"
	}
}

func playgroundImageMultipartHeader(fieldName, fileName, contentType string) textproto.MIMEHeader {
	if strings.TrimSpace(contentType) == "" {
		contentType = "application/octet-stream"
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapePlaygroundMultipartValue(fieldName), escapePlaygroundMultipartValue(fileName)))
	header.Set("Content-Type", contentType)
	return header
}

func escapePlaygroundMultipartValue(value string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, "\\\"").Replace(value)
}

func filterPlaygroundChatMessages(messages []PlaygroundRunChatMessage) []PlaygroundRunChatMessage {
	out := make([]PlaygroundRunChatMessage, 0, len(messages))
	for _, message := range messages {
		if strings.TrimSpace(message.Role) == "" || !playgroundChatContentHasValue(message.Content) {
			continue
		}
		out = append(out, message)
	}
	return out
}

func playgroundChatContentHasValue(content any) bool {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value) != ""
	case []any:
		for _, item := range value {
			if playgroundChatContentHasValue(item) {
				return true
			}
		}
	case map[string]any:
		if text, ok := value["text"].(string); ok && strings.TrimSpace(text) != "" {
			return true
		}
		if content, ok := value["content"].(string); ok && strings.TrimSpace(content) != "" {
			return true
		}
		if imageURL, ok := value["image_url"].(map[string]any); ok {
			if urlValue, ok := imageURL["url"].(string); ok && strings.TrimSpace(urlValue) != "" {
				return true
			}
		}
	default:
		return content != nil
	}
	return false
}

func readPlaygroundChatStream(body io.Reader, onEvent func(json.RawMessage, string)) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var sseLines []string
	process := func(data string) {
		trimmed := strings.TrimSpace(data)
		if trimmed == "" || trimmed == "[DONE]" {
			return
		}
		raw := json.RawMessage([]byte(trimmed))
		var payload any
		if err := json.Unmarshal(raw, &payload); err != nil {
			return
		}
		delta := extractPlaygroundStreamText(payload)
		if delta == "" {
			delta = extractPlaygroundChatText(payload)
		}
		onEvent(raw, delta)
	}

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if len(sseLines) > 0 {
				process(strings.Join(sseLines, "\n"))
				sseLines = sseLines[:0]
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			sseLines = append(sseLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			continue
		}
		if strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") || strings.HasPrefix(line, "retry:") {
			continue
		}
		process(line)
	}
	if len(sseLines) > 0 {
		process(strings.Join(sseLines, "\n"))
	}
	return scanner.Err()
}

func extractPlaygroundStreamText(payload any) string {
	record, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	if choices, ok := record["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if delta, ok := choice["delta"].(map[string]any); ok {
				return extractPlaygroundTextValue(delta["content"])
			}
		}
	}
	if delta, ok := record["delta"].(string); ok {
		return delta
	}
	if text, ok := record["text"].(string); ok {
		return text
	}
	return ""
}

func extractPlaygroundChatText(payload any) string {
	record, ok := payload.(map[string]any)
	if !ok {
		return ""
	}
	if choices, ok := record["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				return extractPlaygroundTextValue(message["content"])
			}
		}
	}
	if text, ok := record["output_text"].(string); ok {
		return text
	}
	return ""
}

func extractPlaygroundChatTextFromRaw(raw json.RawMessage) string {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return extractPlaygroundChatText(payload)
}

func extractPlaygroundTextValue(value any) string {
	switch content := value.(type) {
	case string:
		return content
	case []any:
		var builder strings.Builder
		for _, item := range content {
			if text := extractPlaygroundTextValue(item); text != "" {
				builder.WriteString(text)
			}
		}
		return builder.String()
	case map[string]any:
		if text, ok := content["text"].(string); ok {
			return text
		}
		if text, ok := content["content"].(string); ok {
			return text
		}
	}
	return ""
}

func extractPlaygroundImages(payload playgroundImageUpstreamResponse, outputFormat string) []PlaygroundRunImage {
	if outputFormat == "" {
		outputFormat = "png"
	}
	requestedMimeType := playgroundImageContentType(nil, "image/"+strings.ToLower(outputFormat))
	images := make([]PlaygroundRunImage, 0, len(payload.Data))
	for index := range payload.Data {
		record := &payload.Data[index]
		imageURL := strings.TrimSpace(record.URL)
		if strings.HasPrefix(strings.ToLower(imageURL), "data:") {
			decoded, contentType, err := decodePlaygroundImageDataURL(imageURL)
			if err != nil {
				continue
			}
			width, height, _ := detectOpenAIImageBytesDimensions(decoded)
			images = append(images, PlaygroundRunImage{
				RevisedPrompt: record.RevisedPrompt,
				MimeType:      playgroundImageContentType(decoded, contentType),
				Width:         width,
				Height:        height,
				data:          decoded,
			})
			continue
		}
		if imageURL != "" {
			images = append(images, PlaygroundRunImage{
				URL:           imageURL,
				RevisedPrompt: record.RevisedPrompt,
			})
			continue
		}
		if len(record.B64JSON) == 0 {
			continue
		}
		decoded := record.B64JSON
		record.B64JSON = nil
		width, height, _ := detectOpenAIImageBytesDimensions(decoded)
		images = append(images, PlaygroundRunImage{
			RevisedPrompt: record.RevisedPrompt,
			MimeType:      playgroundImageContentType(decoded, requestedMimeType),
			Width:         width,
			Height:        height,
			data:          decoded,
		})
	}
	return images
}

func extractPlaygroundImagesFromAny(payload any, outputFormat string) []PlaygroundRunImage {
	if outputFormat == "" {
		outputFormat = "png"
	}
	fallbackMimeType := playgroundImageContentType(nil, "image/"+strings.ToLower(outputFormat))
	images := make([]PlaygroundRunImage, 0, 1)
	seen := make(map[string]struct{})
	addData := func(data []byte, mimeType, revisedPrompt string) {
		if len(data) == 0 {
			return
		}
		mimeType = playgroundImageContentType(data, mimeType)
		key := mimeType + ":" + base64.StdEncoding.EncodeToString(data)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		width, height, _ := detectOpenAIImageBytesDimensions(data)
		images = append(images, PlaygroundRunImage{
			RevisedPrompt: revisedPrompt,
			MimeType:      mimeType,
			Width:         width,
			Height:        height,
			data:          data,
		})
	}
	addURL := func(rawURL, mimeType, revisedPrompt string) {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			return
		}
		if strings.HasPrefix(strings.ToLower(rawURL), "data:") {
			data, contentType, err := decodePlaygroundImageDataURL(rawURL)
			if err == nil {
				addData(data, contentType, revisedPrompt)
			}
			return
		}
		key := "url:" + rawURL
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		images = append(images, PlaygroundRunImage{URL: rawURL, RevisedPrompt: revisedPrompt, MimeType: mimeType})
	}
	var walk func(any)
	walk = func(value any) {
		switch item := value.(type) {
		case []any:
			for _, entry := range item {
				walk(entry)
			}
		case map[string]any:
			revisedPrompt, _ := item["revised_prompt"].(string)
			if revisedPrompt == "" {
				revisedPrompt, _ = item["revisedPrompt"].(string)
			}
			if inline, ok := item["inlineData"].(map[string]any); ok {
				data, _ := inline["data"].(string)
				mimeType, _ := inline["mimeType"].(string)
				decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
				if err == nil {
					addData(decoded, mimeType, revisedPrompt)
				}
			}
			if inline, ok := item["inline_data"].(map[string]any); ok {
				data, _ := inline["data"].(string)
				mimeType, _ := inline["mime_type"].(string)
				decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
				if err == nil {
					addData(decoded, mimeType, revisedPrompt)
				}
			}
			if imageURL, ok := item["image_url"].(map[string]any); ok {
				urlValue, _ := imageURL["url"].(string)
				addURL(urlValue, "", revisedPrompt)
			} else if imageURL, ok := item["image_url"].(string); ok {
				addURL(imageURL, "", revisedPrompt)
			}
			if b64, ok := item["b64_json"].(string); ok {
				decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64))
				if err == nil {
					addData(decoded, fallbackMimeType, revisedPrompt)
				}
			}
			if itemType, _ := item["type"].(string); itemType == "image_generation_call" {
				if result, ok := item["result"].(string); ok {
					decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(result))
					if err == nil {
						addData(decoded, fallbackMimeType, revisedPrompt)
					}
				}
			}
			if content, ok := item["content"].(string); ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(content)), "data:") {
				addURL(content, "", revisedPrompt)
			}
			for _, entry := range item {
				walk(entry)
			}
		}
	}
	walk(payload)
	return images
}

func playgroundImageUsesGeminiNativeAPI(request PlaygroundRunRequest) bool {
	switch strings.ToLower(strings.TrimSpace(request.Platform)) {
	case "gemini", "antigravity":
		return true
	}
	return strings.Contains(strings.ToLower(normalizePlaygroundEndpointBase(request.EndpointBase)), "v1beta")
}

func playgroundImageEndpointBase(request PlaygroundRunRequest) string {
	if strings.TrimSpace(request.EndpointBase) != "" {
		return request.EndpointBase
	}
	switch strings.ToLower(strings.TrimSpace(request.Platform)) {
	case "gemini":
		return "/v1beta"
	case "antigravity":
		return "/antigravity/v1beta"
	default:
		return "/v1"
	}
}

func playgroundImageAspectRatio(size string) string {
	switch normalizePlaygroundImageSize(size) {
	case "1024x1024":
		return "1:1"
	case "1536x1024":
		return "3:2"
	case "1024x1536":
		return "2:3"
	default:
		return ""
	}
}

func playgroundImageContentType(data []byte, fallback string) string {
	if len(data) > 0 {
		if detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0])); isPlaygroundImageContentType(detected) {
			return detected
		}
	}
	fallback = strings.ToLower(strings.TrimSpace(strings.Split(fallback, ";")[0]))
	if fallback == "image/jpg" {
		fallback = "image/jpeg"
	}
	if isPlaygroundImageContentType(fallback) {
		return fallback
	}
	return "application/octet-stream"
}

func isPlaygroundImageContentType(value string) bool {
	switch value {
	case "image/png", "image/jpeg", "image/webp", "image/gif", "image/avif":
		return true
	default:
		return false
	}
}

func parsePlaygroundUpstreamError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	message := strings.TrimSpace(resp.Status)
	var payload map[string]any
	if len(body) > 0 && json.Unmarshal(body, &payload) == nil {
		if errorRecord, ok := payload["error"].(map[string]any); ok {
			if value, ok := errorRecord["message"].(string); ok && strings.TrimSpace(value) != "" {
				message = strings.TrimSpace(value)
			}
		}
		if value, ok := payload["message"].(string); ok && strings.TrimSpace(value) != "" {
			message = strings.TrimSpace(value)
		}
		if value, ok := payload["detail"].(string); ok && strings.TrimSpace(value) != "" {
			message = strings.TrimSpace(value)
		}
	}
	return fmt.Errorf("%s (HTTP %d)", message, resp.StatusCode)
}

func normalizePlaygroundImageSize(size string) string {
	trimmed := strings.ToLower(strings.TrimSpace(size))
	if trimmed == "" || trimmed == "auto" {
		return "auto"
	}
	supported := map[string]bool{
		"1024x1024": true,
		"1536x1024": true,
		"1024x1536": true,
	}
	if supported[trimmed] {
		return trimmed
	}
	matches := regexp.MustCompile(`^(\d+)x(\d+)$`).FindStringSubmatch(trimmed)
	if len(matches) != 3 {
		return "auto"
	}
	var width, height int
	_, _ = fmt.Sscanf(trimmed, "%dx%d", &width, &height)
	if width <= 0 || height <= 0 {
		return "auto"
	}
	if width == height {
		return "1024x1024"
	}
	if width > height {
		return "1536x1024"
	}
	return "1024x1536"
}

func playgroundImageRequestSize(model, size string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "gpt-image-2" || strings.HasPrefix(model, "gpt-image-2-") {
		return size
	}
	return normalizePlaygroundImageSize(size)
}

func buildPlaygroundRunEndpointURL(baseURL, endpointBase, endpoint string) (string, error) {
	root, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || root.Scheme == "" || root.Host == "" {
		return "", errors.New("invalid playground request base url")
	}
	base := normalizePlaygroundEndpointBase(endpointBase)
	endpointPath := "/" + strings.TrimLeft(strings.TrimSpace(endpoint), "/")
	relativePath := strings.TrimPrefix(endpointPath, "/v1beta")
	if relativePath == endpointPath {
		relativePath = strings.TrimPrefix(endpointPath, "/v1")
	}
	if strings.HasSuffix(base, endpointPath) || (relativePath != "" && strings.HasSuffix(base, relativePath)) {
		root.Path = base
		return root.String(), nil
	}
	if hasPlaygroundVersionSuffix(base) {
		root.Path = strings.TrimRight(base, "/") + relativePath
		return root.String(), nil
	}
	root.Path = strings.TrimRight(base, "/") + endpointPath
	return root.String(), nil
}

func normalizePlaygroundEndpointBase(raw string) string {
	value := strings.TrimRight(strings.TrimSpace(raw), "/")
	if value == "" {
		return "/v1"
	}
	if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
		value = parsed.Path
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	lower := strings.ToLower(value)
	switch {
	case strings.HasSuffix(lower, "/antigravity/v1beta"):
		return "/antigravity/v1beta"
	case strings.HasSuffix(lower, "/antigravity/v1"):
		return "/antigravity/v1"
	case strings.HasSuffix(lower, "/v1beta"):
		return "/v1beta"
	case strings.HasSuffix(lower, "/v1"):
		return "/v1"
	default:
		return "/v1"
	}
}

func hasPlaygroundVersionSuffix(pathValue string) bool {
	parts := strings.Split(strings.Trim(pathValue, "/"), "/")
	if len(parts) == 0 {
		return false
	}
	last := parts[len(parts)-1]
	return regexp.MustCompile(`(?i)^v\d+(?:(?:\.\d+)|(?:alpha.*|beta.*|preview.*))?$`).MatchString(last)
}
