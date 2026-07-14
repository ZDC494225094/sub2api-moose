-- Add an explicit upstream grouping label. Group membership is intentionally
-- left unassigned until an administrator chooses it.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS upstream_group VARCHAR(100) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_accounts_upstream_group_normalized
    ON accounts ((LOWER(BTRIM(upstream_group))), sort_order, id)
    WHERE deleted_at IS NULL;
