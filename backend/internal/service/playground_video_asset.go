package service

import "context"

// PlaygroundVideoAssetMetadata keeps only the provider URL needed to recover a
// video after the short-lived Redis asset cache expires. Video bytes are never
// written to the database.
type PlaygroundVideoAssetMetadata struct {
	UserID     int64
	RunID      string
	AssetIndex int
	SourceURL  string
	MimeType   string
}

type PlaygroundVideoAssetRepository interface {
	Upsert(ctx context.Context, asset PlaygroundVideoAssetMetadata) error
	Get(ctx context.Context, userID int64, runID string, assetIndex int) (*PlaygroundVideoAssetMetadata, error)
}
