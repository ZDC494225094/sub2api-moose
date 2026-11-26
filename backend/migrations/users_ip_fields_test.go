package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUsersIPFieldsMigration(t *testing.T) {
	content, err := FS.ReadFile("254_add_users_ip_fields.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ALTER TABLE users ADD COLUMN IF NOT EXISTS registration_ip VARCHAR(45)")
	require.Contains(t, sql, "ALTER TABLE users ADD COLUMN IF NOT EXISTS last_used_ip VARCHAR(45)")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_users_registration_ip ON users (registration_ip)")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_users_last_used_ip ON users (last_used_ip)")
}
