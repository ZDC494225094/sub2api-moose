-- Canvas requests reserve money before dispatch. This ledger is independent of Redis
-- task retention: refunds, terminal outcomes and late usage are DB-authoritative.
CREATE TABLE IF NOT EXISTS canvas_run_charges (
    id TEXT PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    subscription_id BIGINT,
    run_id VARCHAR(128) NOT NULL,
    payload_hash TEXT NOT NULL,
    mode TEXT NOT NULL,
    model TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'reserved' CHECK (status IN ('reserved', 'succeeded', 'failed')),
    reserved_cost NUMERIC(20,8) NOT NULL CHECK (reserved_cost >= 0),
    actual_cost NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (actual_cost >= 0),
    usage_recorded BOOLEAN NOT NULL DEFAULT FALSE,
    charged_cost NUMERIC(20,8) NOT NULL DEFAULT 0,
    daily_window_start TIMESTAMPTZ,
    weekly_window_start TIMESTAMPTZ,
    monthly_window_start TIMESTAMPTZ,
    lease_until TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '3 minutes',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, run_id)
);
CREATE INDEX IF NOT EXISTS idx_canvas_run_charges_recovery ON canvas_run_charges (lease_until) WHERE status = 'reserved';
