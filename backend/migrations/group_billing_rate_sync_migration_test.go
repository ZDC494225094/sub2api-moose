package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration185AddsGroupBillingRateSyncAndRateCacheInvalidation(t *testing.T) {
	content, err := FS.ReadFile("185_group_billing_rate_sync.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "billing_rate_sync_account_id BIGINT")
	require.Contains(t, sql, "billing_rate_markup DECIMAL(10,4) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "CHECK (billing_rate_markup >= 0)")
	require.Contains(t, sql, "FOREIGN KEY (billing_rate_sync_account_id)")
	require.Contains(t, sql, "ON DELETE SET NULL")
	require.Contains(t, sql, "OLD.rate_multiplier IS NOT DISTINCT FROM NEW.rate_multiplier")
}
