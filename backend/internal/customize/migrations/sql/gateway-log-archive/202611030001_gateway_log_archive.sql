CREATE TABLE gateway_log_archive_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    config TEXT NOT NULL DEFAULT '',
    access_key_id TEXT NOT NULL DEFAULT '',
    secret_access_key TEXT NOT NULL DEFAULT '',
    session_token TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO gateway_log_archive_config (id) VALUES (1);

-- One row per ZIP. Rows are node-owned: the node_id is stored in the data
-- directory so only the server holding the local file appends or uploads it.
CREATE TABLE gateway_log_archive_bundles (
    id BIGSERIAL PRIMARY KEY,
    node_id VARCHAR(64) NOT NULL,
    day VARCHAR(10) NOT NULL,
    model_key VARCHAR(128) NOT NULL,
    part INTEGER NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    local_path TEXT NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'open' CHECK (status IN ('open','sealed','uploading','uploaded','failed')),
    size_bytes BIGINT NOT NULL DEFAULT 0,
    entry_count INTEGER NOT NULL DEFAULT 0,
    bucket VARCHAR(255) NOT NULL DEFAULT '',
    object_key TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ,
    local_deleted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sealed_at TIMESTAMPTZ,
    uploaded_at TIMESTAMPTZ,
    UNIQUE (day, model_key, part)
);
CREATE INDEX gateway_log_archive_bundles_queue_idx ON gateway_log_archive_bundles (node_id, status, id);
CREATE INDEX gateway_log_archive_bundles_day_idx ON gateway_log_archive_bundles (day, model_key, part);

-- One row per session document inside a ZIP: the admin index and the data
-- needed to rebuild an unsealed ZIP directory after a restart.
CREATE TABLE gateway_log_archive_entries (
    id BIGSERIAL PRIMARY KEY,
    bundle_id BIGINT NOT NULL REFERENCES gateway_log_archive_bundles(id) ON DELETE CASCADE,
    seq INTEGER NOT NULL,
    entry_name VARCHAR(255) NOT NULL,
    session_id VARCHAR(255) NOT NULL DEFAULT '',
    model VARCHAR(255) NOT NULL DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT '',
    user_id BIGINT NOT NULL DEFAULT 0,
    api_key_id BIGINT NOT NULL DEFAULT 0,
    exchange_count INTEGER NOT NULL DEFAULT 0,
    header_offset BIGINT NOT NULL,
    compressed_size BIGINT NOT NULL,
    uncompressed_size BIGINT NOT NULL,
    crc32 BIGINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    ended_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (bundle_id, seq)
);
CREATE INDEX gateway_log_archive_entries_session_idx ON gateway_log_archive_entries (session_id);
CREATE INDEX gateway_log_archive_entries_user_idx ON gateway_log_archive_entries (user_id, id);

-- Capturing request/response bodies is opt-in.
INSERT INTO settings (key, value, updated_at) VALUES
('custom_extensions.gateway-log-archive.enabled','false',CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
