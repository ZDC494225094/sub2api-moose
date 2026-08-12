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
	playgroundRunRedisVideoKey  = ":video:"
	playgroundRunRedisAudioKey  = ":audio:"
	playgroundRunRedisTimeout   = 3 * time.Second
	playgroundImageMaxBytes     = 128 << 20
	playgroundVideoMaxBytes     = 512 << 20
	playgroundAudioMaxBytes     = 128 << 20
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
	ReferenceVideo   *PlaygroundRunVideoInput   `json:"referenceVideo"`
	Duration         int                        `json:"duration"`
	FPS              int                        `json:"fps"`
	AspectRatio      string                     `json:"aspectRatio"`
	Resolution       string                     `json:"resolution"`
	Voice            string                     `json:"voice"`
	Speed            float64                    `json:"speed"`
	Instructions     string                     `json:"instructions"`
}

type PlaygroundRunChatMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type PlaygroundRunImageInput struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	DataURL string `json:"dataUrl"`
	Frame   string `json:"frame,omitempty"`
}

type PlaygroundRunVideoInput struct {
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	DataURL         string  `json:"dataUrl"`
	DurationSeconds float64 `json:"durationSeconds"`
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

type PlaygroundRunVideo struct {
	URL          string `json:"url,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	Duration     *int   `json:"duration,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	MimeType     string `json:"mimeType,omitempty"`
	AssetIndex   *int   `json:"assetIndex,omitempty"`

	data []byte
}

type PlaygroundRunImageAsset struct {
	Data        []byte
	ContentType string
}

type PlaygroundRunVideoAsset = PlaygroundRunImageAsset

type PlaygroundRunAudio struct {
	MimeType   string `json:"mimeType,omitempty"`
	AssetIndex *int   `json:"assetIndex,omitempty"`

	data []byte
}

type PlaygroundRunAudioAsset = PlaygroundRunImageAsset

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
	Videos      []PlaygroundRunVideo `json:"videos,omitempty"`
	Audios      []PlaygroundRunAudio `json:"audios,omitempty"`
	Error       string               `json:"error,omitempty"`
	Raw         json.RawMessage      `json:"raw,omitempty"`
	CreatedAt   time.Time            `json:"createdAt"`
	UpdatedAt   time.Time            `json:"updatedAt"`
	CompletedAt *time.Time           `json:"completedAt,omitempty"`
	DurationMs  int64                `json:"durationMs,omitempty"`

	cancel context.CancelFunc
}

type PlaygroundRunService struct {
	mu                sync.RWMutex
	runs              map[string]*PlaygroundRun
	httpClient        *http.Client
	rdb               *redis.Client
	videoAssetRepo    PlaygroundVideoAssetRepository
	ttl               time.Duration
	videoPollInterval time.Duration
}

func NewPlaygroundRunService() *PlaygroundRunService {
	return newPlaygroundRunService(nil, nil)
}

// ProvidePlaygroundRunService keeps completed Playground tasks available when a
// browser refreshes or a subsequent request lands on a different API instance.
func ProvidePlaygroundRunService(rdb *redis.Client, videoAssetRepo PlaygroundVideoAssetRepository) *PlaygroundRunService {
	return newPlaygroundRunService(rdb, videoAssetRepo)
}

