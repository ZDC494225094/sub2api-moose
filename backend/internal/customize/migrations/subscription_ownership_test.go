package migrations

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSubscriptionAdoptionPreservesGrantsCodesAndChoices(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, choice := range []string{"missing", "true", "false", "malformed"} {
			t.Run(map[bool]string{false: "native", true: "legacy"}[legacy]+"/"+choice, func(t *testing.T) {
				ctx, conn := context.Background(), fixture(t)
				if legacy {
					execute(t, conn, "INSERT INTO schema_migrations VALUES ('146_api_key_platform.sql','original')")
				}
				require.NoError(t, (Runner{}).Prepare(ctx, conn))
				createSettings(t, conn)
				execute(t, conn, "INSERT INTO user_subscriptions (id,user_id,group_id) VALUES (1,7,9),(2,7,9)")
				execute(t, conn, "INSERT INTO redeem_codes (id) VALUES (1)")
				key := customize.Key("subscription-extensions")
				if choice != "missing" {
					execute(t, conn, "INSERT INTO settings (key,value) VALUES ($1,$2)", key, choice)
				}
				require.NoError(t, (Runner{}).Apply(ctx, conn))
				want := choice
				if choice == "missing" {
					want = map[bool]string{false: "false", true: "true"}[legacy]
				}
				require.Equal(t, want, flags(t, conn)[key])
				var n int
				require.NoError(t, conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_subscriptions WHERE custom_subscription_policy='independent-v1'").Scan(&n))
				require.Equal(t, 2, n)
				var policy string
				require.NoError(t, conn.QueryRowContext(ctx, "SELECT custom_subscription_policy FROM redeem_codes WHERE id=1").Scan(&policy))
				require.Equal(t, "independent-v1", policy)
				execute(t, conn, "DELETE FROM settings WHERE key=$1", key)
				require.NoError(t, (Runner{}).Apply(ctx, conn))
				require.NotContains(t, flags(t, conn), key)
			})
		}
	}
}
func TestSubscriptionOwnershipSchemaAndLedgerRollback(t *testing.T) {
	ctx, conn := context.Background(), fixture(t)
	require.NoError(t, (Runner{}).Prepare(ctx, conn))
	createSettings(t, conn)
	execute(t, conn, ledgerDDL)
	execute(t, conn, `CREATE TRIGGER reject_subscription_ledger BEFORE INSERT ON custom_extension_schema_migrations WHEN NEW.module_id='subscription-extensions' BEGIN SELECT RAISE(ABORT,'subscription ledger failed'); END`)
	require.ErrorContains(t, (Runner{}).Apply(ctx, conn), "subscription ledger failed")
	require.NotContains(t, flags(t, conn), customize.Key("subscription-extensions"))
	_, err := conn.ExecContext(ctx, "SELECT custom_subscription_policy FROM user_subscriptions")
	require.Error(t, err)
	_, err = conn.ExecContext(ctx, "SELECT custom_subscription_policy FROM redeem_codes")
	require.Error(t, err)
	execute(t, conn, "DROP TRIGGER reject_subscription_ledger")
	require.NoError(t, (Runner{}).Apply(ctx, conn))
}
