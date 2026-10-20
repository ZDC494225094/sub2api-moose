CREATE TABLE IF NOT EXISTS object_storage_assets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT,
    bucket TEXT NOT NULL,
    object_key TEXT NOT NULL,
    content_type TEXT NOT NULL,
    bytes BIGINT NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(bucket, object_key)
);
CREATE INDEX IF NOT EXISTS idx_object_storage_assets_uploaded ON object_storage_assets(uploaded_at DESC, id DESC);