func newPlaygroundRunService(rdb *redis.Client, videoAssetRepo PlaygroundVideoAssetRepository) *PlaygroundRunService {
	return &PlaygroundRunService{
		runs: make(map[string]*PlaygroundRun),
		httpClient: &http.Client{
			Timeout: 0,
		},
		rdb:               rdb,
		videoAssetRepo:    videoAssetRepo,
		ttl:               6 * time.Hour,
		videoPollInterval: 2 * time.Second,
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
	if request.Mode != "chat" && request.Mode != "image" && request.Mode != "video" && request.Mode != "audio" {
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
	if request.Mode == "video" {
		if err := normalizePlaygroundVideoRequest(&request); err != nil {
			return nil, err
		}
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
	// Canvas jobs can legitimately outlive a browser session or a fixed proxy
	// timeout. They are canceled only by the user through Cancel.
	ctx, cancel := context.WithCancel(context.Background())
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
	persistedRun, persistedFound, persistedErr := s.loadPersistentRun(userID, id)
	if persistedErr != nil {
		log.Printf("playground run persistence read failed for user %d run %s: %v", userID, id, persistedErr)
	}
	key := playgroundRunKey(userID, id)
	s.mu.RLock()
	inMemoryRun := s.runs[key]
	if inMemoryRun != nil {
		inMemoryRun = clonePlaygroundRunForClient(inMemoryRun)
	}
	s.mu.RUnlock()

	if persistedErr == nil && persistedFound && inMemoryRun != nil {
		if playgroundRunIsNewer(inMemoryRun, persistedRun) {
			return inMemoryRun, true
		}
		return clonePlaygroundRunForClient(persistedRun), true
	}
	if persistedErr == nil && persistedFound {
		return clonePlaygroundRunForClient(persistedRun), true
	}
	if inMemoryRun != nil {
		return inMemoryRun, true
	}
	return nil, false
}

func playgroundRunIsNewer(candidate, current *PlaygroundRun) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	if candidate.UpdatedAt.After(current.UpdatedAt) {
		return true
	}
	if candidate.UpdatedAt.Before(current.UpdatedAt) {
		return false
	}
	return isTerminalPlaygroundRunStatus(candidate.Status) && !isTerminalPlaygroundRunStatus(current.Status)
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

func (s *PlaygroundRunService) GetVideo(userID int64, id string, index int) (PlaygroundRunVideoAsset, bool, error) {
	return s.GetVideoContext(context.Background(), userID, id, index)
}

func (s *PlaygroundRunService) GetAudio(userID int64, id string, index int) (PlaygroundRunAudioAsset, bool, error) {
	if s == nil || userID <= 0 || index < 0 {
		return PlaygroundRunAudioAsset{}, false, nil
	}
	id = strings.TrimSpace(id)
	key := playgroundRunKey(userID, id)
	s.mu.RLock()
	run := s.runs[key]
	var audio PlaygroundRunAudio
	found := false
	if run != nil && run.Status == PlaygroundRunSucceeded && index < len(run.Audios) {
		audio = run.Audios[index]
		audio.data = append([]byte(nil), audio.data...)
		found = true
	}
	s.mu.RUnlock()
	if !found {
		persistedRun, persisted, err := s.loadPersistentRun(userID, id)
		if err != nil {
			return PlaygroundRunAudioAsset{}, false, err
		}
		if !persisted || persistedRun.Status != PlaygroundRunSucceeded || index >= len(persistedRun.Audios) {
			return PlaygroundRunAudioAsset{}, false, nil
		}
		audio = persistedRun.Audios[index]
	}
	if len(audio.data) > 0 {
		return playgroundRunAudioAsset(audio.data, audio.MimeType), true, nil
	}
	return s.loadPersistentAudio(userID, id, index, audio.MimeType)
}

func (s *PlaygroundRunService) GetVideoContext(ctx context.Context, userID int64, id string, index int) (PlaygroundRunVideoAsset, bool, error) {
	if s == nil || userID <= 0 || index < 0 {
		return PlaygroundRunVideoAsset{}, false, nil
	}
	id = strings.TrimSpace(id)
	key := playgroundRunKey(userID, id)
	s.mu.RLock()
	run := s.runs[key]
	var video PlaygroundRunVideo
	found := false
	if run != nil && run.Status == PlaygroundRunSucceeded && index < len(run.Videos) {
		video = run.Videos[index]
		video.data = append([]byte(nil), video.data...)
		found = true
	}
	s.mu.RUnlock()
	if !found {
		persistedRun, persisted, err := s.loadPersistentRun(userID, id)
		if err != nil {
			log.Printf("playground video Redis metadata read failed for user %d run %s: %v", userID, id, err)
		}
		if err == nil && persisted && persistedRun.Status == PlaygroundRunSucceeded && index < len(persistedRun.Videos) {
			video = persistedRun.Videos[index]
			found = true
		}
	}
	if found && len(video.data) > 0 {
		asset, err := playgroundRunVideoAsset(video.data, video.MimeType)
		return asset, true, err
	}
	if found {
		if asset, persisted, err := s.loadPersistentVideo(userID, id, index, video.MimeType); err != nil || persisted {
			return asset, persisted, err
		}
	}
	if s.videoAssetRepo == nil {
		if found {
			return PlaygroundRunVideoAsset{}, true, errors.New("playground video is not available as a local asset")
		}
		return PlaygroundRunVideoAsset{}, false, nil
	}
	metadata, err := s.videoAssetRepo.Get(ctx, userID, id, index)
	if err != nil {
		return PlaygroundRunVideoAsset{}, found, err
	}
	if metadata == nil {
		if found {
			return PlaygroundRunVideoAsset{}, true, errors.New("playground video is not available as a local asset")
		}
		return PlaygroundRunVideoAsset{}, false, nil
	}
	assetURL, err := playgroundRemoteAssetURL(metadata.SourceURL)
	if err != nil {
		return PlaygroundRunVideoAsset{}, true, err
	}
	data, mimeType, err := s.downloadPlaygroundVideo(ctx, assetURL, "")
	if err != nil {
		return PlaygroundRunVideoAsset{}, true, err
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = metadata.MimeType
	}
	asset, err := playgroundRunVideoAsset(data, mimeType)
	if err != nil {
		return PlaygroundRunVideoAsset{}, true, err
	}
	return asset, true, nil
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
		videoCount := len(run.Videos)
		audioCount := len(run.Audios)
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
			s.deletePersistentVideos(userID, id, videoCount)
			s.deletePersistentAudios(userID, id, audioCount)
		}
		return out, true
	}
	s.mu.Unlock()

	persistedRun, found, err := s.loadPersistentRun(userID, id)
	if err != nil || !found {
		return nil, false
	}
	imageCount := len(persistedRun.Images)
	videoCount := len(persistedRun.Videos)
	audioCount := len(persistedRun.Audios)
	canceled := !isTerminalPlaygroundRunStatus(persistedRun.Status)
	cancelPlaygroundRun(persistedRun, time.Now())
	if err := s.persistRun(persistedRun); err != nil {
		log.Printf("playground run cancellation persistence failed for user %d run %s: %v", userID, id, err)
	}
	if canceled {
		s.deletePersistentImages(userID, id, imageCount)
		s.deletePersistentVideos(userID, id, videoCount)
		s.deletePersistentAudios(userID, id, audioCount)
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
	case "video":
		raw, err = s.executeVideo(ctx, key, request, baseURL, started)
	case "audio":
		raw, err = s.executeAudio(ctx, key, request, baseURL)
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
		if request.Mode == "image" || request.Mode == "video" || request.Mode == "audio" {
			run.Raw = nil
		} else {
			run.Raw = raw
		}
		run.UpdatedAt = completed
		run.CompletedAt = &completed
		run.DurationMs = completed.Sub(started).Milliseconds()
	})
}

func (s *PlaygroundRunService) executeAudio(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string) (json.RawMessage, error) {
	prompt := strings.TrimSpace(request.Prompt)
	if prompt == "" {
		return nil, errors.New("audio input is required")
	}
	endpoint := "/v1/audio/speech"
	payload := map[string]any{
		"model": request.Model,
		"input": prompt,
	}
	if request.Platform == PlatformGrok {
		// xAI Voice uses a distinct TTS protocol. The canvas keeps its OpenAI
		// voice picker for third-party providers, so use xAI's stable default.
		endpoint = "/v1/tts"
		payload = map[string]any{
			"text":     prompt,
			"language": playgroundGrokTTSLanguage(prompt),
			"voice_id": "Ara",
		}
	} else {
		if voice := strings.TrimSpace(request.Voice); voice != "" {
			payload["voice"] = voice
		}
		if format := strings.TrimSpace(request.OutputFormat); format != "" {
			payload["response_format"] = format
		}
		if request.Speed > 0 {
			payload["speed"] = request.Speed
		}
		if instructions := strings.TrimSpace(request.Instructions); instructions != "" {
			payload["instructions"] = instructions
		}
	}

	endpointURL, err := buildPlaygroundRunEndpointURL(baseURL, request.EndpointBase, endpoint)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Authorization", "Bearer "+request.APIKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := s.httpClient.Do(httpRequest)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, parsePlaygroundUpstreamError(response)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, playgroundAudioMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, errors.New("audio provider returned an empty response")
	}
	if len(data) > playgroundAudioMaxBytes {
		return nil, errors.New("audio provider response exceeds the size limit")
	}
	mimeType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(strings.ToLower(mimeType), "audio/") {
		mimeType = playgroundAudioMimeType(request.OutputFormat)
	}
	s.update(key, func(run *PlaygroundRun) {
		run.Audios = []PlaygroundRunAudio{{MimeType: mimeType, data: data}}
		run.UpdatedAt = time.Now()
	})
	return nil, nil
}

