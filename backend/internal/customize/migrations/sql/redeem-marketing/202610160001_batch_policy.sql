-- Independent immutable policy; no changes to upstream Ent schemas.
CREATE TABLE IF NOT EXISTS custom_redeem_marketing_batches (
    batch_id VARCHAR(64) PRIMARY KEY,
    registered_after TIMESTAMPTZ NOT NULL
);

INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.redeem-marketing.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;
