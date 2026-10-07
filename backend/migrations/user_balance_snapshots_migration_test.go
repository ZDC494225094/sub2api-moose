package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserBalanceSnapshotsMigrationCapturesMonthEndBeforeBalanceChanges(t *testing.T) {
	content, err := FS.ReadFile("242_add_user_balance_snapshots.sql")
	require.NoError(t, err)
	query := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, query, "CREATE TABLE IF NOT EXISTS user_balance_snapshots")
	require.Contains(t, query, "ON user_balance_snapshots (user_id, period)")
	require.Contains(t, query, "BEFORE UPDATE OF balance ON users")
	require.Contains(t, query, "OLD.balance, captured_at")
	require.Contains(t, query, "OLD.created_at < month_start")
	require.Contains(t, query, "WHERE user_balance_snapshots.created_at < month_start")
}
