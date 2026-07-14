-- Migration 175 briefly inferred group labels before this field was released.
-- Clear only values on accounts that have not been edited since migration 175;
-- groups assigned through the application after that migration are preserved.
WITH migration_175 AS (
    SELECT applied_at
    FROM schema_migrations
    WHERE filename = '175_add_account_upstream_group.sql'
)
UPDATE accounts AS account
SET upstream_group = ''
FROM migration_175
WHERE account.upstream_group <> ''
  AND account.updated_at < migration_175.applied_at;
