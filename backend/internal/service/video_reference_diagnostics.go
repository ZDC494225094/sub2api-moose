package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// Count the actual wire payload without recording signed URLs, prompts or media.
func logVideoReferencePayload(ctx context.Context, stage string, accountID int64, body []byte) {
	charge := CanvasChargeFromContext(ctx)
	if charge == nil || !gjson.ValidBytes(body) {
		return
	}
	parts := gjson.GetBytes(body, "content").Array()
	if len(parts) == 0 {
		parts = gjson.GetBytes(body, "contents").Array()
	}
	if len(parts) == 0 {
		parts = gjson.GetBytes(body, "input").Array()
	}
	images, videos, audios := 0, 0, 0
	for _, part := range parts {
		switch part.Get("type").String() {
		case "image_url", "image", "first_frame", "last_frame", "refer_image":
			images++
		case "video_url", "video", "feature_video", "base_video":
			videos++
		case "audio_url", "audio":
			audios++
		}
	}
	if len(parts) == 0 {
		for _, field := range []string{"image", "image_tail", "first_frame_image", "last_frame_image"} {
			if gjson.GetBytes(body, field).String() != "" {
				images++
			}
		}
	}
	logger.L().Info("playground.video_reference_payload",
		zap.String("run_id", charge.RunID), zap.String("stage", stage), zap.Int64("account_id", accountID),
		zap.String("model", firstNonEmpty(gjson.GetBytes(body, "model").String(), gjson.GetBytes(body, "model_name").String())), zap.Int("content_parts", len(parts)),
		zap.Int("content_images", images), zap.Int("content_videos", videos), zap.Int("content_audios", audios),
		zap.Int("reference_videos", len(gjson.GetBytes(body, "reference_videos").Array())),
		zap.Int("reference_audios", len(gjson.GetBytes(body, "reference_audios").Array())))
}
