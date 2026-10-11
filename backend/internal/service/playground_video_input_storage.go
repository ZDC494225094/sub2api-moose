package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"go.uber.org/zap"
)

// prepareVideoInputImages offloads inline references only for URL-based video
// protocols. Resolve per job so changes to the existing storage switch apply
// without restarting; disabled storage retains the previous inline behavior.
func (s *PlaygroundRunService) prepareVideoInputImages(ctx context.Context, request *PlaygroundRunRequest) error {
	if isGeminiPlaygroundVideoRequest(*request) || s.imageStorageResolver == nil {
		return nil
	}
	hasInline := false
	for _, image := range request.Images {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(image.DataURL)), "data:") {
			hasInline = true
			break
		}
	}
	if !hasInline {
		return nil
	}
	uploader, enabled := s.imageStorageResolver()
	if !enabled {
		return nil
	}
	if uploader == nil || uploader.storage == nil {
		return fmt.Errorf("视频参考图对象存储不可用，请检查异步生图对象存储配置")
	}
	// Do not mutate the caller's images or use a caller-controlled run ID in S3 keys.
	images := append([]PlaygroundRunImageInput(nil), request.Images...)
	objectID := "video-inputs/" + uuid.NewString()
	uploaded := make(map[string]string)
	for i := range images {
		raw := strings.TrimSpace(images[i].DataURL)
		if !strings.HasPrefix(strings.ToLower(raw), "data:") {
			continue
		}
		if cached, ok := uploaded[raw]; ok {
			images[i].DataURL = cached
			continue
		}
		data, contentType, err := uploader.decodeImageDataURL(raw)
		if err != nil {
			return fmt.Errorf("视频参考图 %d 校验失败: %w", i+1, err)
		}
		if detectedImageContentType(data) == "" {
			return fmt.Errorf("视频参考图 %d 不是有效的图片内容", i+1)
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
					return fmt.Errorf("视频参考图 %d 转存对象存储失败：%s", i+1, classifyVideoStorageFailure(uploadCtx.Err()).hint)
				case <-timer.C:
				}
			}
			attempts++
			imageURL, saveErr = uploader.storage.Save(uploadCtx, uploader.buildKey(objectID, i, contentType), contentType, data)
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
				zap.Int("image_index", i+1), zap.Int("attempts", attempts),
				zap.String("category", failure.category), zap.Int("http_status", failure.status))
			return fmt.Errorf("视频参考图 %d 转存对象存储失败：%s", i+1, failure.hint)
		}
		parsed, err := url.Parse(imageURL)
		if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return fmt.Errorf("视频参考图 %d 的对象存储地址不是有效的 HTTP(S) URL", i+1)
		}
		uploaded[raw] = imageURL
		images[i].DataURL = imageURL
	}
	request.Images = images
	return nil
}
