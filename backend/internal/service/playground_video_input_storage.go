package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// Resolve storage per job and replace inline references before building the
// upstream payload. Keep caller slices untouched, including on partial failure.
func (s *PlaygroundRunService) prepareVideoInputImages(ctx context.Context, request *PlaygroundRunRequest) error {
	if s.imageStorageResolver == nil && s.videoStorageResolver == nil {
		return nil
	}
	images := append([]PlaygroundRunImageInput(nil), request.Images...)
	videos := append([]PlaygroundRunVideoInput(nil), request.ReferenceVideos...)
	audios := append([]PlaygroundRunVideoInput(nil), request.ReferenceAudios...)
	var video *PlaygroundRunVideoInput
	if request.ReferenceVideo != nil {
		copy := *request.ReferenceVideo
		video = &copy
	}
	type reference struct {
		value *string
		kind  string
	}
	refs := make([]reference, 0, len(images)+len(videos)+len(audios)+1)
	for i := range images {
		refs = append(refs, reference{&images[i].DataURL, "图片"})
	}
	for i := range videos {
		refs = append(refs, reference{&videos[i].DataURL, "视频"})
	}
	for i := range audios {
		refs = append(refs, reference{&audios[i].DataURL, "音频"})
	}
	if video != nil {
		refs = append(refs, reference{&video.DataURL, "视频"})
	}
	hasInline := false
	for _, ref := range refs {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(*ref.value)), "data:") {
			hasInline = true
			break
		}
	}
	if !hasInline {
		logger.L().Info("playground.video_reference_storage_no_inline", zap.String("run_id", request.ID), zap.Int("references", len(refs)))
		return nil
	}
	var uploader *ImageResultUploader
	var enabled bool
	if s.videoStorageResolver != nil {
		var err error
		uploader, enabled, err = s.videoStorageResolver()
		if err != nil {
			return fmt.Errorf("参考素材对象存储配置异常：%w", err)
		}
	} else {
		uploader, enabled = s.imageStorageResolver()
	}
	if !enabled {
		logger.L().Info("playground.video_reference_storage_disabled", zap.String("run_id", request.ID), zap.Int("references", len(refs)))
		return nil
	}
	if uploader == nil || uploader.storage == nil {
		return fmt.Errorf("参考素材对象存储不可用，请检查异步生图对象存储配置")
	}
	objectID := "video-inputs/" + uuid.NewString()
	logger.L().Info("playground.video_reference_storage_started", zap.String("run_id", request.ID), zap.Int("references", len(refs)))
	uploaded := make(map[string]string)
	for i, ref := range refs {
		raw := strings.TrimSpace(*ref.value)
		if !strings.HasPrefix(strings.ToLower(raw), "data:") {
			continue
		}
		if cached, ok := uploaded[raw]; ok {
			*ref.value = cached
			continue
		}
		data, contentType, err := decodeVideoReferenceData(uploader, raw, ref.kind)
		if err != nil {
			return fmt.Errorf("参考%s %d 校验失败: %w", ref.kind, i+1, err)
		}
		uploadCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		// Retry transient failures only; a denied bucket or invalid signature
		// cannot be repaired by resubmitting the same bytes and credentials.
		var imageURL string
		var saveErr error
		var failure videoStorageFailure
		attempts := 0
		for attempt := 0; attempt < 3; attempt++ {
			if err := uploadCtx.Err(); err != nil {
				saveErr = err
				failure = classifyVideoStorageFailure(err)
				break
			}
			if attempt > 0 {
				timer := time.NewTimer(time.Duration(attempt) * 2 * time.Second)
				select {
				case <-uploadCtx.Done():
					timer.Stop()
					cancel()
					return fmt.Errorf("参考%s %d 转存对象存储失败：%s", ref.kind, i+1, classifyVideoStorageFailure(uploadCtx.Err()).hint)
				case <-timer.C:
				}
			}
			attempts++
			imageURL, saveErr = uploader.storage.Save(uploadCtx, uploader.prefix+objectID+fmt.Sprintf("-%d", i)+videoReferenceExtension(contentType), contentType, data)
			if saveErr == nil {
				break
			}
			failure = classifyVideoStorageFailure(saveErr)
			if !failure.retry {
				break
			}
		}
		cancel()
		if saveErr != nil {
			// Never log the original SDK error: it may contain credentials,
			// signed URLs or a provider-controlled error message.
			logger.L().Warn("playground.video_reference_storage_failed",
				zap.String("run_id", request.ID),
				zap.Int("image_index", i+1), zap.Int("attempts", attempts),
				zap.String("category", failure.category), zap.Int("http_status", failure.status))
			return fmt.Errorf("参考%s %d 转存对象存储失败：%s", ref.kind, i+1, failure.hint)
		}
		parsed, err := url.Parse(imageURL)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("参考%s %d 的对象存储地址不是有效的 HTTP(S) URL", ref.kind, i+1)
		}
		uploaded[raw] = imageURL
		*ref.value = imageURL
		logger.L().Info("playground.video_reference_storage_saved", zap.String("run_id", request.ID), zap.Int("reference_index", i+1), zap.String("content_type", contentType), zap.Int("bytes", len(data)))
	}
	request.Images = images
	request.ReferenceVideos = videos
	request.ReferenceAudios = audios
	request.ReferenceVideo = video
	return nil
}

func decodeVideoReferenceData(uploader *ImageResultUploader, raw, kind string) ([]byte, string, error) {
	if kind == "图片" {
		data, contentType, err := uploader.decodeImageDataURL(raw)
		if err != nil {
			return nil, "", err
		}
		if detectedImageContentType(data) == "" {
			return nil, "", fmt.Errorf("不是有效的图片内容")
		}
		return data, contentType, nil
	}
	header, payload, ok := strings.Cut(raw, ",")
	if !ok || !strings.HasSuffix(strings.ToLower(header), ";base64") {
		return nil, "", fmt.Errorf("素材必须为 base64 data URL")
	}
	contentType, _, err := mime.ParseMediaType(header[len("data:") : len(header)-len(";base64")])
	prefix, limit := "video/", playgroundVideoMaxBytes
	if kind == "音频" {
		prefix, limit = "audio/", playgroundAudioMaxBytes
	}
	if err != nil || !strings.HasPrefix(strings.ToLower(contentType), prefix) {
		return nil, "", fmt.Errorf("素材 MIME 类型不匹配")
	}
	if base64.StdEncoding.DecodedLen(len(payload)) > limit+2 {
		return nil, "", fmt.Errorf("素材超过大小上限")
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(data) == 0 || len(data) > limit {
		return nil, "", fmt.Errorf("素材为空、base64 无效或超过大小上限")
	}
	return data, strings.ToLower(contentType), nil
}

func videoReferenceExtension(contentType string) string {
	switch contentType {
	case "video/mp4", "audio/mp4":
		return ".mp4"
	case "video/webm", "audio/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "audio/mpeg":
		return ".mp3"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/ogg", "video/ogg":
		return ".ogg"
	case "audio/flac":
		return ".flac"
	}
	if strings.HasPrefix(contentType, "image/") {
		return extensionForContentType(contentType)
	}
	return ".bin"
}
