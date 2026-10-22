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

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/mediagateway"
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

	// Only this fork's video adapter is gated. Historical reads drain without
	// consulting state, and the upstream Seedance handler is untouched.
	if endpoint == GrokMediaEndpointVideosGenerations {
		if err := s.CheckMediaGatewayAdmission(ctx, mediagateway.Submit); err != nil {
			return nil, err
		}
	}
	token, _, err := s.getRequestCredential(ctx, c, account)
	if err != nil {
		return nil, err
	}
	baseURL, err := s.validateUpstreamBaseURL(account.GetOpenAIBaseURL())
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
	nativeTask, nativeLookup, err := parseNativeVideoTask(requestID)
	if err != nil {
		return nil, err
	}
	protocol := ""
	if nativeLookup {
		protocol = nativeTask.Protocol
		if endpoint == GrokMediaEndpointVideoContent && protocol != "omni" {
			return nil, errors.New("native video provider returned no downloadable output; query task status for provider details")
		}
		escapedID := url.PathEscape(nativeTask.ID)
		if strings.Contains(nativeTask.Path, "?") {
			escapedID = url.QueryEscape(nativeTask.ID)
		}
		targetURL, err = nativeVideoURL(baseURL, nativeTask.Path+escapedID)
		if err != nil {
			return nil, err
		}
	}

	requestInfo := ParseGrokMediaRequest(contentType, body)
	upstreamModel := strings.TrimSpace(account.GetMappedModel(requestInfo.Model))
	if upstreamModel == "" {
		upstreamModel = requestInfo.Model
	}
	if nativeLookup {
		upstreamModel = nativeTask.Model
	}
	if endpoint == GrokMediaEndpointVideosGenerations {
		if arkNative && gjson.GetBytes(body, "_video_protocol").String() == "relay" {
			writeGrokMediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "当前账号地址是 /api/v3 原生方舟接口，兼容网关格式需配置网关地址")
			return nil, errors.New("relay format requires a relay account base URL")
		}
		protocol = nativeVideoProtocol(baseURL, body)
		if upstreamModel != requestInfo.Model && gjson.ValidBytes(body) {
			body, err = sjson.SetBytes(body, "model", upstreamModel)
			if err != nil {
				return nil, fmt.Errorf("rewrite video model: %w", err)
			}
		}
		if arkNative && protocol == "" {
			body, err = normalizeArkVideoGenerationBody(body)
			if err != nil {
				return nil, err
			}
			contentType = "application/json"
		}
		if protocol != "" {
			var path string
			body, path, nativeTask, err = nativeVideoGeneration(protocol, body)
			if err != nil {
				writeGrokMediaErrorResponse(c, http.StatusBadRequest, "invalid_request_error", sanitizeUpstreamErrorMessage(err.Error()))
				return nil, err
			}
			nativeTask.BillingModel = requestInfo.Model
			targetURL, err = nativeVideoURL(baseURL, path)
			if err != nil {
				return nil, err
			}
			contentType = "application/json"
		} else if !arkNative && gjson.ValidBytes(body) {
			body, err = normalizeOpenAICompatibleVideoRelayBody(body, requestInfo.Model)
			if err != nil {
				return nil, err
			}
		}
		logVideoReferencePayload(ctx, "upstream_forward", account.ID, body)
	}

	var bodyReader io.Reader
	method := http.MethodGet
	if endpoint == GrokMediaEndpointVideosGenerations {
		method = http.MethodPost
		bodyReader = bytes.NewReader(body)
	}
	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	if protocol != "" {
		upstreamCtx = WithHTTPUpstreamRedirectsDisabled(upstreamCtx)
	}
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, method, targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+token)
	if protocol == "omni" {
		upstreamReq.Header.Del("Authorization")
		upstreamReq.Header.Set("x-goog-api-key", token)
	}
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
	if protocol != "" && resp.StatusCode >= 300 && resp.StatusCode < 400 {
		writeGrokMediaErrorResponse(c, http.StatusBadGateway, "upstream_error", "原生视频接口不允许重定向，请检查渠道地址")
		return nil, errors.New("native video redirect rejected")
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return s.handleCompatErrorResponse(resp, c, account, writeGrokMediaErrorResponse, requestInfo.Model)
	}
	requestIDHeader := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("x-goog-request-id"))
	if endpoint == GrokMediaEndpointVideoContent && protocol != "omni" {
		if err := writeGrokMediaContentResponse(c, resp); err != nil {
			return nil, err
		}
		return openAICompatibleVideoContentResult(requestID, requestIDHeader, resp.Header, time.Since(started)), nil
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	if arkNative && protocol == "" && endpoint == GrokMediaEndpointVideoStatus {
		respBody = normalizeArkVideoStatusResponse(respBody)
	}
	if protocol != "" {
		respBody, nativeTask, err = normalizeNativeVideoResponse(respBody, nativeTask, endpoint == GrokMediaEndpointVideosGenerations)
		if err != nil {
			writeGrokMediaErrorResponse(c, http.StatusBadGateway, "upstream_error", sanitizeUpstreamErrorMessage(err.Error()))
			return nil, err
		}
		if fileID := gjson.GetBytes(respBody, "file_id").String(); protocol == "minimax" && fileID != "" {
			fileURL, err := nativeVideoURL(baseURL, "/v1/files/retrieve?file_id="+url.QueryEscape(fileID))
			if err != nil {
				return nil, err
			}
			fileReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodGet, fileURL, nil)
			if err != nil {
				return nil, err
			}
			fileReq.Header = upstreamReq.Header.Clone()
			fileResp, err := s.httpUpstream.Do(fileReq, proxyURL, account.ID, account.Concurrency)
			if err != nil {
				return nil, err
			}
			defer fileResp.Body.Close()
			if fileResp.StatusCode >= 400 {
				return s.handleCompatErrorResponse(fileResp, c, account, writeGrokMediaErrorResponse, requestInfo.Model)
			}
			fileBody, err := ReadUpstreamResponseBody(fileResp.Body, s.cfg, c, openAITooLargeError)
			if err != nil {
				return nil, err
			}
			if download := gjson.GetBytes(fileBody, "file.download_url").String(); download != "" {
				respBody, _ = sjson.SetBytes(respBody, "video.url", download)
			} else {
				return nil, errors.New("MiniMax completed without a file download URL")
			}
		}
		if protocol == "omni" {
			output := gjson.GetBytes(respBody, "video.url").String()
			if endpoint != GrokMediaEndpointVideoContent && output != "" && !strings.HasPrefix(output, "data:") {
				state, err := s.nativeOmniFileState(upstreamCtx, account, baseURL, output, token, proxyURL)
				if err != nil {
					return nil, err
				}
				if state != "ACTIVE" {
					respBody, _ = sjson.DeleteBytes(respBody, "video")
					status := "in_progress"
					if state == "FAILED" {
						status = "failed"
						respBody, _ = sjson.SetBytes(respBody, "error.message", "Gemini Omni 视频文件处理失败")
					}
					respBody, _ = sjson.SetBytes(respBody, "status", status)
					output = ""
				}
			}
			if endpoint == GrokMediaEndpointVideoContent {
				if strings.HasPrefix(output, "data:video/") {
					data, mime, err := decodePlaygroundVideoDataURL(output)
					if err != nil {
						return nil, err
					}
					c.Data(http.StatusOK, mime, data)
				} else {
					if err := s.forwardNativeOmniContent(upstreamCtx, c, account, baseURL, output, token, proxyURL); err != nil {
						return nil, err
					}
				}
				return openAICompatibleVideoContentResult(requestID, requestIDHeader, resp.Header, time.Since(started)), nil
			}
			if output != "" && !strings.HasPrefix(output, "data:") {
				if _, err := nativeOmniContentURL(baseURL, output); err != nil {
					return nil, err
				}
				respBody, _ = sjson.SetBytes(respBody, "video.url", "/v1/videos/"+nativeTask.key()+"/content")
			}
		}
	}
	writeGrokMediaResponse(c, resp, respBody, s.responseHeaderFilter)
	usage := openAICompatibleVideoUsageFromResponse(endpoint, requestInfo, respBody)
	if usage.VideoCount > 0 && strings.TrimSpace(usage.ResponseID) == "" {
		// Synchronous relays may return only the final asset. Use their request
		// header as the stable one-shot billing key when no task id is present.
		usage.ResponseID = requestIDHeader
	}
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

