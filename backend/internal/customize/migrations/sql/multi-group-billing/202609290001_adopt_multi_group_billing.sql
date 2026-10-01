-- Independent multi-group key admission adoption; do not modify earlier files.
-- Only new key configuration reads this switch; every existing key keeps the
-- routing ownership recorded by 202609260003 regardless of the value below.
-- Preserve explicit administrator choices, including false or invalid values.
INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.multi-group-billing.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;
