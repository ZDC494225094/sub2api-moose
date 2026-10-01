package rechargecampaigns

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSQLInviterLookupKeepsEligibilityAndPropagatesStorageErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	a := testCampaign()
	a.NewInviteesOnly = true
	lookup := NewSQLInviterLookup(db)
	query := regexp.QuoteMeta("SELECT af.inviter_id FROM user_affiliates af JOIN users u ON u.id=af.inviter_id WHERE af.user_id=$1 AND af.inviter_id<>$1 AND u.status='active' AND u.deleted_at IS NULL AND ($2=FALSE OR (af.invited_at >= $3 AND af.invited_at < $4))")
	for _, tc := range []struct {
		rows   *sqlmock.Rows
		id     int64
		failed bool
	}{
		{sqlmock.NewRows([]string{"inviter_id"}).AddRow(99), 99, false},
		{sqlmock.NewRows([]string{"inviter_id"}), 0, false},
		{sqlmock.NewRows([]string{"inviter_id"}).AddRow("bad-id"), 0, true},
		{sqlmock.NewRows([]string{"inviter_id"}).AddRow(99).RowError(0, errors.New("stream failed")), 0, true},
	} {
		mock.ExpectQuery(query).WithArgs(int64(42), true, a.StartsAt, a.EndsAt).WillReturnRows(tc.rows).RowsWillBeClosed()
		id, err := lookup.EligibleInviter(context.Background(), 42, a)
		if tc.failed {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.id, id)
		}
	}
	wantErr := errors.New("database unavailable")
	mock.ExpectQuery(query).WillReturnError(wantErr)
	_, err = lookup.EligibleInviter(context.Background(), 42, a)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

type rewardAccrueFunc func(context.Context, int64, int64, float64, int, *int64) (bool, error)

func (f rewardAccrueFunc) AccrueQuota(ctx context.Context, inviter, user int64, amount float64, freeze int, order *int64) (bool, error) {
	return f(ctx, inviter, user, amount, freeze, order)
}

func TestRewardAccrualUsesOnlySavedSnapshotAndHostTransactionContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "host-transaction")
	snap := &Snapshot{Campaign: testCampaign(), InviterID: 99, Reward: 2.5}
	snap.Campaign.Enabled = false
	snap.Campaign.FreezeHours = 72
	repo := rewardAccrueFunc(func(got context.Context, inviter, user int64, amount float64, freeze int, order *int64) (bool, error) {
		require.Equal(t, ctx, got)
		require.Equal(t, int64(99), inviter)
		require.Equal(t, int64(42), user)
		require.Equal(t, int64(7), *order)
		require.Equal(t, 2.5, amount)
		require.Equal(t, 72, freeze)
		return true, nil
	})
	amount, err := AccrueReward(ctx, repo, snap, 7, 42)
	require.NoError(t, err)
	require.Equal(t, snap.Reward, amount)
	_, err = AccrueReward(ctx, nil, snap, 7, 42)
	require.ErrorContains(t, err, "repository unavailable")
	_, err = AccrueReward(ctx, rewardAccrueFunc(func(context.Context, int64, int64, float64, int, *int64) (bool, error) { return false, nil }), snap, 7, 42)
	require.ErrorContains(t, err, "inviter 99 unavailable")
	wantErr := errors.New("reward write failed")
	_, err = AccrueReward(ctx, rewardAccrueFunc(func(context.Context, int64, int64, float64, int, *int64) (bool, error) { return false, wantErr }), snap, 7, 42)
	require.ErrorIs(t, err, wantErr)
	for _, empty := range []*Snapshot{nil, {}, {InviterID: 99}, {Reward: 3}} {
		amount, err = AccrueReward(ctx, nil, empty, 7, 42)
		require.NoError(t, err)
		require.Zero(t, amount)
	}
}

const rewardLockSQL = "UPDATE user_affiliates SET updated_at=updated_at WHERE user_id=$1"
const rewardLedgerSQL = "SELECT id,user_id,amount,frozen_until FROM user_affiliate_ledger WHERE source_order_id=$1 AND action='accrue' FOR UPDATE"
const rewardInsertSQL = "INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,source_order_id,frozen_until,created_at,updated_at) VALUES($1,'campaign_refund',$2,$3,$4,$5,NOW(),NOW())"

func rewardQuotaSQL(column string) string {
	return "UPDATE user_affiliates SET " + column + "=" + column + "-$1,aff_history_quota=aff_history_quota-$1,updated_at=NOW() WHERE user_id=$2"
}
func rewardRefund() Refund {
	return Refund{Snapshot: &Snapshot{Campaign: testCampaign(), InviterID: 99, Reward: 3}, OrderID: 7, UserID: 42, OrderAmount: 110, RefundAmount: 55}
}