func nativeOmniContentURL(baseURL, output string) (string, error) {
	base, _ := url.Parse(baseURL)
	target, err := url.Parse(output)
	if err != nil || base == nil || target.Scheme != "https" || target.User != nil || target.Fragment != "" || (target.Port() != "" && target.Port() != "443") {
		return "", errors.New("Gemini Omni returned an invalid video download URL")
	}
	if target.Hostname() != base.Hostname() && target.Hostname() != "generativelanguage.googleapis.com" && target.Hostname() != "storage.googleapis.com" {
		return "", errors.New("Gemini Omni returned an untrusted video download host")
	}
	return target.String(), nil
}

func (s *OpenAIGatewayService) nativeOmniFileState(ctx context.Context, account *Account, baseURL, output, token, proxyURL string) (string, error) {
	validated, err := nativeOmniContentURL(baseURL, output)
	if err != nil {
		return "", err
	}
	target, _ := url.Parse(validated)
	if target.Hostname() != "generativelanguage.googleapis.com" || !strings.HasPrefix(target.Path, "/v1beta/files/") {
		return "ACTIVE", nil
	}
	target.Path = strings.TrimSuffix(target.Path, ":download")
	target.RawPath, target.RawQuery = "", ""
	req, err := http.NewRequestWithContext(WithHTTPUpstreamRedirectsDisabled(ctx), http.MethodGet, target.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("x-goog-api-key", token)
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Gemini Omni file status returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	state := strings.ToUpper(gjson.GetBytes(body, "state").String())
	if !playgroundStringAllowed(state, "ACTIVE", "PROCESSING", "FAILED") {
		return "", errors.New("Gemini Omni returned an unknown file state")
	}
	return state, nil
}

func (s *OpenAIGatewayService) forwardNativeOmniContent(ctx context.Context, c *gin.Context, account *Account, baseURL, output, token, proxyURL string) error {
	target, err := nativeOmniContentURL(baseURL, output)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(WithHTTPUpstreamRedirectsDisabled(ctx), http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("x-goog-api-key", token)
	req.Header.Set("Accept", "video/*")
	resp, err := s.httpUpstream.Do(req, proxyURL, account.ID, account.Concurrency)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Gemini Omni video download returned HTTP %d", resp.StatusCode)
	}
	return writeGrokMediaContentResponse(c, resp)
}

func openAICompatibleVideoContentResult(requestID, requestIDHeader string, headers http.Header, duration time.Duration) *OpenAIForwardResult {
	return &OpenAIForwardResult{
		RequestID:       strings.TrimSpace(requestIDHeader),
		ResponseID:      strings.TrimSpace(requestID),
		ResponseHeaders: headers.Clone(),
		Duration:        duration,
		// A successful content response proves the asynchronous task produced a
		// video. The handler merges the create-time model/pricing snapshot and
		// claims this task id once, so repeated/range downloads cannot double bill.
		VideoCount: 1,
	}
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

func normalizeOpenAICompatibleVideoRelayBody(body []byte, requestModel string) ([]byte, error) {
	standard := strings.TrimSpace(gjson.GetBytes(body, "video_standard").String())
	seedance := strings.HasPrefix(standard, "seedance-") || (standard == "" &&
		(playgroundSeedanceModelVersion(requestModel) != "" || playgroundSeedanceModelVersion(gjson.GetBytes(body, "model").String()) != ""))
	fields := []string{"_video_protocol", "video_standard", "video_mode"}
	// Keep the relay endpoint and unrelated provider options. Remove only aliases
	// after converting every reference into the authoritative Seedance content array.
	if gjson.GetBytes(body, "_video_protocol").String() != "relay" && seedance {
		normalized, err := normalizeArkVideoGenerationBody(body)
		if err != nil {
			return nil, err
		}
		for _, field := range []string{"content", "ratio", "framespersecond", "resolution", "duration"} {
			if value := gjson.GetBytes(normalized, field); value.Exists() {
				body, err = sjson.SetRawBytes(body, field, []byte(value.Raw))
				if err != nil {
					return nil, fmt.Errorf("normalize Seedance field %s: %w", field, err)
				}
			}
		}
		fields = append(fields, "prompt", "image", "image_url", "image_tail", "first_frame_image", "last_frame_image", "last_frame",
			"reference_images", "video", "reference_videos", "audio", "reference_audios", "aspect_ratio", "fps", "size", "seconds")
	}
	var err error
	for _, field := range fields {
		body, err = sjson.DeleteBytes(body, field)
		if err != nil {
			return nil, fmt.Errorf("remove video alias field %s: %w", field, err)
		}
	}
	return body, nil
}

func normalizeArkVideoGenerationBody(body []byte) ([]byte, error) {
	if !gjson.ValidBytes(body) {
		return nil, errors.New("invalid video generation JSON")
	}
	content := []map[string]any{{
		"type": "text",
		"text": gjson.GetBytes(body, "prompt").String(),
	}}
	videoURLs := openAICompatibleVideoReferenceURLs(body, "reference_videos", "video")
	audioURLs := openAICompatibleVideoReferenceURLs(body, "reference_audios", "audio")
	imageURL := openAICompatibleVideoImageURL(body)
	if imageURL != "" {
		role := "first_frame"
		if (len(videoURLs) > 0 || len(audioURLs) > 0) && strings.TrimSpace(gjson.GetBytes(body, "first_frame_image").String()) == "" {
			role = "reference_image"
		}
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": imageURL,
			},
			"role": role,
		})
	}
	lastFrameURL := openAICompatibleVideoImageTailURL(body)
	if lastFrameURL != "" {
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]any{
				"url": lastFrameURL,
			},
			"role": "last_frame",
		})
	}
	for _, reference := range gjson.GetBytes(body, "reference_images").Array() {
		if reference.Type != gjson.String || strings.TrimSpace(reference.String()) == "" {
			continue
		}
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": strings.TrimSpace(reference.String())},
			"role":      "reference_image",
		})
	}
	for _, media := range []struct {
		kind string
		urls []string
	}{{"video", videoURLs}, {"audio", audioURLs}} {
		for _, reference := range media.urls {
			content = append(content, map[string]any{
				"type":              media.kind + "_url",
				media.kind + "_url": map[string]any{"url": reference},
				"role":              "reference_" + media.kind,
			})
		}
	}
	payload := map[string]any{
		"model":   gjson.GetBytes(body, "model").String(),
		"content": content,
	}
	if existing := gjson.GetBytes(body, "content"); existing.IsArray() && len(existing.Array()) > 0 {
		payload["content"] = existing.Value()
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
	copyNumber("framespersecond", "framespersecond")
	copyString("aspect_ratio", "ratio")
	copyString("ratio", "ratio")
	copyString("resolution", "resolution")
	for _, field := range []string{"watermark", "generate_audio", "seed", "camera_fixed", "return_last_frame", "draft", "service_tier", "output_format", "omni_reference_task_type"} {
		if value := gjson.GetBytes(body, field); value.Exists() {
			payload[field] = value.Value()
		}
	}
	info := ParseGrokMediaRequest("application/json", body)
	if _, exists := payload["resolution"]; !exists && strings.TrimSpace(info.Size) != "" {
		if resolution := videoBillingResolutionFromSize(info.Size); resolution != "" {
			payload["resolution"] = resolution
		}
	}
	if _, exists := payload["duration"]; !exists && gjson.GetBytes(body, "seconds").Exists() {
		payload["duration"] = info.DurationSeconds
	}
	return json.Marshal(payload)
}

func openAICompatibleVideoReferenceURLs(body []byte, field, legacyField string) []string {
	urls := make([]string, 0)
	for _, item := range gjson.GetBytes(body, field).Array() {
		if item.Type == gjson.String && strings.TrimSpace(item.String()) != "" {
			urls = append(urls, strings.TrimSpace(item.String()))
		}
	}
	legacy := gjson.GetBytes(body, legacyField)
	if legacy.Type == gjson.String && strings.TrimSpace(legacy.String()) != "" {
		value := strings.TrimSpace(legacy.String())
		for _, reference := range urls {
			if reference == value {
				return urls
			}
		}
		urls = append(urls, value)
	}
	return urls
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

func openAICompatibleVideoImageTailURL(body []byte) string {
	for _, path := range []string{
		"image_tail",
		"last_frame_image",
		"last_frame",
		"image.last_frame",
		"image.image_tail",
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
	if resolution := extractVideoResponseResolution(body); resolution != "" {
		payload["resolution"] = resolution
	}
	if duration := extractVideoStatusDurationSeconds(body, true); duration > 0 {
		payload["duration"] = duration
	}
	if model := strings.TrimSpace(gjson.GetBytes(body, "model").String()); model != "" {
		payload["model"] = model
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
