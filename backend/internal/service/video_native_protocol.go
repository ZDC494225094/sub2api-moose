package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
)

type nativeVideoTask struct {
	Protocol     string `json:"p"`
	Path         string `json:"q"`
	ID           string `json:"id"`
	Model        string `json:"m"`
	BillingModel string `json:"b,omitempty"`
}

func (task nativeVideoTask) key() string {
	data, _ := json.Marshal(task)
	return "vstd_" + base64.RawURLEncoding.EncodeToString(data)
}

func parseNativeVideoTask(key string) (nativeVideoTask, bool, error) {
	var task nativeVideoTask
	if !strings.HasPrefix(key, "vstd_") {
		return task, false, nil
	}
	data, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(key, "vstd_"))
	if err != nil || len(data) > 4096 || json.Unmarshal(data, &task) != nil || !isSafeUpstreamPathSegment(task.ID) {
		return task, true, errors.New("invalid native video task")
	}
	// Paths are task-type identifiers, never client-supplied upstream URLs.
	allowed := map[string][]string{
		"ark":     {"/api/v3/contents/generations/tasks/"},
		"minimax": {"/v1/query/video_generation?task_id="}, "minimax-h3": {"/v2/query/video_generation/"},
		"kling": {"/v1/videos/text2video/", "/v1/videos/image2video/"}, "kling-v3": {"/tasks?task_ids="},
		"omni": {"/v1beta/interactions/"},
	}
	for _, path := range allowed[task.Protocol] {
		if task.Path == path {
			return task, true, nil
		}
	}
	return task, true, errors.New("invalid native video task protocol")
}

func nativeVideoProtocol(base string, body []byte) string {
	requested := gjson.GetBytes(body, "_video_protocol").String()
	if requested == "relay" {
		return ""
	}
	if playgroundStringAllowed(requested, "ark", "minimax", "minimax-h3", "kling", "kling-v3", "omni") {
		return requested
	}
	parsed, _ := url.Parse(base)
	if parsed == nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	model := strings.ToLower(gjson.GetBytes(body, "model").String())
	switch host {
	case "api.minimax.io", "api.minimax.chat", "api.minimaxi.com":
		if strings.Contains(model, "h3") || strings.HasPrefix(gjson.GetBytes(body, "video_standard").String(), "minimax-h3") {
			return "minimax-h3"
		}
		return "minimax"
	case "api.klingai.com", "api-singapore.klingai.com", "api-beijing.klingai.com":
		if strings.Contains(model, "3.0") || strings.Contains(model, "v3") || strings.Contains(model, "o1") || strings.HasPrefix(gjson.GetBytes(body, "video_standard").String(), "kling-3") || gjson.GetBytes(body, "video_standard").String() == "kling-o1" {
			return "kling-v3"
		}
		return "kling"
	case "generativelanguage.googleapis.com":
		if strings.Contains(model, "omni") || gjson.GetBytes(body, "video_standard").String() == "gemini-omni" {
			return "omni"
		}
	}
	return ""
}

func nativeVideoURL(base, path string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	prefix := strings.TrimRight(parsed.Path, "/")
	for _, suffix := range []string{"/v1beta", "/v1", "/v2", "/api/v3"} {
		prefix = strings.TrimSuffix(prefix, suffix)
	}
	rel, err := url.Parse(path)
	if err != nil || rel.IsAbs() || !strings.HasPrefix(path, "/") {
		return "", errors.New("invalid native video path")
	}
	parsed.Path, parsed.RawPath = prefix+rel.Path, ""
	parsed.RawQuery, parsed.Fragment = rel.RawQuery, ""
	return parsed.String(), nil
}

