-- Independent marketing adoption; do not modify extension-host adoption v1.
-- Preserve explicit administrator choices, including false or invalid values.
-- Historical fork installs retain existing business; native installs opt in.
INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.marketing-tools.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;