func playgroundGrokTTSLanguage(text string) string {
	for _, r := range text {
		if r >= 0x3400 && r <= 0x9fff {
			return "zh"
		}
	}
	return "en"
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
	return s.executeParallelImageRequests(ctx, key, request, baseURL, started, imageCount, func(requestCtx context.Context) ([]PlaygroundRunImage, error) {
		return s.executeImageRequest(requestCtx, endpointURL, request.APIKey, "application/json", bytes.NewReader(responseBody), outputFormat)
	})
}

func (s *PlaygroundRunService) executeVideo(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string, started time.Time) (json.RawMessage, error) {
	geminiVideo := isGeminiPlaygroundVideoRequest(request)
	payload, err := playgroundVideoGenerationPayload(request, geminiVideo)
	if err != nil {
		return nil, err
	}
	endpointBase := request.EndpointBase
	endpoint := "/v1/videos/generations"
	if geminiVideo {
		endpointBase = "/v1beta"
		if strings.ContainsAny(request.Model, "/\\") {
			return nil, errors.New("invalid Gemini video model")
		}
		endpoint = "/v1beta/models/" + url.PathEscape(request.Model) + ":predictLongRunning"
	}

	submitURL, err := buildPlaygroundRunEndpointURL(baseURL, endpointBase, endpoint)
	if err != nil {
		return nil, err
	}
	result, err := s.executePlaygroundVideoJSON(ctx, http.MethodPost, submitURL, request.APIKey, payload)
	if err != nil {
		return nil, err
	}
	if videos := extractPlaygroundVideosFromAny(result); len(videos) > 0 {
		return nil, s.commitPlaygroundVideos(ctx, key, request, baseURL, started, videos)
	}

	taskID := playgroundVideoTaskID(result, geminiVideo)
	if taskID == "" {
		return nil, errors.New("video provider returned neither a video nor a task id")
	}
	encodedOperation := ""
	statusEndpointBase := request.EndpointBase
	statusEndpoint := "/v1/videos/" + url.PathEscape(taskID)
	if geminiVideo {
		encodedOperation = base64.RawURLEncoding.EncodeToString([]byte(taskID))
		statusEndpointBase = "/v1beta"
		statusEndpoint = "/v1beta/video-operations/" + encodedOperation
	}
	statusURL, err := buildPlaygroundRunEndpointURL(baseURL, statusEndpointBase, statusEndpoint)
	if err != nil {
		return nil, err
	}

	pollInterval := s.videoPollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	for {
		statusPayload, err := s.executePlaygroundVideoJSON(ctx, http.MethodGet, statusURL, request.APIKey, nil)
		if err != nil {
			return nil, err
		}
		state, taskErr := playgroundVideoTaskState(statusPayload, geminiVideo)
		switch state {
		case playgroundVideoTaskSucceeded:
			videos := extractPlaygroundVideosFromAny(statusPayload)
			if geminiVideo {
				videos = []PlaygroundRunVideo{{
					URL:      "/v1beta/video-operations/" + encodedOperation + "/content",
					MimeType: "video/mp4",
				}}
			} else if len(videos) == 0 {
				videos = []PlaygroundRunVideo{{
					URL:      "/v1/videos/" + url.PathEscape(taskID) + "/content",
					MimeType: "video/mp4",
				}}
			}
			if len(videos) == 0 {
				return nil, errors.New("video task completed without a video output")
			}
			return nil, s.commitPlaygroundVideos(ctx, key, request, baseURL, started, videos)
		case playgroundVideoTaskFailed:
			if taskErr == "" {
				taskErr = "video generation failed"
			}
			return nil, errors.New(taskErr)
		}

		s.update(key, func(run *PlaygroundRun) {
			if !isTerminalPlaygroundRunStatus(run.Status) {
				run.UpdatedAt = time.Now()
			}
		})
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

type playgroundVideoTaskStatus int

const (
	playgroundVideoTaskPending playgroundVideoTaskStatus = iota
	playgroundVideoTaskSucceeded
	playgroundVideoTaskFailed
)

var (
	geminiOmniPromptRatioPattern      = regexp.MustCompile(`(?i)(16[[:space:]]*[:：./比][[:space:]]*9|9[[:space:]]*[:：./比][[:space:]]*16)`)
	geminiOmniPromptDurationPattern   = regexp.MustCompile(`(?i)([0-9]+([.][0-9]+)?[[:space:]]*(s|sec|secs|second|seconds)\b|[0-9]+([.][0-9]+)?[[:space:]]*(秒|秒钟))`)
	geminiOmniPromptStoryboardPattern = regexp.MustCompile(`(?i)(故事板|故事版|分镜|story[[:space:]]*board|storyboard|(shot|scene)[[:space:]]*[0-9]+)`)
	seedance20ModelPattern            = regexp.MustCompile(`(?i)(^|/)((doubao-)?seedance|sd)[-_:]?2(?:[._-]?0)(?:$|[-_:])`)
	seedance25ModelPattern            = regexp.MustCompile(`(?i)(^|/)((doubao-)?seedance|sd)[-_:]?2(?:[._-]?5)(?:$|[-_:])`)
)

func isPlaygroundVideoModel(model, target string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if slash := strings.LastIndex(normalized, "/"); slash >= 0 {
		normalized = normalized[slash+1:]
	}
	if normalized == target {
		return true
	}
	for _, separator := range []string{"-", "_", ":"} {
		if strings.HasPrefix(normalized, target+separator) {
			return true
		}
	}
	return false
}

func playgroundSeedanceModelVersion(model string) string {
	normalized := strings.ToLower(strings.TrimSpace(model))
	if seedance25ModelPattern.MatchString(normalized) {
		return "2.5"
	}
	if seedance20ModelPattern.MatchString(normalized) {
		return "2.0"
	}
	if isPlaygroundVideoModel(normalized, "seedance") || isPlaygroundVideoModel(normalized, "doubao-seedance") {
		return "legacy"
	}
	return ""
}

func normalizePlaygroundVideoRequest(request *PlaygroundRunRequest) error {
	if request == nil {
		return errors.New("video request is required")
	}
	imageCount := len(filterPlaygroundImageInputs(request.Images))
	hasReferenceVideo := request.ReferenceVideo != nil && strings.TrimSpace(request.ReferenceVideo.DataURL) != ""

	switch {
	case isPlaygroundVideoModel(request.Model, "grok-imagine-video"):
		// Grok Imagine only accepts landscape or portrait output. Older canvas
		// projects stored image dimensions (or 1:1) in this field, so normalize
		// them here as well as exposing an explicit picker in the canvas UI.
		if !playgroundStringAllowed(request.AspectRatio, "16:9", "9:16") {
			request.AspectRatio = "16:9"
		}
	case isPlaygroundVideoModel(request.Model, "grok-video-10"):
		if hasReferenceVideo {
			return errors.New("grok-video-10 does not support reference videos")
		}
		if request.Duration == 0 {
			request.Duration = 10
		}
		if imageCount > 1 && request.Duration > 10 {
			request.Duration = 10
		}
		if request.Duration != 6 && request.Duration != 10 && !(imageCount <= 1 && request.Duration == 16) {
			return errors.New("grok-video-10 duration must be 6, 10, or 16 seconds; multi-reference mode supports at most 10 seconds")
		}
		if request.Resolution != "" && request.Resolution != "480p" && request.Resolution != "720p" {
			return errors.New("grok-video-10 resolution must be 480p or 720p")
		}
		if request.AspectRatio != "" && !playgroundStringAllowed(request.AspectRatio, "16:9", "9:16", "4:3", "3:4", "2:3", "3:2", "1:1") {
			return errors.New("grok-video-10 aspect ratio is not supported")
		}
	case isPlaygroundVideoModel(request.Model, "grok-video-r"):
		if hasReferenceVideo {
			return errors.New("grok-video-r does not support reference videos")
		}
		if request.Duration == 0 {
			request.Duration = 10
		}
		if request.Duration < 6 || request.Duration > 30 {
			return errors.New("grok-video-r duration must be between 6 and 30 seconds")
		}
		if imageCount > 7 {
			return errors.New("grok-video-r supports at most 7 reference images")
		}
	case isPlaygroundVideoModel(request.Model, "kling"), isPlaygroundVideoModel(request.Model, "keling"):
		if hasReferenceVideo {
			return errors.New("kling does not support reference videos")
		}
		if request.Duration == 0 {
			request.Duration = 5
		}
		if request.Duration != 5 && request.Duration != 10 && request.Duration != 15 {
			return errors.New("kling duration must be 5, 10, or 15 seconds")
		}
		if imageCount > 2 {
			return errors.New("kling supports at most two frame images")
		}
	case playgroundSeedanceModelVersion(request.Model) != "":
		if hasReferenceVideo {
			return errors.New("seedance does not support reference videos")
		}
		if request.Duration == 0 {
			request.Duration = 8
		}
		minDuration, maxDuration := 2, 12
		switch playgroundSeedanceModelVersion(request.Model) {
		case "2.0":
			minDuration, maxDuration = 4, 15
		case "2.5":
			minDuration, maxDuration = 4, 30
		}
		if request.Duration < minDuration || request.Duration > maxDuration {
			return fmt.Errorf("seedance duration must be between %d and %d seconds", minDuration, maxDuration)
		}
		if imageCount > 2 {
			return errors.New("seedance supports at most two frame images")
		}
	case isPlaygroundVideoModel(request.Model, "gemini-omni-flash"):
		if request.Duration == 0 {
			request.Duration = 8
		}
		if request.Duration < 1 || request.Duration > 10 {
			return errors.New("gemini-omni-flash duration cannot exceed 10 seconds")
		}
		if imageCount > 5 {
			return errors.New("gemini-omni-flash supports at most 5 reference images")
		}
		if hasReferenceVideo && request.ReferenceVideo.DurationSeconds > 10.05 {
			return errors.New("gemini-omni-flash reference video cannot exceed 10 seconds")
		}
		if geminiOmniPromptRatioPattern.MatchString(request.Prompt) {
			return errors.New("gemini-omni-flash prompt cannot contain 16:9 or 9:16 aspect ratios")
		}
		if geminiOmniPromptDurationPattern.MatchString(request.Prompt) {
			return errors.New("gemini-omni-flash prompt cannot contain durations in seconds")
		}
		if geminiOmniPromptStoryboardPattern.MatchString(request.Prompt) {
			return errors.New("gemini-omni-flash does not support storyboard prompts")
		}
	default:
		if hasReferenceVideo {
			return errors.New("the selected video model does not support a reference video")
		}
	}
	return nil
}

func playgroundStringAllowed(value string, allowed ...string) bool {
	value = strings.TrimSpace(value)
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func isGeminiPlaygroundVideoRequest(request PlaygroundRunRequest) bool {
	platform := strings.ToLower(strings.TrimSpace(request.Platform))
	model := strings.ToLower(strings.TrimSpace(request.Model))
	return platform == PlatformGemini || strings.HasPrefix(model, "veo-") || strings.Contains(model, "/veo-") || isPlaygroundVideoModel(model, "gemini-omni-flash")
}

func playgroundVideoGenerationPayload(request PlaygroundRunRequest, geminiVideo bool) (map[string]any, error) {
	images := filterPlaygroundImageInputs(request.Images)
	if geminiVideo {
		instance := map[string]any{"prompt": request.Prompt}
		if isPlaygroundVideoModel(request.Model, "gemini-omni-flash") {
			referenceImages := make([]map[string]any, 0, len(images))
			for _, image := range images {
				encoded, err := playgroundGeminiImageMedia(image)
				if err != nil {
					return nil, err
				}
				referenceImages = append(referenceImages, map[string]any{
					"image":         encoded,
					"referenceType": "asset",
				})
			}
			if len(referenceImages) > 0 {
				instance["referenceImages"] = referenceImages
			}
		} else if len(images) > 0 {
			encoded, err := playgroundGeminiImageMedia(images[0])
			if err != nil {
				return nil, err
			}
			instance["image"] = encoded
		}
		if request.ReferenceVideo != nil && strings.TrimSpace(request.ReferenceVideo.DataURL) != "" {
			data, mimeType, err := decodePlaygroundVideoDataURL(request.ReferenceVideo.DataURL)
			if err != nil {
				return nil, err
			}
			if requestType := strings.TrimSpace(request.ReferenceVideo.Type); strings.HasPrefix(strings.ToLower(requestType), "video/") {
				mimeType = requestType
			}
			instance["video"] = map[string]any{
				"bytesBase64Encoded": base64.StdEncoding.EncodeToString(data),
				"mimeType":           mimeType,
			}
		}
		parameters := map[string]any{}
		if request.Duration > 0 {
			parameters["durationSeconds"] = request.Duration
		}
		if aspectRatio := strings.TrimSpace(request.AspectRatio); aspectRatio != "" {
			parameters["aspectRatio"] = aspectRatio
		}
		if resolution := strings.TrimSpace(request.Resolution); resolution != "" {
			parameters["resolution"] = resolution
		}
		if request.N > 0 {
			parameters["sampleCount"] = request.N
		}
		return map[string]any{
			"instances":  []map[string]any{instance},
			"parameters": parameters,
		}, nil
	}
	// OpenAI-compatible video providers do not share the image API's `n` field;
	// the Playground always requests one video, so omit it from the wire payload.
	payload := map[string]any{"model": request.Model, "prompt": request.Prompt}
	firstFrame := ""
	lastFrame := ""
	referenceImages := make([]string, 0, len(images))
	for _, image := range images {
		imageURL := strings.TrimSpace(image.DataURL)
		if imageURL == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(image.Frame)) {
		case "first":
			if firstFrame == "" {
				firstFrame = imageURL
			}
		case "last", "tail":
			if lastFrame == "" {
				lastFrame = imageURL
			}
		default:
			referenceImages = append(referenceImages, imageURL)
		}
	}
	if firstFrame != "" || lastFrame != "" {
		// Keep the shared contract string-based. Kling consumes image/image_tail
		// directly, while the Ark adapter converts them to first/last_frame parts.
		if firstFrame != "" {
			payload["image"] = firstFrame
		}
		if lastFrame != "" {
			payload["image_tail"] = lastFrame
		}
		if len(referenceImages) > 0 {
			payload["reference_images"] = referenceImages
		}
	} else if len(referenceImages) == 1 {
		payload["image"] = referenceImages[0]
	} else if len(referenceImages) > 1 {
		payload["reference_images"] = referenceImages
	}
	if request.ReferenceVideo != nil && strings.TrimSpace(request.ReferenceVideo.DataURL) != "" {
		payload["video"] = request.ReferenceVideo.DataURL
	}
	if request.Duration > 0 {
		payload["duration"] = request.Duration
	}
	if request.FPS > 0 {
		payload["fps"] = request.FPS
	}
	if aspectRatio := strings.TrimSpace(request.AspectRatio); aspectRatio != "" {
		payload["aspect_ratio"] = aspectRatio
	}
	if resolution := strings.TrimSpace(request.Resolution); resolution != "" {
		payload["resolution"] = resolution
	}
	return payload, nil
}

func playgroundGeminiImageMedia(image PlaygroundRunImageInput) (map[string]any, error) {
	data, mimeType, err := decodePlaygroundImageDataURL(image.DataURL)
	if err != nil {
		return nil, err
	}
	if requestType := strings.TrimSpace(image.Type); strings.HasPrefix(strings.ToLower(requestType), "image/") {
		mimeType = requestType
	}
	return map[string]any{
		"bytesBase64Encoded": base64.StdEncoding.EncodeToString(data),
		"mimeType":           mimeType,
	}, nil
}

func (s *PlaygroundRunService) executePlaygroundVideoJSON(ctx context.Context, method, endpointURL, apiKey string, payload any) (any, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpointURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parsePlaygroundUpstreamError(resp)
	}
	limited := &io.LimitedReader{R: resp.Body, N: 8<<20 + 1}
	decoder := json.NewDecoder(limited)
	decoder.UseNumber()
	var result any
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if limited.N <= 0 {
		return nil, errors.New("video provider response exceeds 8 MiB")
	}
	return result, nil
}

func playgroundVideoTaskID(payload any, geminiVideo bool) string {
	if geminiVideo {
		return playgroundStringAtPath(payload, "name")
	}
	for _, path := range []string{"request_id", "id", "task_id", "data.request_id", "data.id", "data.task_id", "video.request_id", "video.id"} {
		if value := playgroundStringAtPath(payload, path); value != "" {
			return value
		}
	}
	return ""
}

func playgroundVideoTaskState(payload any, geminiVideo bool) (playgroundVideoTaskStatus, string) {
	if geminiVideo {
		if message := playgroundVideoErrorMessage(payload); message != "" && playgroundBoolAtPath(payload, "done") {
			return playgroundVideoTaskFailed, message
		}
		if playgroundBoolAtPath(payload, "done") {
			return playgroundVideoTaskSucceeded, ""
		}
		return playgroundVideoTaskPending, ""
	}
	status := ""
	for _, path := range []string{"status", "state", "data.status", "data.state"} {
		if status = strings.ToLower(playgroundStringAtPath(payload, path)); status != "" {
			break
		}
	}
	switch status {
	case "completed", "complete", "succeeded", "success", "done", "finished":
		return playgroundVideoTaskSucceeded, ""
	case "failed", "failure", "error", "cancelled", "canceled", "expired":
		return playgroundVideoTaskFailed, playgroundVideoErrorMessage(payload)
	}
	if len(extractPlaygroundVideosFromAny(payload)) > 0 {
		return playgroundVideoTaskSucceeded, ""
	}
	return playgroundVideoTaskPending, ""
}

func playgroundVideoErrorMessage(payload any) string {
	for _, path := range []string{"error.message", "error", "message", "data.error.message", "data.error", "failure_reason"} {
		if value := playgroundStringAtPath(payload, path); value != "" {
			return value
		}
	}
	return ""
}

func playgroundStringAtPath(payload any, path string) string {
	value := playgroundValueAtPath(payload, path)
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	default:
		return ""
	}
}

