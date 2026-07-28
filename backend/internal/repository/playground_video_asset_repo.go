package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type playgroundVideoAssetRepository struct {
	db *sql.DB
}

func NewPlaygroundVideoAssetRepository(db *sql.DB) service.PlaygroundVideoAssetRepository {
	return &playgroundVideoAssetRepository{db: db}
}

func (r *playgroundVideoAssetRepository) Upsert(ctx context.Context, asset service.PlaygroundVideoAssetMetadata) error {
	_, err := r.db.ExecContext(ctx, `
INSERT INTO playground_video_assets (user_id, run_id, asset_index, source_url, mime_type)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, run_id, asset_index) DO UPDATE SET
    source_url = EXCLUDED.source_url,
    mime_type = EXCLUDED.mime_type,
    updated_at = CURRENT_TIMESTAMP`,
		asset.UserID, asset.RunID, asset.AssetIndex, asset.SourceURL, asset.MimeType,
	)
	return err
}

func (r *playgroundVideoAssetRepository) Get(ctx context.Context, userID int64, runID string, assetIndex int) (*service.PlaygroundVideoAssetMetadata, error) {
	asset := &service.PlaygroundVideoAssetMetadata{}
	err := r.db.QueryRowContext(ctx, `
SELECT user_id, run_id, asset_index, source_url, mime_type
FROM playground_video_assets
WHERE user_id = $1 AND run_id = $2 AND asset_index = $3`, userID, runID, assetIndex).Scan(
		&asset.UserID, &asset.RunID, &asset.AssetIndex, &asset.SourceURL, &asset.MimeType,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return asset, nil
}
