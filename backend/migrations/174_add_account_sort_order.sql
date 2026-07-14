-- Add a display-only sort order for the admin account list.
-- This is intentionally separate from accounts.priority, which affects scheduling.
ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS sort_order BIGINT NOT NULL DEFAULT 0;

-- Preserve the existing stable order for accounts created before this migration.
UPDATE accounts
SET sort_order = id
WHERE sort_order = 0;

CREATE INDEX IF NOT EXISTS idx_accounts_sort_order
    ON accounts(sort_order, id)
    WHERE deleted_at IS NULL;
