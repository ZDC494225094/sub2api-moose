package service

import (
	"context"
	"errors"
	"time"
)

type storageUserContextKey struct{}

func WithStorageUser(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, storageUserContextKey{}, userID)
}

func StorageUser(ctx context.Context) int64 {
	id, _ := ctx.Value(storageUserContextKey{}).(int64)
	return id
}

type ObjectStorageAsset struct {
	ID          int64     `json:"id"`
	UserID      *int64    `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	Bucket      string    `json:"bucket"`
	ObjectKey   string    `json:"object_key"`
	ContentType string    `json:"content_type"`
	Bytes       int64     `json:"bytes"`
	UploadedAt  time.Time `json:"uploaded_at"`
	URL         string    `json:"url"`
}

type ObjectStorageAssetPage struct {
	Items []ObjectStorageAsset `json:"items"`
	Total int64                `json:"total"`
}

type objectStorageAssetLister interface {
	ListAssets(context.Context, int, int, string) (*ObjectStorageAssetPage, error)
}

func (s *ImageStorageSettingService) ListAssets(ctx context.Context, page, pageSize int, search string) (*ObjectStorageAssetPage, error) {
	cfg, err := s.effectiveConfig(ctx)
	if err != nil {
		return nil, err
	}
	storage, err := s.factory(ctx, cfg)
	if err != nil {
		return nil, err
	}
	lister, ok := storage.(objectStorageAssetLister)
	if !ok {
		return nil, errors.New("object storage asset listing is unavailable")
	}
	return lister.ListAssets(ctx, page, pageSize, search)
}
