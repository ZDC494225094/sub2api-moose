-- New submissions only; no mutation of historical tasks or billing snapshots.
INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.media-gateway.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;