func playgroundBoolAtPath(payload any, path string) bool {
	value, _ := playgroundValueAtPath(payload, path).(bool)
	return value
}

func playgroundValueAtPath(payload any, path string) any {
	current := payload
	for _, segment := range strings.Split(path, ".") {
		switch typed := current.(type) {
		case map[string]any:
			current = typed[segment]
		case []any:
			index := -1
			if _, err := fmt.Sscanf(segment, "%d", &index); err != nil || index < 0 || index >= len(typed) {
				return nil
			}
			current = typed[index]
		default:
			return nil
		}
	}
	return current
}

func extractPlaygroundVideosFromAny(payload any) []PlaygroundRunVideo {
	videos := make([]PlaygroundRunVideo, 0, 1)
	seen := make(map[string]struct{})
	var walk func(any)
	walk = func(value any) {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			videoURL := firstPlaygroundString(typed, "video_url", "url", "uri")
			if videoURL == "" {
				if encoded := firstPlaygroundString(typed, "b64", "b64_json"); encoded != "" {
					videoURL = "data:video/mp4;base64," + encoded
				}
			}
			if videoURL != "" {
				if _, duplicate := seen[videoURL]; !duplicate {
					seen[videoURL] = struct{}{}
					video := PlaygroundRunVideo{
						URL:          videoURL,
						ThumbnailURL: firstPlaygroundString(typed, "thumbnail_url", "thumbnailUrl", "cover_url"),
						Width:        playgroundAnyInt(typed["width"]),
						Height:       playgroundAnyInt(typed["height"]),
						MimeType:     firstPlaygroundString(typed, "mime_type", "mimeType"),
					}
					if duration := playgroundAnyInt(typed["duration"]); duration > 0 {
						video.Duration = &duration
					}
					if video.MimeType == "" {
						video.MimeType = "video/mp4"
					}
					videos = append(videos, video)
				}
			}
			for key, child := range typed {
				lowerKey := strings.ToLower(key)
				if key == "url" || key == "uri" || key == "video_url" || key == "thumbnail_url" || key == "cover_url" || key == "b64" || key == "b64_json" || strings.Contains(lowerKey, "thumbnail") || strings.Contains(lowerKey, "cover") {
					continue
				}
				walk(child)
			}
		}
	}
	walk(payload)
	return videos
}

