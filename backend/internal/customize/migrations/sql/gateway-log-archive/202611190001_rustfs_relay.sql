ALTER TABLE gateway_log_archive_buckets ADD COLUMN storage_role TEXT NOT NULL DEFAULT 'destination' CHECK (storage_role IN ('destination','rustfs'));
ALTER TABLE gateway_log_archive_buckets ADD COLUMN privacy_mode TEXT NOT NULL DEFAULT 'inherit' CHECK (privacy_mode IN ('inherit','redact','original'));
CREATE UNIQUE INDEX gateway_log_archive_one_primary ON gateway_log_archive_buckets (storage_role) WHERE enabled AND storage_role = 'rustfs';

ALTER TABLE gateway_log_archive_bundles ADD COLUMN stage_bucket_id BIGINT REFERENCES gateway_log_archive_buckets(id);
ALTER TABLE gateway_log_archive_bundles ADD COLUMN stage_prefix TEXT NOT NULL DEFAULT '';
ALTER TABLE gateway_log_archive_bundles ADD COLUMN stage_redacted BOOLEAN NOT NULL DEFAULT false;
CREATE UNIQUE INDEX gateway_log_archive_primary_key ON gateway_log_archive_bundles (stage_bucket_id,object_key) WHERE stage_bucket_id IS NOT NULL AND object_key <> '';
ALTER TABLE gateway_log_archive_entries ADD COLUMN staged_remote BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE gateway_log_archive_entries ADD COLUMN stage_key TEXT NOT NULL DEFAULT '';
ALTER TABLE gateway_log_archive_entries ADD COLUMN stage_event TEXT NOT NULL DEFAULT '';
ALTER TABLE gateway_log_archive_entries ADD COLUMN stage_cleaned BOOLEAN NOT NULL DEFAULT false;
CREATE UNIQUE INDEX gateway_log_archive_stage_event ON gateway_log_archive_entries (stage_event) WHERE stage_event <> '';

CREATE TABLE gateway_log_archive_deliveries (
    bundle_id BIGINT NOT NULL REFERENCES gateway_log_archive_bundles(id) ON DELETE CASCADE,
    source_bucket_id BIGINT NOT NULL REFERENCES gateway_log_archive_buckets(id),
    bucket_id BIGINT NOT NULL REFERENCES gateway_log_archive_buckets(id),
    object_key TEXT NOT NULL,
    key_reserved BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (bundle_id, bucket_id)
);
CREATE INDEX gateway_log_archive_delivery_queue ON gateway_log_archive_deliveries (status, next_attempt_at, bundle_id);
CREATE UNIQUE INDEX gateway_log_archive_delivery_key ON gateway_log_archive_deliveries (bucket_id,object_key) WHERE key_reserved;
