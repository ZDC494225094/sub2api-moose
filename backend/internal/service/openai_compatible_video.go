package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const arkVideoTasksPath = "/contents/generations/tasks"

// ForwardOpenAICompatibleVideo forwards the normalized /v1/videos contract to
// an OpenAI API-key account. Native Volcengine Ark accounts are detected from
// their configured /api/v3 base URL and translated to the Seedance task API.
func (s *OpenAIGatewayService) ForwardOpenAICompatibleVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint GrokMediaEndpoint,
	requestID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	started := time.Now()
	if account == nil || account.Platform != PlatformOpenAI {
		return nil, errors.New("openai video account is required")
	}
	if endpoint != GrokMediaEndpointVideosGenerations && endpoint != GrokMediaEndpointVideoStatus && endpoint != GrokMediaEndpointVideoContent {
		return nil, fmt.Errorf("unsupported OpenAI-compatible video endpoint: %s", endpoint)
	}

	token, _, err := s.getRequestCredential(ctx, c, account)
	if err != nil {
		return nil, err
	}
	targetURL, arkNative, err := s.openAICompatibleVideoURL(account, endpoint, requestID)
	if err != nil {
		return nil, err
	}
	if arkNative && endpoint == GrokMediaEndpointVideoContent {
		return nil, errors.New("native Ark video tasks do not expose a content proxy endpoint")
	}

	requestInfo := ParseGrokMediaRequest(contentType, body)
	upstreamModel := strings.TrimSpace(account.GetMappedModel(requestInfo.Model))
	if upstreamModel == "" {
		upstreamModel = requestInfo.Model
	}
	if endpoint == GrokMediaEndpointVideosGenerations {
		if upstreamModel != requestInfo.Model && gjson.ValidBytes(body) {
			body, err = sjson.SetBytes(body, "model", upstreamModel)
			if err != nil {
				return nil, fmt.Errorf("rewrite video model: %w", err)
			}
		}
		if arkNative {
			body, err = normalizeArkVideoGenerationBody(body)
			if err != nil {
				return nil, err
			}
			contentType = "application/json"
		}
	}

	var bodyReader io.Reader
	method := http.MethodGet
	if endpoint == GrokMediaEndpointVideosGenerations {
		method = http.MethodPost
		bodyReader = bytes.NewReader(body)
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, method, targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	if method == http.MethodPost {
		if strings.TrimSpace(contentType) == "" {
			contentType = "application/json"
		}
		upstreamReq.Header.Set("Content-Type", contentType)
	}
	if endpoint == GrokMediaEndpointVideoContent {
		upstreamReq.Header.Set("Accept", "*/*")
		if c != nil {
			upstreamReq.Header.Set("Range", strings.TrimSpace(c.GetHeader("Range")))
		}
	} else {
		upstreamReq.Header.Set("Accept", "application/json")
	}
	account.ApplyHeaderOverrides(upstreamReq.Header)

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	upstreamStarted := time.Now()
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStarted).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return s.handleCompatErrorResponse(resp, c, account, writeGrokMediaErrorResponse, requestInfo.Model)
	}
	requestIDHeader := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("x-goog-request-id"))
	if endpoint == GrokMediaEndpointVideoContent {
		if err := writeGrokMediaContentResponse(c, resp); err != nil {
			return nil, err
		}
		return &OpenAIForwardResult{RequestID: requestIDHeader, ResponseHeaders: resp.Header.Clone(), Duration: time.Since(started)}, nil
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if arkNative && endpoint == GrokMediaEndpointVideoStatus {
		respBody = normalizeArkVideoStatusResponse(respBody)
	}
	writeGrokMediaResponse(c, resp, respBody, s.responseHeaderFilter)
	usage := grokMediaUsageFromResponse(endpoint, requestInfo, respBody)
	return &OpenAIForwardResult{
		RequestID:            requestIDHeader,
		ResponseID:           usage.ResponseID,
		Usage:                usage.Usage,
		Model:                requestInfo.Model,
		BillingModel:         requestInfo.Model,
		UpstreamModel:        upstreamModel,
		ResponseHeaders:      resp.Header.Clone(),
		Duration:             time.Since(started),
		ImageCount:           usage.ImageCount,
		VideoCount:           usage.VideoCount,
		VideoResolution:      usage.VideoResolution,
		VideoDurationSeconds: usage.VideoDurationSeconds,
	}, nil
}