func nativeVideoGeneration(protocol string, body []byte) ([]byte, string, nativeVideoTask, error) {
	if !gjson.ValidBytes(body) {
		return nil, "", nativeVideoTask{}, errors.New("invalid video generation JSON")
	}
	if existing := gjson.GetBytes(body, "content"); !existing.IsArray() || len(existing.Array()) == 0 {
		normalized, err := normalizeArkVideoGenerationBody(body)
		if err != nil {
			return nil, "", nativeVideoTask{}, err
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, "", nativeVideoTask{}, err
		}
		payload["content"] = gjson.GetBytes(normalized, "content").Value()
		body, _ = json.Marshal(payload)
	}
	root := gjson.ParseBytes(body)
	task := nativeVideoTask{Protocol: protocol, Model: root.Get("model").String()}
	mode := root.Get("video_mode").String()
	content := root.Get("content").Array()
	images, videos, audios := []gjson.Result{}, []gjson.Result{}, []gjson.Result{}
	for _, item := range content {
		switch item.Get("type").String() {
		case "image_url":
			images = append(images, item)
		case "video_url":
			videos = append(videos, item)
		case "audio_url":
			audios = append(audios, item)
		}
	}
	payload := map[string]any{}
	path := ""
	switch protocol {
	case "ark":
		encoded, err := normalizeArkVideoGenerationBody(body)
		task.Path = "/api/v3/contents/generations/tasks/"
		return encoded, "/api/v3/contents/generations/tasks", task, err
	case "minimax-h3":
		path, task.Path = "/v2/video_generation", "/v2/query/video_generation/"
		payload = map[string]any{"model": task.Model, "content": root.Get("content").Value(), "duration": root.Get("duration").Int(), "resolution": strings.ToUpper(root.Get("resolution").String())}
		if !(mode == "frames" && len(images) > 0) {
			payload["ratio"] = root.Get("aspect_ratio").String()
		}
	case "minimax", "kling":
		if len(videos)+len(audios) > 0 || len(images) > 2 || (len(images) > 1 && mode == "reference") {
			return nil, "", task, errors.New("原生首尾帧接口不支持当前参考组合，请选择 H3 / Omni 标准或兼容网关；素材未截断")
		}
		payload = map[string]any{"prompt": root.Get("prompt").String()}
		if protocol == "minimax" {
			path, task.Path = "/v1/video_generation", "/v1/query/video_generation?task_id="
			payload["model"], payload["duration"], payload["resolution"] = task.Model, root.Get("duration").Int(), strings.ToUpper(root.Get("resolution").String())
			for i, image := range images {
				field := "first_frame_image"
				if i == 1 {
					field = "last_frame_image"
				}
				payload[field] = image.Get("image_url.url").String()
			}
		} else {
			path = "/v1/videos/text2video"
			if len(images) > 0 {
				path = "/v1/videos/image2video"
			}
			task.Path = path + "/"
			payload["model_name"], payload["duration"], payload["mode"] = task.Model, root.Get("duration").String(), firstNonEmpty(root.Get("mode").String(), "pro")
			payload["aspect_ratio"] = root.Get("aspect_ratio").String()
			if value := root.Get("negative_prompt"); value.Exists() {
				payload["negative_prompt"] = value.String()
			}
			if value := root.Get("generate_audio"); value.Exists() {
				payload["sound"] = "off"
				if value.Bool() {
					payload["sound"] = "on"
				}
			}
			for i, image := range images {
				field := "image"
				if i == 1 {
					field = "image_tail"
				}
				payload[field] = image.Get("image_url.url").String()
			}
		}
	case "kling-v3":
		if len(audios) > 0 || len(videos) > 1 {
			return nil, "", task, errors.New("可灵原生接口不支持音频参考或多个视频，素材未截断")
		}
		standard, model := root.Get("video_standard").String(), strings.ToLower(task.Model)
		o1 := standard == "kling-o1" || (standard == "" && strings.Contains(model, "o1"))
		turbo := standard == "kling-3-turbo" || (standard == "" && strings.Contains(model, "turbo"))
		omni := o1 || standard == "kling-3-omni" || (standard == "" && strings.Contains(model, "omni"))
		if !omni && (len(videos) > 0 || len(images) > 2) {
			return nil, "", task, errors.New("当前参考需要可灵 Omni 标准")
		}
		if mode == "frames" && (len(images) > 2 || len(videos) > 0) {
			return nil, "", task, errors.New("可灵首尾帧不能混用视频或超过两张图；素材未截断")
		}
		for _, item := range append(images, videos...) {
			kind := strings.TrimSuffix(item.Get("type").String(), "_url")
			value := item.Get(kind + "_url.url").String()
			if kind == "image" && strings.HasPrefix(value, "data:image/") && strings.Contains(value, ";base64,") {
				continue
			}
			parsed, err := url.Parse(value)
			if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
				return nil, "", task, errors.New("可灵图片需为 HTTP(S) / Base64，视频需为公网 HTTP(S) 地址，请配置托管对象存储")
			}
		}
		if turbo && len(images) > 1 {
			return nil, "", task, errors.New("可灵 3.0 Turbo 仅支持首帧图，素材未截断")
		}
		path, task.Path = "/text-to-video/kling-3.0", "/tasks?task_ids="
		settings := map[string]any{"resolution": strings.ToLower(root.Get("resolution").String()), "duration": root.Get("duration").Int(), "aspect_ratio": root.Get("aspect_ratio").String()}
		if !turbo {
			settings["audio"] = "off"
		}
		if !o1 && !turbo {
			settings["multi_shot"] = mode != "edit"
		}
		if !omni && len(images) > 0 {
			delete(settings, "aspect_ratio")
		}
		if !o1 && !turbo && len(videos) == 0 && root.Get("generate_audio").Bool() {
			settings["audio"] = "native"
		}
		if o1 && len(videos) > 0 && root.Get("generate_audio").Bool() {
			settings["audio"] = "original"
		}
		payload = map[string]any{"settings": settings, "options": map[string]any{"watermark_info": map[string]any{"enabled": root.Get("watermark").Bool()}}}
		if omni {
			path = "/omni-video/kling-3.0-omni"
			if o1 {
				path = "/omni-video/kling-o1"
			}
			parts := []map[string]any{{"type": "prompt", "text": root.Get("prompt").String()}}
			for i, image := range images {
				kind := "refer_image"
				if image.Get("role").String() == "first_frame" || image.Get("role").String() == "last_frame" {
					kind = image.Get("role").String()
				}
				parts = append(parts, map[string]any{"type": kind, "url": nativeKlingImage(image.Get("image_url.url").String()), "id": fmt.Sprintf("image_%d", i+1)})
			}
			for _, video := range videos {
				kind := "feature_video"
				if mode == "edit" {
					kind = "base_video"
				}
				parts = append(parts, map[string]any{"type": kind, "url": video.Get("video_url.url").String(), "id": "video_1"})
			}
			payload["contents"] = parts
		} else {
			if len(images) > 0 {
				path = "/image-to-video/kling-3.0"
				if turbo {
					path += "-turbo"
				}
				parts := []map[string]any{{"type": "prompt", "text": root.Get("prompt").String()}}
				for i, image := range images {
					kind := "first_frame"
					if i == 1 {
						kind = "last_frame"
					}
					parts = append(parts, map[string]any{"type": kind, "url": nativeKlingImage(image.Get("image_url.url").String())})
				}
				payload["contents"] = parts
			} else {
				if turbo {
					path += "-turbo"
				}
				payload["prompt"] = root.Get("prompt").String()
			}
		}
	case "omni":
		if len(audios) > 0 {
			return nil, "", task, errors.New("Gemini Omni 当前原生 API 不支持上传音频参考")
		}
		path, task.Path = "/v1beta/interactions", "/v1beta/interactions/"
		parts := []map[string]any{}
		tags := []string{}
		for i, image := range images {
			parts = append(parts, nativeOmniMedia("image", image.Get("image_url.url").String()))
			if role := image.Get("role").String(); role == "first_frame" || role == "last_frame" {
				tags = append(tags, fmt.Sprintf("<%s>@Image%d", strings.ToUpper(role), i+1))
			}
		}
		for _, video := range videos {
			parts = append(parts, nativeOmniMedia("video", video.Get("video_url.url").String()))
		}
		prompt := root.Get("prompt").String()
		if len(tags) > 0 {
			prompt = "[# Sources " + strings.Join(tags, " ") + "] " + prompt
		}
		parts = append(parts, map[string]any{"type": "text", "text": prompt})
		payload = map[string]any{"model": task.Model, "input": parts, "response_format": map[string]any{"type": "video", "aspect_ratio": root.Get("aspect_ratio").String(), "resolution": strings.ToLower(root.Get("resolution").String()), "delivery": "uri"}}
		if mode == "edit" || mode == "extend" {
			payload["generation_config"] = map[string]any{"video_config": map[string]any{"task": mode}}
		}
	default:
		return nil, "", task, errors.New("unsupported native video protocol")
	}
	encoded, err := json.Marshal(payload)
	return encoded, path, task, err
}

