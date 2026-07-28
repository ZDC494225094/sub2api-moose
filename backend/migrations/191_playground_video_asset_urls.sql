CREATE TABLE IF NOT EXISTS playground_video_assets (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    run_id VARCHAR(128) NOT NULL,
    asset_index INTEGER NOT NULL CHECK (asset_index >= 0),
    source_url TEXT NOT NULL,
    mime_type VARCHAR(128) NOT NULL DEFAULT 'video/mp4',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, run_id, asset_index)
);

COMMENT ON TABLE playground_video_assets IS 'Playground video provider URL metadata; video bytes are not stored in PostgreSQL';
COMMENT ON COLUMN playground_video_assets.source_url IS 'Private provider URL used only by the backend for cache recovery';
