package migrations

import (
	"context"
	"github.com/stretchr/testify/require"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestKeyRoutingOwnershipMigration(t *testing.T) {
	for _, failLedger := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "ledger rollback"}[failLedger], func(t *testing.T) {
			ctx, conn := context.Background(), fixture(t)
			execute(t, conn, "INSERT INTO api_keys(id) VALUES (7)")
			require.NoError(t, (Runner{}).Prepare(ctx, conn))
			path := "sql/multi-group-billing/202609260003_key_routing_ownership.sql"
			raw, err := fs.ReadFile(migrationFiles, path)
			require.NoError(t, err)
			files := fstest.MapFS{path: &fstest.MapFile{Data: raw}}
			if failLedger {
				execute(t, conn, ledgerDDL)
				execute(t, conn, `CREATE TRIGGER reject_ownership BEFORE INSERT ON custom_extension_schema_migrations WHEN NEW.module_id='multi-group-billing' BEGIN SELECT RAISE(ABORT,'ownership ledger failed'); END`)
				require.ErrorContains(t, applyFS(ctx, conn, files), "ownership ledger failed")
				var value string
				require.Error(t, conn.QueryRowContext(ctx, "SELECT custom_routing_policy FROM api_keys WHERE id=7").Scan(&value), "DDL must roll back with ledger")
				require.Zero(t, recorded(t, conn))
				execute(t, conn, "DROP TRIGGER reject_ownership")
			}
			require.NoError(t, applyFS(ctx, conn, files))
			var value string
			require.NoError(t, conn.QueryRowContext(ctx, "SELECT custom_routing_policy FROM api_keys WHERE id=7").Scan(&value))
			require.Equal(t, "multigroup-v1", value)
			execute(t, conn, "INSERT INTO api_keys(id, custom_routing_policy) VALUES (8,'upstream-v1')")
			require.NoError(t, applyFS(ctx, conn, files))
			require.NoError(t, conn.QueryRowContext(ctx, "SELECT custom_routing_policy FROM api_keys WHERE id=8").Scan(&value))
			require.Equal(t, "upstream-v1", value, "restart must not reassign explicit ownership")
			require.Equal(t, 1, recorded(t, conn))
		})
	}
}
