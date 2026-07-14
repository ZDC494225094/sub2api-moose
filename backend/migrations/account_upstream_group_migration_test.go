package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration175AddsExplicitAccountUpstreamGroupWithoutInferringMembership(t *testing.T) {
	content, err := FS.ReadFile("175_add_account_upstream_group.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS upstream_group VARCHAR(100) NOT NULL DEFAULT ''")
	require.NotContains(t, sql, "base_url")
	require.NotContains(t, sql, "platform")
	require.NotContains(t, sql, "UPDATE accounts")
}

func TestMigration175AddsNormalizedUpstreamGroupOrderingIndex(t *testing.T) {
	content, err := FS.ReadFile("175_add_account_upstream_group.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_accounts_upstream_group_normalized")
	require.Contains(t, sql, "ON accounts ((LOWER(BTRIM(upstream_group))), sort_order, id)")
	require.Contains(t, sql, "WHERE deleted_at IS NULL")
}

func TestMigration176ClearsValuesCreatedByTheUnreleasedInferenceDraft(t *testing.T) {
	content, err := FS.ReadFile("176_clear_inferred_account_upstream_groups.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "FROM schema_migrations")
	require.Contains(t, sql, "WHERE filename = '175_add_account_upstream_group.sql'")
	require.Contains(t, sql, "UPDATE accounts")
	require.Contains(t, sql, "SET upstream_group = ''")
	require.Contains(t, sql, "account.upstream_group <> ''")
	require.Contains(t, sql, "account.updated_at < migration_175.applied_at")
	require.NotContains(t, sql, "base_url")
	require.NotContains(t, sql, "platform")
}

func TestMigration177CreatesDirectoryFromExplicitUpstreamGroupsOnly(t *testing.T) {
	content, err := FS.ReadFile("177_create_account_upstream_group_directory.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS account_upstream_groups")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS upstream_group_id BIGINT NULL")
	require.Contains(t, sql, "FROM accounts")
	require.Contains(t, sql, "BTRIM(COALESCE(upstream_group, '')) <> ''")
	require.Contains(t, sql, "LOWER(BTRIM(account.upstream_group)) = directory.normalized_name")
	withoutComments := removeSQLLineComments(sql)
	require.NotContains(t, withoutComments, "base_url")
	require.NotContains(t, withoutComments, "platform")
}

func removeSQLLineComments(sql string) string {
	lines := strings.Split(sql, "\n")
	statements := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			statements = append(statements, line)
		}
	}
	return strings.Join(statements, "\n")
}
