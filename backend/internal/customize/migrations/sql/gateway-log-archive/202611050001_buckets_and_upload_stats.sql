-- Several storage buckets: uploads go to the enabled bucket with the lowest
-- priority value and fall over to the next one. Credentials are encrypted by
-- the host SecretEncryptor, like the single-bucket columns they replace.
CREATE TABLE gateway_log_archive_buckets (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(64) NOT NULL DEFAULT '',
    provider VARCHAR(32) NOT NULL DEFAULT 'aws',
    bucket VARCHAR(255) NOT NULL,
    region VARCHAR(64) NOT NULL DEFAULT '',
    endpoint TEXT NOT NULL DEFAULT '',
    prefix TEXT NOT NULL DEFAULT '',
    force_path_style BOOLEAN NOT NULL DEFAULT false,
    access_key_id TEXT NOT NULL DEFAULT '',
    secret_access_key TEXT NOT NULL DEFAULT '',
    session_token TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX gateway_log_archive_buckets_order_idx ON gateway_log_archive_buckets (enabled, priority, id);

-- Which bucket configuration received the bundle (NULL until uploaded, or for
-- bundles uploaded before buckets existed; those are matched by bucket name).
ALTER TABLE gateway_log_archive_bundles ADD COLUMN bucket_id BIGINT;

-- Cumulative uploaded totals per bucket and model. Bundle index rows are pruned
-- after the retention window; these counters are not, so totals stay complete.
-- bucket_id 0 holds uploads made before buckets existed until they are adopted.
CREATE TABLE gateway_log_archive_upload_stats (
    bucket_id BIGINT NOT NULL DEFAULT 0,
    model_key VARCHAR(128) NOT NULL,
    bundle_count BIGINT NOT NULL DEFAULT 0,
    entry_count BIGINT NOT NULL DEFAULT 0,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    last_uploaded_at TIMESTAMPTZ,
    PRIMARY KEY (bucket_id, model_key)
);

INSERT INTO gateway_log_archive_upload_stats (bucket_id, model_key, bundle_count, entry_count, size_bytes, last_uploaded_at)
SELECT 0, model_key, COUNT(*), COALESCE(SUM(entry_count), 0), COALESCE(SUM(size_bytes), 0), MAX(uploaded_at)
FROM gateway_log_archive_bundles
WHERE status = 'uploaded'
GROUP BY model_key;
