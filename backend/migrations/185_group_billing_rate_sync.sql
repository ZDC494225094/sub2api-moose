-- Allow an OpenAI API Key account's scheduled upstream billing probe to drive
-- the group's base rate, with an additive per-group markup.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS billing_rate_sync_account_id BIGINT,
    ADD COLUMN IF NOT EXISTS billing_rate_markup DECIMAL(10,4) NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_billing_rate_sync_account_id_fkey'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_billing_rate_sync_account_id_fkey
            FOREIGN KEY (billing_rate_sync_account_id)
            REFERENCES accounts(id)
            ON DELETE SET NULL;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'groups_billing_rate_markup_non_negative'
    ) THEN
        ALTER TABLE groups
            ADD CONSTRAINT groups_billing_rate_markup_non_negative
            CHECK (billing_rate_markup >= 0);
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_groups_billing_rate_sync_account_id
    ON groups (billing_rate_sync_account_id)
    WHERE billing_rate_sync_account_id IS NOT NULL AND deleted_at IS NULL;

-- Group rate changes also affect API-key auth snapshots. Migration 184 created
-- this function; extend its no-op guard so direct probe-driven rate updates
-- enter the durable invalidation outbox as well.
CREATE OR REPLACE FUNCTION enqueue_group_auth_cache_invalidation()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    target_group_id BIGINT;
BEGIN
    target_group_id := OLD.id;
    IF TG_OP = 'UPDATE'
       AND OLD.status IS NOT DISTINCT FROM NEW.status
       AND OLD.is_exclusive IS NOT DISTINCT FROM NEW.is_exclusive
       AND OLD.rate_multiplier IS NOT DISTINCT FROM NEW.rate_multiplier
       AND OLD.deleted_at IS NOT DISTINCT FROM NEW.deleted_at THEN
        RETURN NEW;
    END IF;

    INSERT INTO auth_cache_invalidation_outbox (cache_key)
    SELECT encode(sha256(convert_to(k.key, 'UTF8')), 'hex')
    FROM api_keys AS k
    WHERE k.group_id = target_group_id
      AND k.deleted_at IS NULL
      AND k.key <> '';
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;
