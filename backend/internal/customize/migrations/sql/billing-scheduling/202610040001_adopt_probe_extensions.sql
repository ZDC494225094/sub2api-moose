-- Only legacy fork deployments adopt probe extensions. Preserve all explicit values.
INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.billing-scheduling.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;
