package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
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
		// Retry the upload up to 3 times with backoff. Transient network
		// issues or brief R2/S3 unavailability should not fail the whole
		// video task. Root cause of widespread "转存对象存储失败": single
		// attempt with no retry, so any transient error aborts the task.
		var imageURL string
		var saveErr error
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				select {
				case <-ctx.Done():
					cancel()
					return ctx.Err()
				case <-time.After(time.Duration(attempt) * 2 * time.Second):
				}
			}
			imageURL, saveErr = uploader.storage.Save(uploadCtx, uploader.buildKey(objectID, i, contentType), contentType, data)
			if saveErr == nil {
				break
			}
		}
		cancel()
		if saveErr != nil {
			// Do not expose storage credentials or signed URLs through task errors.
			return fmt.Errorf("视频参考图 %d 转存对象存储失败，请检查存储配置及连接", i+1)
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