func firstPlaygroundString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func playgroundAnyInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func (s *PlaygroundRunService) materializePlaygroundImages(ctx context.Context, request PlaygroundRunRequest, baseURL string, images []PlaygroundRunImage) error {
	for index := range images {
		if len(images[index].data) > 0 {
			mimeType := playgroundImageContentType(images[index].data, images[index].MimeType)
			if !isPlaygroundImageContentType(mimeType) {
				return errors.New("image provider asset did not contain a supported image")
			}
			images[index].MimeType = mimeType
			images[index].URL = ""
			continue
		}
		rawURL := strings.TrimSpace(images[index].URL)
		if rawURL == "" {
			return errors.New("image provider returned an empty asset URL")
		}
		var (
			data     []byte
			mimeType string
			err      error
		)
		switch {
		case strings.HasPrefix(strings.ToLower(rawURL), "data:image/"):
			data, mimeType, err = decodePlaygroundImageDataURL(rawURL)
		case strings.HasPrefix(rawURL, "/"):
			var assetURL string
			assetURL, err = playgroundLocalAssetURL(baseURL, rawURL)
			if err == nil {
				data, mimeType, err = s.downloadPlaygroundImage(ctx, assetURL, request.APIKey)
			}
		default:
			var assetURL string
			assetURL, err = playgroundRemoteAssetURL(rawURL)
			if err == nil {
				// Provider-controlled CDN URLs must never receive the user's Sub2API key.
				data, mimeType, err = s.downloadPlaygroundImage(ctx, assetURL, "")
			}
		}
		if err != nil {
			return err
		}
		mimeType = playgroundImageContentType(data, mimeType)
		if !isPlaygroundImageContentType(mimeType) {
			return errors.New("image provider asset did not contain a supported image")
		}
		images[index].data = data
		images[index].MimeType = mimeType
		images[index].URL = ""
	}
	return nil
}

