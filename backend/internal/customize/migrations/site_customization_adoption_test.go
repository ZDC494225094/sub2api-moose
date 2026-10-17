package migrations

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/stretchr/testify/require"
)

func TestSiteCustomizationAdoptionPreservesExplicitChoices(t *testing.T) {
	key := customize.Key("site-customization")
	for _, legacy := range []bool{false, true} {
		for _, value := range []string{"true", "false", "malformed"} {
			name := "native/" + value
			if legacy {
				name = "legacy/" + value
			}
			t.Run(name, func(t *testing.T) {
				ctx, conn := context.Background(), fixture(t)
				if legacy {
					execute(t, conn, "INSERT INTO schema_migrations VALUES ('146_api_key_platform.sql','original')")
				}
				require.NoError(t, (Runner{}).Prepare(ctx, conn))
				createSettings(t, conn)
				execute(t, conn, "INSERT INTO settings (key,value) VALUES ($1,$2)", key, value)
				require.NoError(t, (Runner{}).Apply(ctx, conn))
				require.Equal(t, value, flags(t, conn)[key])
				// A deleted flag is not re-seeded on restart: the ledger records adoption once.
				execute(t, conn, "DELETE FROM settings WHERE key=$1", key)
				require.NoError(t, (Runner{}).Prepare(ctx, conn))
				require.NoError(t, (Runner{}).Apply(ctx, conn))
				require.NotContains(t, flags(t, conn), key)
			})
		}
	}
}

func TestSiteCustomizationAdoptionDefaultsAndLedgerRollback(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "legacy"}[legacy], func(t *testing.T) {
			ctx, conn := context.Background(), fixture(t)
			if legacy {
				execute(t, conn, "INSERT INTO schema_migrations VALUES ('146_api_key_platform.sql','original')")
			}
			require.NoError(t, (Runner{}).Prepare(ctx, conn))
			createSettings(t, conn)
			execute(t, conn, ledgerDDL)
			execute(t, conn, `CREATE TRIGGER reject_site_ledger BEFORE INSERT ON custom_extension_schema_migrations
WHEN NEW.module_id='site-customization' BEGIN SELECT RAISE(ABORT,'site ledger failed'); END`)
			require.ErrorContains(t, (Runner{}).Apply(ctx, conn), "site ledger failed")
			require.NotContains(t, flags(t, conn), customize.Key("site-customization"))
			require.Equal(t, 12, recorded(t, conn))
			execute(t, conn, "DROP TRIGGER reject_site_ledger")
			require.NoError(t, (Runner{}).Apply(ctx, conn))
			require.Equal(t, map[bool]string{false: "false", true: "true"}[legacy], flags(t, conn)[customize.Key("site-customization")])
		})
	}
}