func (s *OpenAIGatewayService) openAICompatibleVideoURL(account *Account, endpoint GrokMediaEndpoint, requestID string) (string, bool, error) {
	baseURL, err := s.validateUpstreamBaseURL(account.GetOpenAIBaseURL())
	if err != nil {
		return "", false, err
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", false, err
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	escapedBasePath := strings.TrimRight(parsed.EscapedPath(), "/")
	arkNative := strings.HasSuffix(strings.ToLower(basePath), "/api/v3")

	var suffix string
	var escapedSuffix string
	if arkNative {
		suffix = arkVideoTasksPath
		escapedSuffix = suffix
		if endpoint != GrokMediaEndpointVideosGenerations {
			if strings.TrimSpace(requestID) == "" {
				return "", true, errors.New("video request id is required")
			}
			suffix += "/" + strings.TrimSpace(requestID)
			escapedSuffix += "/" + url.PathEscape(strings.TrimSpace(requestID))
		}
	} else {
		prefix := "/v1"
		if strings.HasSuffix(strings.ToLower(basePath), "/v1") {
			prefix = ""
		}
		switch endpoint {
		case GrokMediaEndpointVideosGenerations:
			suffix = prefix + "/videos/generations"
			escapedSuffix = suffix
		case GrokMediaEndpointVideoStatus, GrokMediaEndpointVideoContent:
			if strings.TrimSpace(requestID) == "" {
				return "", false, errors.New("video request id is required")
			}
			suffix = prefix + "/videos/" + strings.TrimSpace(requestID)
			escapedSuffix = prefix + "/videos/" + url.PathEscape(strings.TrimSpace(requestID))
			if endpoint == GrokMediaEndpointVideoContent {
				suffix += "/content"
				escapedSuffix += "/content"
			}
		default:
			return "", false, fmt.Errorf("unsupported video endpoint: %s", endpoint)
		}
	}
	parsed.Path = basePath + suffix
	parsed.RawPath = escapedBasePath + escapedSuffix
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), arkNative, nil
}

func normalizeArkVideoGenerationBody(body []byte) ([]byte, error) {
	if !gjson.ValidBytes(body) {
		return nil, errors.New("invalid video generation JSON")
	}
	content := []map[string]any{{
		"type": "text",
		"text": gjson.GetBytes(body, "prompt").String(),
	}}
	imageURL := openAICompatibleVideoImageURL(body)
	if imageURL != "" {
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": imageURL,
			},
			"role": "first_frame",
		})
	}
	payload := map[string]any{
		"model":   gjson.GetBytes(body, "model").String(),
		"content": content,
	}
	copyNumber := func(source, target string) {
		if value := gjson.GetBytes(body, source); value.Exists() && value.Type == gjson.Number {
			payload[target] = value.Value()
		}
	}
	copyString := func(source, target string) {
		if value := strings.TrimSpace(gjson.GetBytes(body, source).String()); value != "" {
			payload[target] = value
		}
	}
	copyNumber("duration", "duration")
	copyNumber("fps", "framespersecond")
	copyString("aspect_ratio", "ratio")
	copyString("resolution", "resolution")
	copyString("watermark", "watermark")
	return json.Marshal(payload)
}

func openAICompatibleVideoImageURL(body []byte) string {
	for _, path := range []string{
		"image.url",
		"image.image_url",
		"image",
		"image_url.url",
		"image_url",
		"first_frame_image",
	} {
		value := gjson.GetBytes(body, path)
		if value.Type != gjson.String {
			continue
		}
		if imageURL := strings.TrimSpace(value.String()); imageURL != "" {
			return imageURL
		}
	}
	return ""
}

func normalizeArkVideoStatusResponse(body []byte) []byte {
	if !gjson.ValidBytes(body) {
		return body
	}
	status := strings.ToLower(strings.TrimSpace(gjson.GetBytes(body, "status").String()))
	switch status {
	case "succeeded", "success", "completed":
		status = "completed"
	case "failed", "error", "cancelled", "canceled":
		status = "failed"
	case "queued", "pending", "submitted":
		status = "queued"
	default:
		status = "in_progress"
	}
	videoURL := firstNonEmpty(
		gjson.GetBytes(body, "content.video_url").String(),
		gjson.GetBytes(body, "content.video.url").String(),
		gjson.GetBytes(body, "video_url").String(),
	)
	payload := map[string]any{
		"id":     firstNonEmpty(gjson.GetBytes(body, "id").String(), gjson.GetBytes(body, "task_id").String()),
		"status": status,
	}
	if videoURL != "" {
		payload["video"] = map[string]any{"url": videoURL}
	}
	if errMessage := firstNonEmpty(gjson.GetBytes(body, "error.message").String(), gjson.GetBytes(body, "message").String()); errMessage != "" && status == "failed" {
		payload["error"] = map[string]any{"message": errMessage}
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return normalized
}
