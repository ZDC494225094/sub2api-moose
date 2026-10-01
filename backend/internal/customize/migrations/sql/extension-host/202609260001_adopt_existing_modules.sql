-- Immutable adoption v1: only these six already-managed modules are inherited.
-- Do not expand this list for future modules: new modules default to disabled.
-- Existing administrator choices (including false) are never overwritten.

INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.premium-home.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.playground.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.infinite-canvas.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.operations-analytics.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.recharge-campaigns.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.customer-support.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;
