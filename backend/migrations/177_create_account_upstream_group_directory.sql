-- Persist administrator-defined upstream provider groups independently from
-- account Base URL and platform settings.
CREATE TABLE IF NOT EXISTS account_upstream_groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    normalized_name VARCHAR(100) NOT NULL,
    sort_order BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_account_upstream_groups_normalized_name UNIQUE (normalized_name),
    CONSTRAINT chk_account_upstream_groups_name_nonempty CHECK (BTRIM(name) <> '')
);

ALTER TABLE accounts
    ADD COLUMN IF NOT EXISTS upstream_group_id BIGINT NULL
    REFERENCES account_upstream_groups(id) ON DELETE RESTRICT;

-- Backfill only the explicit upstream_group labels already stored on accounts.
-- The label is never inferred from Base URL or platform.
WITH variants AS (
    SELECT
        LOWER(BTRIM(upstream_group)) AS normalized_name,
        BTRIM(upstream_group) AS name,
        COUNT(*) AS variant_count
    FROM accounts
    WHERE BTRIM(COALESCE(upstream_group, '')) <> ''
    GROUP BY LOWER(BTRIM(upstream_group)), BTRIM(upstream_group)
), ranked_variants AS (
    SELECT
        normalized_name,
        name,
        ROW_NUMBER() OVER (
            PARTITION BY normalized_name
            ORDER BY variant_count DESC, name ASC
        ) AS variant_rank
    FROM variants
), canonical_groups AS (
    SELECT normalized_name, name
    FROM ranked_variants
    WHERE variant_rank = 1
)
INSERT INTO account_upstream_groups (name, normalized_name, sort_order)
SELECT
    name,
    normalized_name,
    ROW_NUMBER() OVER (ORDER BY normalized_name) * 10
FROM canonical_groups
ON CONFLICT (normalized_name) DO NOTHING;

UPDATE accounts AS account
SET
    upstream_group_id = directory.id,
    upstream_group = directory.name
FROM account_upstream_groups AS directory
WHERE BTRIM(COALESCE(account.upstream_group, '')) <> ''
  AND LOWER(BTRIM(account.upstream_group)) = directory.normalized_name;

CREATE INDEX IF NOT EXISTS idx_account_upstream_groups_sort_order
    ON account_upstream_groups(sort_order, id);

CREATE INDEX IF NOT EXISTS idx_accounts_upstream_group_id_sort_order
    ON accounts(upstream_group_id, sort_order, id)
    WHERE deleted_at IS NULL;
