-- Existing grants and unredeemed codes keep historical independent semantics.
-- No global user/group uniqueness constraint: historical multiple grants remain valid.
ALTER TABLE user_subscriptions ADD COLUMN custom_subscription_policy VARCHAR(32) NOT NULL DEFAULT 'independent-v1';
ALTER TABLE redeem_codes ADD COLUMN custom_subscription_policy VARCHAR(32) NOT NULL DEFAULT 'independent-v1';
CREATE INDEX idx_subscription_native_owner ON user_subscriptions (user_id, group_id, custom_subscription_policy);
INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.subscription-extensions.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;
