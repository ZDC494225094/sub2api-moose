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
	URL           string `json:"url"`
	RevisedPrompt string `json:"revisedPrompt,omitempty"`
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
	ttl        time.Duration
}

func NewPlaygroundRunService() *PlaygroundRunService {
	return &PlaygroundRunService{
		runs: make(map[string]*PlaygroundRun),
		httpClient: &http.Client{
			Timeout: 0,
		},
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
		out := clonePlaygroundRun(existing)
		s.mu.Unlock()
		return out, nil
	}

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
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
	run.cancel = cancel
	s.runs[key] = run
	out := clonePlaygroundRun(run)
	s.mu.Unlock()

	go s.execute(ctx, key, request, baseURL)

	return out, nil
}

func (s *PlaygroundRunService) Get(userID int64, id string) (*PlaygroundRun, bool) {
	if s == nil || userID <= 0 {
		return nil, false
	}
	key := playgroundRunKey(userID, strings.TrimSpace(id))
	s.mu.RLock()
	run := s.runs[key]
	if run == nil {
		s.mu.RUnlock()
		return nil, false
	}
	out := clonePlaygroundRun(run)
	s.mu.RUnlock()
	return out, true
}

func (s *PlaygroundRunService) Cancel(userID int64, id string) (*PlaygroundRun, bool) {
	if s == nil || userID <= 0 {
		return nil, false
	}
	key := playgroundRunKey(userID, strings.TrimSpace(id))
	s.mu.Lock()
	run := s.runs[key]
	if run == nil {
		s.mu.Unlock()
		return nil, false
	}
	if run.cancel != nil && !isTerminalPlaygroundRunStatus(run.Status) {
		run.cancel()
	}
	now := time.Now()
	if !isTerminalPlaygroundRunStatus(run.Status) {
		run.Status = PlaygroundRunCanceled
		run.Error = "request canceled"
		run.UpdatedAt = now
		run.CompletedAt = &now
	}
	out := clonePlaygroundRun(run)
	s.mu.Unlock()
	return out, true
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
		run.Raw = raw
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
	n := request.N
	if n <= 0 {
		n = 1
	}
	if n > 4 {
		n = 4
	}
	outputFormat := strings.TrimSpace(request.OutputFormat)
	if outputFormat == "" {
		outputFormat = "png"
	}
	imageInputs := filterPlaygroundImageInputs(request.Images)
	if len(imageInputs) > 0 {
		return s.executeImageEdit(ctx, key, request, baseURL, started, imageInputs, outputFormat, n)
	}
	payload := map[string]any{
		"model":           request.Model,
		"prompt":          request.Prompt,
		"size":            normalizePlaygroundImageSize(request.Size),
		"n":               n,
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
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	images := extractPlaygroundImages(raw, outputFormat)
	s.update(key, func(run *PlaygroundRun) {
		run.Images = images
		run.Content = ""
		run.DurationMs = time.Since(started).Milliseconds()
		run.UpdatedAt = time.Now()
	})
	return json.RawMessage(raw), nil
}

func (s *PlaygroundRunService) executeImageEdit(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string, started time.Time, images []PlaygroundRunImageInput, outputFormat string, n int) (json.RawMessage, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if err := writer.WriteField("model", request.Model); err != nil {
		return nil, err
	}
	if err := writer.WriteField("prompt", request.Prompt); err != nil {
		return nil, err
	}
	if err := writer.WriteField("size", normalizePlaygroundImageSize(request.Size)); err != nil {
		return nil, err
	}
	if err := writer.WriteField("n", fmt.Sprintf("%d", n)); err != nil {
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

	endpointURL, err := buildPlaygroundRunEndpointURL(baseURL, request.EndpointBase, "/v1/images/edits")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+request.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parsePlaygroundUpstreamError(resp)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	resultImages := extractPlaygroundImages(raw, outputFormat)
	s.update(key, func(run *PlaygroundRun) {
		run.Images = resultImages
		run.Content = ""
		run.DurationMs = time.Since(started).Milliseconds()
		run.UpdatedAt = time.Now()
	})
	return json.RawMessage(raw), nil
}

func (s *PlaygroundRunService) update(key string, fn func(*PlaygroundRun)) {
	s.mu.Lock()
	if run := s.runs[key]; run != nil {
		fn(run)
	}
	s.mu.Unlock()
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

func clonePlaygroundRun(run *PlaygroundRun) *PlaygroundRun {
	if run == nil {
		return nil
	}
	out := *run
	out.cancel = nil
	out.Images = append([]PlaygroundRunImage(nil), run.Images...)
	if run.Raw != nil {
		out.Raw = append(json.RawMessage(nil), run.Raw...)
	}
	return &out
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

func extractPlaygroundImages(raw json.RawMessage, outputFormat string) []PlaygroundRunImage {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	data, ok := payload["data"].([]any)
	if !ok {
		return nil
	}
	if outputFormat == "" {
		outputFormat = "png"
	}
	images := make([]PlaygroundRunImage, 0, len(data))
	for _, item := range data {
		record, ok := item.(map[string]any)
		if !ok {
			continue
		}
		imageURL, _ := record["url"].(string)
		if imageURL == "" {
			if b64, ok := record["b64_json"].(string); ok && b64 != "" {
				imageURL = "data:image/" + outputFormat + ";base64," + b64
			}
		}
		if imageURL == "" {
			continue
		}
		revisedPrompt, _ := record["revised_prompt"].(string)
		images = append(images, PlaygroundRunImage{
			URL:           imageURL,
			RevisedPrompt: revisedPrompt,
		})
	}
	return images
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

func buildPlaygroundRunEndpointURL(baseURL, endpointBase, endpoint string) (string, error) {
	root, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || root.Scheme == "" || root.Host == "" {
		return "", errors.New("invalid playground request base url")
	}
	base := normalizePlaygroundEndpointBase(endpointBase)
	endpointPath := "/" + strings.TrimLeft(strings.TrimSpace(endpoint), "/")
	relativePath := strings.TrimPrefix(endpointPath, "/v1")
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
