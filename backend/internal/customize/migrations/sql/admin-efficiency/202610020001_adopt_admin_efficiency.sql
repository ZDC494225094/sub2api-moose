-- Admit existing legacy installations once; native installations default off.
-- Explicit administrator choices and all account/user/group data remain untouched.
INSERT INTO settings (key, value, updated_at)
SELECT 'custom_extensions.admin-efficiency.enabled',
       CASE installation_kind WHEN 'legacy' THEN 'true' ELSE 'false' END,
       CURRENT_TIMESTAMP
FROM custom_extension_installation WHERE id = 1
ON CONFLICT (key) DO NOTHING;
