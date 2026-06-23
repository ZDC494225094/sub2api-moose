-- Allow users to own multiple independent subscriptions for the same group and
-- let API keys carry multiple candidate groups plus a billing preference.

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS billing_priority VARCHAR(32) NOT NULL DEFAULT 'balance_first';

UPDATE api_keys
SET group_ids = CASE
    WHEN group_id IS NULL THEN '[]'::jsonb
    ELSE jsonb_build_array(group_id)
END
WHERE group_ids IS NULL OR group_ids = '[]'::jsonb;

ALTER TABLE user_subscriptions
    DROP CONSTRAINT IF EXISTS user_subscriptions_user_id_group_id_key;

DROP INDEX IF EXISTS user_subscriptions_user_id_group_id_key;
DROP INDEX IF EXISTS usersubscription_user_id_group_id;
DROP INDEX IF EXISTS user_subscriptions_user_group_unique_active;

CREATE INDEX IF NOT EXISTS user_subscriptions_user_group_active_lookup
    ON user_subscriptions(user_id, group_id, status, expires_at, created_at)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS api_keys_group_ids_gin
    ON api_keys USING GIN (group_ids);

CREATE INDEX IF NOT EXISTS api_keys_billing_priority
    ON api_keys(billing_priority)
    WHERE deleted_at IS NULL;
