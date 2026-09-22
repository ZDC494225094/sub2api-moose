CREATE TABLE IF NOT EXISTS recharge_campaigns (
    id BIGSERIAL PRIMARY KEY,
    config JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Historical bindings intentionally stay NULL: they are not new campaign invitations.
ALTER TABLE user_affiliates ADD COLUMN IF NOT EXISTS invited_at TIMESTAMPTZ;