func (s *PlaygroundRunService) commitPlaygroundVideos(ctx context.Context, key string, request PlaygroundRunRequest, baseURL string, started time.Time, videos []PlaygroundRunVideo) error {
	recoverableURLs := make(map[int]string)
	for index := range videos {
		rawURL := strings.TrimSpace(videos[index].URL)
		if rawURL == "" {
			return errors.New("video provider returned an empty asset URL")
		}
		var (
			data     []byte
			mimeType string
			err      error
		)
		switch {
		case strings.HasPrefix(strings.ToLower(rawURL), "data:video/"):
			data, mimeType, err = decodePlaygroundVideoDataURL(rawURL)
		case strings.HasPrefix(rawURL, "/"):
			var assetURL string
			assetURL, err = playgroundLocalAssetURL(baseURL, rawURL)
			if err == nil {
				data, mimeType, err = s.downloadPlaygroundVideo(ctx, assetURL, request.APIKey)
			}
		default:
			var assetURL string
			assetURL, err = playgroundRemoteAssetURL(rawURL)
			if err == nil {
				recoverableURLs[index] = assetURL
				// Absolute asset URLs are provider-controlled. Never forward the user's Sub2API key.
				data, mimeType, err = s.downloadPlaygroundVideo(ctx, assetURL, "")
			}
		}
		if err != nil {
			return err
		}
		videos[index].data = data
		videos[index].MimeType = mimeType
		videos[index].URL = ""
		videos[index].ThumbnailURL = ""
	}
	updated := false
	var runUserID int64
	var runID string
	s.update(key, func(run *PlaygroundRun) {
		if isTerminalPlaygroundRunStatus(run.Status) {
			return
		}
		run.Videos = videos
		runUserID = run.UserID
		runID = run.ID
		run.DurationMs = time.Since(started).Milliseconds()
		run.UpdatedAt = time.Now()
		updated = true
	})
	if !updated {
		return context.Canceled
	}
	if s.videoAssetRepo != nil && runUserID > 0 && strings.TrimSpace(runID) != "" {
		for index, sourceURL := range recoverableURLs {
			metadata := PlaygroundVideoAssetMetadata{
				UserID:     runUserID,
				RunID:      runID,
				AssetIndex: index,
				SourceURL:  sourceURL,
				MimeType:   videos[index].MimeType,
			}
			if err := s.videoAssetRepo.Upsert(ctx, metadata); err != nil {
				log.Printf("playground video URL persistence failed for user %d run %s asset %d: %v", runUserID, runID, index, err)
			}
		}
	}
	return nil
}

func playgroundRemoteAssetURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return "", errors.New("invalid remote playground asset URL")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("remote playground asset URL must use HTTP or HTTPS")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return "", errors.New("remote playground asset URL contains unsupported components")
	}
	return parsed.String(), nil
}

func playgroundLocalAssetURL(baseURL, assetPath string) (string, error) {
	root, err := url.Parse(strings.TrimRight(strings.TrimSpace(baseURL), "/"))
	if err != nil || root.Scheme == "" || root.Host == "" {
		return "", errors.New("invalid playground request base url")
	}
	asset, err := url.Parse(strings.TrimSpace(assetPath))
	if err != nil || asset.IsAbs() || !strings.HasPrefix(asset.Path, "/") {
		return "", errors.New("invalid local playground video URL")
	}
	root.Path = asset.Path
	root.RawPath = asset.RawPath
	root.RawQuery = asset.RawQuery
	return root.String(), nil
}

func (s *PlaygroundRunService) downloadPlaygroundVideo(ctx context.Context, assetURL, apiKey string) ([]byte, string, error) {
	data, mimeType, err := s.downloadPlaygroundAsset(ctx, assetURL, apiKey, playgroundVideoMaxBytes, "video")
	if err != nil {
		return nil, "", err
	}
	mimeType = playgroundVideoContentType(data, mimeType)
	if !isPlaygroundVideoContentType(mimeType) {
		return nil, "", errors.New("video provider asset did not contain a supported video")
	}
	return data, mimeType, nil
}

func (s *PlaygroundRunService) downloadPlaygroundImage(ctx context.Context, assetURL, apiKey string) ([]byte, string, error) {
	return s.downloadPlaygroundAsset(ctx, assetURL, apiKey, playgroundImageMaxBytes, "image")
}

func (s *PlaygroundRunService) downloadPlaygroundAsset(ctx context.Context, assetURL, apiKey string, maxBytes int64, assetKind string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return nil, "", err
	}
	if apiKey = strings.TrimSpace(apiKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", parsePlaygroundUpstreamError(resp)
	}
	limited := &io.LimitedReader{R: resp.Body, N: maxBytes + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, "", err
	}
	if limited.N <= 0 {
		return nil, "", fmt.Errorf("playground %s exceeds %d MiB", assetKind, maxBytes>>20)
	}
	if len(data) == 0 {
		return nil, "", fmt.Errorf("playground %s is empty", assetKind)
	}
	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	return data, mimeType, nil
}

func playgroundVideoContentType(data []byte, fallback string) string {
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(data), ";")[0]))
	if isPlaygroundVideoContentType(detected) {
		return detected
	}
	if strings.HasPrefix(detected, "application/json") || detected == "text/html" {
		return "application/octet-stream"
	}
	fallback = strings.ToLower(strings.TrimSpace(strings.Split(fallback, ";")[0]))
	if isPlaygroundVideoContentType(fallback) {
		return fallback
	}
	return "application/octet-stream"
}

func isPlaygroundVideoContentType(value string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(value)), "video/")
}