func TestRewardRefundKeepsPartialPrecisionFrozenLedgerAndDebtSemantics(t *testing.T) {
	for _, frozen := range []bool{false, true} {
		for _, ratio := range []struct{ amount, refund, expected float64 }{{110, 55, 1.5}, {110, 220, 3}, {7, 1, 0.42857143}, {110, 0, 0}} {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			p := rewardRefund()
			p.OrderAmount, p.RefundAmount = ratio.amount, ratio.refund
			p.Snapshot.Campaign.Enabled = false // Historical processing ignores this flag.
			var frozenUntil any
			column := "aff_quota"
			if frozen {
				frozenUntil = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
				column = "aff_frozen_quota"
			}
			mock.ExpectBegin()
			mock.ExpectExec(regexp.QuoteMeta(rewardLockSQL)).WithArgs(int64(99)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(regexp.QuoteMeta(rewardLedgerSQL)).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "amount", "frozen_until"}).AddRow(123, 99, 3, frozenUntil)).RowsWillBeClosed()
			if ratio.expected > 0 {
				mock.ExpectExec(regexp.QuoteMeta(rewardQuotaSQL(column))).WithArgs(ratio.expected, int64(99)).WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectExec(regexp.QuoteMeta(rewardInsertSQL)).WithArgs(int64(99), -ratio.expected, int64(42), int64(7), frozenUntil).WillReturnResult(sqlmock.NewResult(1, 1))
			}
			mock.ExpectCommit()
			tx, err := db.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			require.NoError(t, ReverseReward(context.Background(), tx, p))
			require.NoError(t, tx.Commit(), "only the host commits the shared transaction")
			require.NoError(t, mock.ExpectationsWereMet())
			mock.ExpectClose()
			require.NoError(t, db.Close())
		}
	}
}

func TestRewardRefundStorageFailuresCanRollBackTheSameHostTransaction(t *testing.T) {
	for _, stage := range []string{"lock", "query", "scan", "stream", "quota", "ledger"} {
		t.Run(stage, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			wantErr := errors.New("failure at " + stage)
			mock.ExpectBegin()
			lock := mock.ExpectExec(regexp.QuoteMeta(rewardLockSQL)).WithArgs(int64(99))
			if stage == "lock" {
				lock.WillReturnError(wantErr)
			} else {
				lock.WillReturnResult(sqlmock.NewResult(0, 1))
				query := mock.ExpectQuery(regexp.QuoteMeta(rewardLedgerSQL)).WithArgs(int64(7))
				if stage == "query" {
					query.WillReturnError(wantErr)
				} else {
					rows := sqlmock.NewRows([]string{"id", "user_id", "amount", "frozen_until"})
					if stage == "scan" {
						rows.AddRow("bad-id", 99, 3, nil)
					} else {
						rows.AddRow(123, 99, 3, nil)
					}
					if stage == "stream" {
						rows.RowError(0, wantErr)
					}
					query.WillReturnRows(rows).RowsWillBeClosed()
					if stage != "scan" && stage != "stream" {
						quota := mock.ExpectExec(regexp.QuoteMeta(rewardQuotaSQL("aff_quota"))).WithArgs(1.5, int64(99))
						if stage == "quota" {
							quota.WillReturnError(wantErr)
						} else {
							quota.WillReturnResult(sqlmock.NewResult(0, 1))
							mock.ExpectExec(regexp.QuoteMeta(rewardInsertSQL)).WillReturnError(wantErr)
						}
					}
				}
			}
			mock.ExpectRollback()
			tx, err := db.BeginTx(context.Background(), nil)
			require.NoError(t, err)
			err = ReverseReward(context.Background(), tx, rewardRefund())
			if stage == "scan" {
				require.Error(t, err)
			} else {
				require.ErrorIs(t, err, wantErr)
			}
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRewardRefundNoSnapshotOrUnaccruedRewardHasNoWrite(t *testing.T) {
	for _, snap := range []*Snapshot{nil, {}} {
		require.NoError(t, ReverseReward(context.Background(), nil, Refund{Snapshot: snap}))
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(rewardLockSQL)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(rewardLedgerSQL)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "amount", "frozen_until"})).RowsWillBeClosed()
	mock.ExpectCommit()
	var tx *sql.Tx
	tx, err = db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	require.NoError(t, ReverseReward(context.Background(), tx, rewardRefund()))
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}