func nativeKlingImage(value string) string {
	if _, data, ok := strings.Cut(value, ";base64,"); ok && strings.HasPrefix(value, "data:image/") {
		return data
	}
	return value
}

func nativeOmniMedia(kind, value string) map[string]any {
	if header, data, ok := strings.Cut(value, ";base64,"); ok && strings.HasPrefix(header, "data:") {
		return map[string]any{"type": kind, "data": data, "mime_type": strings.TrimPrefix(header, "data:")}
	}
	mime := "video/mp4"
	if kind == "image" {
		mime = "image/png"
	}
	if parsed, err := url.Parse(value); err == nil {
		if strings.HasSuffix(strings.ToLower(parsed.Path), ".webm") {
			mime = "video/webm"
		}
		if strings.HasSuffix(strings.ToLower(parsed.Path), ".jpg") || strings.HasSuffix(strings.ToLower(parsed.Path), ".jpeg") {
			mime = "image/jpeg"
		}
	}
	return map[string]any{"type": kind, "uri": value, "mime_type": mime}
}

func normalizeNativeVideoResponse(body []byte, task nativeVideoTask, create bool) ([]byte, nativeVideoTask, error) {
	if !gjson.ValidBytes(body) {
		return nil, task, errors.New("video provider returned invalid JSON")
	}
	root := gjson.ParseBytes(body)
	if code := root.Get("base_resp.status_code"); code.Exists() && code.Int() != 0 {
		return nil, task, fmt.Errorf("MiniMax: %s", sanitizeUpstreamErrorMessage(root.Get("base_resp.status_msg").String()))
	}
	if code := root.Get("code"); code.Exists() && code.Int() != 0 {
		return nil, task, fmt.Errorf("video provider: %s", sanitizeUpstreamErrorMessage(root.Get("message").String()))
	}
	if data := root.Get("data"); data.IsArray() {
		if len(data.Array()) == 0 {
			return nil, task, errors.New("video provider returned no matching task")
		}
		root = data.Array()[0]
	} else if data.IsObject() {
		root = data
	}
	if nested := root.Get("task"); nested.IsObject() {
		root = nested
	}
	if create {
		task.ID = firstNonEmpty(root.Get("task_id").String(), root.Get("id").String())
	}
	if task.ID == "" {
		return nil, task, errors.New("video provider returned no task id")
	}
	if !isSafeUpstreamPathSegment(task.ID) {
		return nil, task, errors.New("video provider returned an invalid task id")
	}
	status := strings.ToLower(firstNonEmpty(root.Get("status").String(), root.Get("task_status").String()))
	videoURL := firstNonEmpty(root.Get("content.url").String(), root.Get("content.video_url").String(), root.Get("task_result.videos.0.url").String())
	actualDuration := root.Get("duration")
	for _, output := range root.Get("outputs").Array() {
		if output.Get("type").String() == "video" {
			videoURL = output.Get("url").String()
			if value := output.Get("duration"); value.Exists() {
				actualDuration = value
			}
			break
		}
	}
	for _, step := range root.Get("steps").Array() {
		if step.Get("type").String() != "model_output" {
			continue
		}
		for _, part := range step.Get("content").Array() {
			if part.Get("type").String() != "video" {
				continue
			}
			videoURL = part.Get("uri").String()
			if data := part.Get("data").String(); data != "" {
				videoURL = "data:video/mp4;base64," + data
			}
		}
	}
	switch status {
	case "succeed", "succeeded", "success", "completed":
		status = "completed"
	case "failed", "error", "canceled", "cancelled", "expired":
		status = "failed"
	default:
		status = "in_progress"
	}
	result := map[string]any{"id": task.key(), "status": status}
	if task.BillingModel != "" {
		result["model"] = task.BillingModel
	}
	if videoURL != "" {
		result["video"] = map[string]any{"url": videoURL}
	}
	if fileID := root.Get("file_id").String(); fileID != "" {
		result["file_id"] = fileID
	}
	if status == "failed" {
		result["error"] = map[string]any{"message": firstNonEmpty(root.Get("error.message").String(), root.Get("error").String(), root.Get("message").String(), root.Get("task_status_msg").String(), "video generation failed")}
	}
	if actualDuration.Exists() {
		result["duration"] = actualDuration.Value()
	}
	if value := root.Get("resolution"); value.Exists() {
		result["resolution"] = value.Value()
	}
	encoded, err := json.Marshal(result)
	return encoded, task, err
}
