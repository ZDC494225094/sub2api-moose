package migrations

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/stretchr/testify/require"
)

func TestMarketingAdoptionIsSeparateFromFrozenV1(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "native"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			ctx, conn := context.Background(), fixture(t)
			if legacy {
				execute(t, conn, "INSERT INTO schema_migrations VALUES ('138_marketing_lottery_coupon.sql','original')")
			}
			require.NoError(t, (Runner{}).Prepare(ctx, conn))
			createSettings(t, conn)
			path := "sql/extension-host/202609260001_adopt_existing_modules.sql"
			raw, err := fs.ReadFile(migrationFiles, path)
			require.NoError(t, err)
			require.NoError(t, applyFS(ctx, conn, fstest.MapFS{path: &fstest.MapFile{Data: raw}}))
			v1 := flags(t, conn)
			require.Len(t, v1, 6)
			require.NotContains(t, v1, customize.Key("marketing-tools"))
			require.Equal(t, 1, recorded(t, conn))
			require.NoError(t, (Runner{}).Apply(ctx, conn))
			current := flags(t, conn)
			require.Len(t, current, 13)
			want := "false"
			if legacy {
				want = "true"
			}
			require.Equal(t, want, current[customize.Key("marketing-tools")])
			for key, value := range v1 {
				require.Equal(t, value, current[key])
			}
			require.Equal(t, 9, recorded(t, conn))
		})
	}
}

func TestMarketingAdoptionPreservesChoicesAndDeletedFlags(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, value := range []string{"true", "false", "malformed"} {
			name := "native/" + value
			if legacy {
				name = "legacy/" + value
			}
			t.Run(name, func(t *testing.T) {
				ctx, conn := context.Background(), fixture(t)
				if legacy {
					execute(t, conn, "INSERT INTO schema_migrations VALUES ('138_marketing_lottery_coupon.sql','original')")
				}
				require.NoError(t, (Runner{}).Prepare(ctx, conn))
				createSettings(t, conn)
				key := customize.Key("marketing-tools")
				execute(t, conn, "INSERT INTO settings (key,value) VALUES ($1,$2)", key, value)
				require.NoError(t, (Runner{}).Apply(ctx, conn))
				require.Equal(t, value, flags(t, conn)[key])
				execute(t, conn, "DELETE FROM settings WHERE key=$1", key)
				require.NoError(t, (Runner{}).Prepare(ctx, conn))
				require.NoError(t, (Runner{}).Apply(ctx, conn))
				require.NotContains(t, flags(t, conn), key)
				require.Equal(t, 9, recorded(t, conn))
			})
		}
	}
}

func TestMarketingAdoptionLedgerFailureRollsBackOnlyItsMigration(t *testing.T) {
	ctx, conn := context.Background(), fixture(t)
	execute(t, conn, "INSERT INTO schema_migrations VALUES ('138_marketing_lottery_coupon.sql','original')")
	require.NoError(t, (Runner{}).Prepare(ctx, conn))
	createSettings(t, conn)
	// Fail after the setting write, at the ledger insert, not before any work.
	execute(t, conn, ledgerDDL)
	execute(t, conn, `CREATE TRIGGER reject_marketing_ledger BEFORE INSERT ON custom_extension_schema_migrations
 WHEN NEW.module_id='marketing-tools' BEGIN SELECT RAISE(ABORT,'marketing ledger failed'); END`)
	require.ErrorContains(t, (Runner{}).Apply(ctx, conn), "marketing ledger failed")
	require.Equal(t, 3, recorded(t, conn))
	require.Len(t, flags(t, conn), 8)
	require.NotContains(t, flags(t, conn), customize.Key("marketing-tools"))
	execute(t, conn, "DROP TRIGGER reject_marketing_ledger")
	require.NoError(t, (Runner{}).Apply(ctx, conn))
	require.Equal(t, "true", flags(t, conn)[customize.Key("marketing-tools")])
	require.Equal(t, 9, recorded(t, conn))
}
