package service

import (
	"context"
	"errors"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/redeemmarketing"
	"github.com/stretchr/testify/require"
)

func TestMarketingBatchAtomicPolicyAndCodes(t *testing.T) {
	for _, mode := range []string{"commit", "generator_failure", "policy_failure", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			_, err := client.ExecContext(ctx, `CREATE TABLE custom_redeem_marketing_batches(batch_id TEXT PRIMARY KEY, registered_after TIMESTAMP NOT NULL)`)
			require.NoError(t, err)
			flag := "true"
			if mode == "disabled" {
				flag = "false"
			}
			_, err = client.Setting.Create().SetKey("custom_extensions.redeem-marketing.enabled").SetValue(flag).Save(ctx)
			require.NoError(t, err)
			if mode == "policy_failure" {
				_, err := client.ExecContext(ctx, `CREATE TRIGGER reject_policy BEFORE INSERT ON custom_redeem_marketing_batches BEGIN SELECT RAISE(ABORT,'policy failure'); END`)
				require.NoError(t, err)
			}
			svc := &RedeemService{entClient: client}
			called := false
			_, err = svc.GenerateMarketingBatch(ctx, redeemmarketing.Options{NewUsersOnly: true}, func(txCtx context.Context) ([]RedeemCode, error) {
				called = true
				tx := dbent.TxFromContext(txCtx)
				require.NotNil(t, tx)
				_, err := tx.Client().RedeemCode.Create().SetCode("TEST").SetType(RedeemTypeMarketing).SetValue(10).SetStatus(StatusUnused).SetBatchID("batch").Save(txCtx)
				require.NoError(t, err)
				if mode == "generator_failure" {
					return nil, errors.New("generator failure")
				}
				batch := "batch"
				return []RedeemCode{{BatchID: &batch}}, nil
			})
			want := 0
			if mode == "commit" {
				require.NoError(t, err)
				want = 1
			} else {
				require.Error(t, err)
			}
			if mode == "disabled" {
				require.False(t, called)
			}
			count, err := client.RedeemCode.Query().Count(ctx)
			require.NoError(t, err)
			require.Equal(t, want, count)
			rows, err := client.QueryContext(ctx, "SELECT COUNT(*) FROM custom_redeem_marketing_batches")
			require.NoError(t, err)
			require.True(t, rows.Next())
			require.NoError(t, rows.Scan(&count))
			require.NoError(t, rows.Close())
			require.Equal(t, want, count)
		})
	}
}

type marketingEligibilityUserRepo struct {
	UserRepository
	registered time.Time
}

func (r marketingEligibilityUserRepo) GetByID(context.Context, int64) (*User, error) {
	return &User{ID: 1, CreatedAt: r.registered}, nil
}

type marketingEligibilityRepo struct {
	*redeemRejectRepo
	checked bool
}

var eligibilityReachedBatchGuard = errors.New("reached batch guard")

func (r *marketingEligibilityRepo) HasMarketingBatchUsage(context.Context, string, int64) (bool, error) {
	r.checked = true
	return false, eligibilityReachedBatchGuard
}
func (*marketingEligibilityRepo) CreateMarketingBatchUsage(context.Context, string, int64, int64) error {
	panic("unexpected write")
}

func TestMarketingEligibilityIsEnforcedBeforeRedeemWrites(t *testing.T) {
	for _, mode := range []string{"old_user", "new_user", "unrestricted"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			_, err := client.ExecContext(ctx, `CREATE TABLE custom_redeem_marketing_batches(batch_id TEXT PRIMARY KEY, registered_after TIMESTAMP NOT NULL)`)
			require.NoError(t, err)
			cutoff := time.Now().UTC()
			if mode != "unrestricted" {
				require.NoError(t, redeemmarketing.SavePolicy(ctx, client, "batch", cutoff))
			}
			registered := cutoff.Add(-time.Hour)
			if mode == "new_user" {
				registered = cutoff.Add(time.Hour)
			}
			batch := "batch"
			repo := &marketingEligibilityRepo{redeemRejectRepo: &redeemRejectRepo{code: RedeemCode{ID: 1, Code: "TEST", Type: RedeemTypeMarketing, Status: StatusUnused, BatchID: &batch}}}
			svc := NewRedeemService(repo, marketingEligibilityUserRepo{registered: registered}, nil, nil, nil, client, nil, nil)
			// No enabled flag: historical restrictions do not depend on admission.
			_, err = svc.Redeem(ctx, 1, "TEST")
			if mode == "old_user" {
				require.ErrorContains(t, err, "仅限")
				require.False(t, repo.checked)
			} else {
				require.ErrorIs(t, err, eligibilityReachedBatchGuard)
				require.True(t, repo.checked)
			}
			require.False(t, repo.useCalled)
		})
	}
}