func decodePlaygroundVideoDataURL(dataURL string) ([]byte, string, error) {
	value := strings.TrimSpace(dataURL)
	lower := strings.ToLower(value)
	const marker = ";base64,"
	comma := strings.Index(lower, marker)
	if !strings.HasPrefix(lower, "data:video/") || comma < 0 {
		return nil, "", errors.New("video result must be a base64 video data URL")
	}
	mimeType := strings.TrimSpace(value[len("data:"):comma])
	data, err := base64.StdEncoding.DecodeString(value[comma+len(marker):])
	if err != nil {
		return nil, "", errors.New("invalid base64 video data")
	}
	if len(data) == 0 {
		return nil, "", errors.New("playground video is empty")
	}
	if len(data) > playgroundVideoMaxBytes {
		return nil, "", errors.New("playground video exceeds 512 MiB")
	}
	return data, mimeType, nil
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
	return s.executeParallelImageRequests(ctx, key, request, baseURL, started, imageCount, func(requestCtx context.Context) ([]PlaygroundRunImage, error) {
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
	return s.executeParallelImageRequests(ctx, key, request, baseURL, started, imageCount, func(requestCtx context.Context) ([]PlaygroundRunImage, error) {
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
	return s.executeParallelImageRequests(ctx, key, request, baseURL, started, imageCount, func(requestCtx context.Context) ([]PlaygroundRunImage, error) {
		return s.executeImageRequest(requestCtx, endpointURL, request.APIKey, contentType, bytes.NewReader(bodyBytes), outputFormat)
	})
}

type playgroundImageRequestResult struct {
	images []PlaygroundRunImage
}

func (s *PlaygroundRunService) executeParallelImageRequests(
	ctx context.Context,
	key string,
	request PlaygroundRunRequest,
	baseURL string,
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
	if err := s.materializePlaygroundImages(requestCtx, request, baseURL, allImages); err != nil {
		return nil, err
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
	for index, video := range run.Videos {
		if len(video.data) == 0 {
			continue
		}
		pipe.Set(ctx, playgroundRunRedisVideoKeyFor(run.UserID, run.ID, index), video.data, s.ttl)
	}
	for index, audio := range run.Audios {
		if len(audio.data) == 0 {
			continue
		}
		pipe.Set(ctx, playgroundRunRedisAudioKeyFor(run.UserID, run.ID, index), audio.data, s.ttl)
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

func (s *PlaygroundRunService) loadPersistentVideo(userID int64, id string, index int, mimeType string) (PlaygroundRunVideoAsset, bool, error) {
	if s.rdb == nil || userID <= 0 || index < 0 {
		return PlaygroundRunVideoAsset{}, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	data, err := s.rdb.Get(ctx, playgroundRunRedisVideoKeyFor(userID, id, index)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return PlaygroundRunVideoAsset{}, false, nil
		}
		return PlaygroundRunVideoAsset{}, false, err
	}
	asset, err := playgroundRunVideoAsset(data, mimeType)
	return asset, true, err
}

func (s *PlaygroundRunService) loadPersistentAudio(userID int64, id string, index int, mimeType string) (PlaygroundRunAudioAsset, bool, error) {
	if s.rdb == nil || userID <= 0 || index < 0 {
		return PlaygroundRunAudioAsset{}, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	data, err := s.rdb.Get(ctx, playgroundRunRedisAudioKeyFor(userID, id, index)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return PlaygroundRunAudioAsset{}, false, nil
		}
		return PlaygroundRunAudioAsset{}, false, err
	}
	return playgroundRunAudioAsset(data, mimeType), true, nil
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

func (s *PlaygroundRunService) deletePersistentVideos(userID int64, id string, videoCount int) {
	if s.rdb == nil || videoCount <= 0 {
		return
	}
	keys := make([]string, 0, videoCount)
	for index := 0; index < videoCount; index++ {
		keys = append(keys, playgroundRunRedisVideoKeyFor(userID, id, index))
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("playground run video cleanup failed for user %d run %s: %v", userID, id, err)
	}
}

func (s *PlaygroundRunService) deletePersistentAudios(userID int64, id string, audioCount int) {
	if s.rdb == nil || audioCount <= 0 {
		return
	}
	keys := make([]string, 0, audioCount)
	for index := 0; index < audioCount; index++ {
		keys = append(keys, playgroundRunRedisAudioKeyFor(userID, id, index))
	}
	ctx, cancel := context.WithTimeout(context.Background(), playgroundRunRedisTimeout)
	defer cancel()
	if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
		log.Printf("playground run audio cleanup failed for user %d run %s: %v", userID, id, err)
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

func playgroundRunRedisVideoKeyFor(userID int64, id string, index int) string {
	return fmt.Sprintf("%s%s%d", playgroundRunRedisKey(userID, id), playgroundRunRedisVideoKey, index)
}

func playgroundRunRedisAudioKeyFor(userID int64, id string, index int) string {
	return fmt.Sprintf("%s%s%d", playgroundRunRedisKey(userID, id), playgroundRunRedisAudioKey, index)
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
	out.Videos = append([]PlaygroundRunVideo(nil), run.Videos...)
	for index := range out.Videos {
		if len(run.Videos[index].data) > 0 {
			assetIndex := index
			out.Videos[index].AssetIndex = &assetIndex
			out.Videos[index].URL = ""
		}
		out.Videos[index].data = nil
	}
	out.Audios = append([]PlaygroundRunAudio(nil), run.Audios...)
	for index := range out.Audios {
		if len(run.Audios[index].data) > 0 {
			assetIndex := index
			out.Audios[index].AssetIndex = &assetIndex
		}
		out.Audios[index].data = nil
	}
	if run.Mode == "image" || run.Mode == "video" || run.Mode == "audio" {
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
	out.Videos = append([]PlaygroundRunVideo(nil), run.Videos...)
	for index := range out.Videos {
		out.Videos[index].data = append([]byte(nil), run.Videos[index].data...)
	}
	out.Audios = append([]PlaygroundRunAudio(nil), run.Audios...)
	for index := range out.Audios {
		out.Audios[index].data = append([]byte(nil), run.Audios[index].data...)
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

func playgroundRunVideoAsset(data []byte, mimeType string) (PlaygroundRunVideoAsset, error) {
	mimeType = playgroundVideoContentType(data, mimeType)
	if !isPlaygroundVideoContentType(mimeType) {
		return PlaygroundRunVideoAsset{}, errors.New("stored playground video does not contain a supported video")
	}
	return PlaygroundRunVideoAsset{Data: append([]byte(nil), data...), ContentType: mimeType}, nil
}

func playgroundRunAudioAsset(data []byte, mimeType string) PlaygroundRunAudioAsset {
	return PlaygroundRunAudioAsset{Data: append([]byte(nil), data...), ContentType: playgroundAudioMimeType(mimeType)}
}

func playgroundAudioMimeType(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	if strings.HasPrefix(value, "audio/") {
		return value
	}
	switch value {
	case "aac":
		return "audio/aac"
	case "flac":
		return "audio/flac"
	case "opus":
		return "audio/ogg"
	case "pcm":
		return "audio/pcm"
	case "wav":
		return "audio/wav"
	default:
		return "audio/mpeg"
	}
}

func cancelPlaygroundRun(run *PlaygroundRun, now time.Time) {
	if run == nil || isTerminalPlaygroundRunStatus(run.Status) {
		return
	}
	run.Status = PlaygroundRunCanceled
	run.Error = "request canceled"
	run.Images = nil
	run.Videos = nil
	run.Audios = nil
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
	message = sanitizePlaygroundUpstreamErrorMessage(message)
	return fmt.Errorf("%s (HTTP %d)", message, resp.StatusCode)
}

var playgroundUpstreamGroupPrefixPattern = regexp.MustCompile(`^分组\s+.+?\s+下模型\s+(.+)$`)

func sanitizePlaygroundUpstreamErrorMessage(message string) string {
	message = strings.TrimSpace(message)
	if matches := playgroundUpstreamGroupPrefixPattern.FindStringSubmatch(message); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return message
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
