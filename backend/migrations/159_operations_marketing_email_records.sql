CREATE TABLE IF NOT EXISTS operations_marketing_email_records (
    id BIGSERIAL PRIMARY KEY,
    subject VARCHAR(120) NOT NULL,
    body_format VARCHAR(16) NOT NULL DEFAULT 'plain',
    body_preview VARCHAR(500) NOT NULL DEFAULT '',
    body_hash VARCHAR(64) NOT NULL DEFAULT '',
    audience VARCHAR(32) NOT NULL DEFAULT 'all',
    status_filter VARCHAR(32) NOT NULL DEFAULT 'active',
    active_days INTEGER NOT NULL DEFAULT 30,
    min_balance NUMERIC(20,8),
    max_balance NUMERIC(20,8),
    min_total_recharged NUMERIC(20,8),
    max_total_recharged NUMERIC(20,8),
    selected_user_count INTEGER NOT NULL DEFAULT 0,
    total_matched INTEGER NOT NULL DEFAULT 0,
    targeted INTEGER NOT NULL DEFAULT 0,
    sent INTEGER NOT NULL DEFAULT 0,
    failed INTEGER NOT NULL DEFAULT 0,
    skipped_invalid INTEGER NOT NULL DEFAULT 0,
    errors JSONB NOT NULL DEFAULT '[]'::jsonb,
    sample JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_operations_marketing_email_records_created_at
    ON operations_marketing_email_records(created_at DESC);
