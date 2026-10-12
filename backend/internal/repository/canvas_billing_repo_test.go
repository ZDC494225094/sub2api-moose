package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func canvasRows(c service.CanvasCharge) *sqlmock.Rows {
	return sqlmock.NewRows(strings.Split(canvasChargeColumns, ",")).AddRow(c.ID, c.UserID, c.APIKeyID, c.GroupID, c.SubscriptionID, c.RunID, c.PayloadHash, c.Mode, c.Model, c.Status, c.ReservedCost, c.ActualCost, c.ChargedCost, c.UsageRecorded, c.DailyWindowStart, c.WeeklyWindowStart, c.MonthlyWindowStart, c.CreatedAt)
}
func canvasTestCharge() service.CanvasCharge {
	return service.CanvasCharge{ID: "charge", UserID: 7, APIKeyID: 9, GroupID: 11, RunID: "run", PayloadHash: "payload", Mode: "image", Model: "model", Status: "reserved", ReservedCost: 3, CreatedAt: time.Now()}
}
func canvasTestRepo(t *testing.T) (*usageBillingRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, m, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, m.ExpectationsWereMet()); _ = db.Close() })
	return &usageBillingRepository{db: db}, m
}

const canvasMoneySQL = `(?s)UPDATE users SET balance=balance-\$1,frozen_balance=COALESCE\(frozen_balance,0\)-\$2.*WHERE id=\$3 AND COALESCE\(frozen_balance,0\)>=\$2`
const canvasHoldSQL = `(?s)UPDATE users SET balance=balance-\$1,frozen_balance=COALESCE\(frozen_balance,0\)\+\$1.*AND \(\$1::numeric=0 OR balance >= \$1\) RETURNING balance`

func expectCanvasInsert(m sqlmock.Sqlmock, c service.CanvasCharge) *sqlmock.ExpectedQuery {
	return m.ExpectQuery(`(?s)INSERT INTO canvas_run_charges.*ON CONFLICT \(user_id,run_id\) DO NOTHING RETURNING id`).WithArgs(c.ID, "hash", c.UserID, c.APIKeyID, c.GroupID, c.SubscriptionID, c.RunID, c.PayloadHash, c.Mode, c.Model, c.ReservedCost, c.DailyWindowStart, c.WeeklyWindowStart, c.MonthlyWindowStart)
}
func TestCanvasReserveAtomicBalanceHold(t *testing.T) {
	for _, sufficient := range []bool{true, false} {
		t.Run(map[bool]string{true: "success", false: "insufficient"}[sufficient], func(t *testing.T) {
			r, m := canvasTestRepo(t)
			c := canvasTestCharge()
			c.ReservedCost = 0.268
			m.ExpectBegin()
			expectCanvasInsert(m, c).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(c.ID))
			hold := m.ExpectQuery(canvasHoldSQL).WithArgs(0.268, int64(7))
			if sufficient {
				hold.WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(7.0))
				m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1`).WithArgs(c.ID).WillReturnRows(canvasRows(c))
				m.ExpectCommit()
			} else {
				hold.WillReturnError(sql.ErrNoRows)
				m.ExpectRollback()
			}
			got, created, err := r.ReserveCanvasCharge(context.Background(), &service.CanvasChargeReservation{CanvasCharge: c, TokenHash: "hash"})
			if sufficient {
				require.NoError(t, err)
				require.True(t, created)
				require.Equal(t, 0.268, got.ReservedCost)
			} else {
				require.ErrorIs(t, err, service.ErrInsufficientBalance)
				require.False(t, created)
			}
		})
	}
}
func TestCanvasReserveDuplicateDoesNotDeductAgain(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{true: "conflict", false: "same"}[conflict], func(t *testing.T) {
			r, m := canvasTestRepo(t)
			c := canvasTestCharge()
			stored := c
			if conflict {
				stored.PayloadHash = "different"
			}
			m.ExpectBegin()
			expectCanvasInsert(m, c).WillReturnError(sql.ErrNoRows)
			m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE user_id=\$1 AND run_id=\$2`).WithArgs(c.UserID, c.RunID).WillReturnRows(canvasRows(stored))
			m.ExpectRollback()
			got, created, err := r.ReserveCanvasCharge(context.Background(), &service.CanvasChargeReservation{CanvasCharge: c, TokenHash: "hash"})
			require.False(t, created)
			if conflict {
				require.ErrorIs(t, err, service.ErrCanvasChargeConflict)
			} else {
				require.NoError(t, err)
				require.Equal(t, c.ID, got.ID)
			}
		})
	}
}
func TestCanvasFinishReconcilesAndRefundsExactlyOnce(t *testing.T) {
	cases := []struct {
		name           string
		success, seen  bool
		actual, target float64
		status         string
	}{
		{"failure", false, true, 2, 0, "failed"}, {"success_less", true, true, 2, 2, "succeeded"},
		{"success_more", true, true, 5, 5, "succeeded"}, {"zero_actual", true, true, 0, 0, "succeeded"},
		{"late_metering", true, false, 0, 3, "succeeded"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r, m := canvasTestRepo(t)
			c := canvasTestCharge()
			c.ActualCost = tt.actual
			c.UsageRecorded = tt.seen
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1 FOR UPDATE`).WithArgs(c.ID).WillReturnRows(canvasRows(c))
			m.ExpectExec(canvasMoneySQL).WithArgs(tt.target-c.ReservedCost, c.ReservedCost, c.UserID).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`UPDATE canvas_run_charges SET status=\$2,charged_cost=\$3,updated_at=NOW\(\) WHERE id=\$1`).WithArgs(c.ID, tt.status, tt.target).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
			got, err := r.FinishCanvasCharge(context.Background(), c.ID, tt.success)
			require.NoError(t, err)
			require.Equal(t, tt.target, got.ChargedCost)
			require.Equal(t, tt.status, got.Status)
			// A repeated/contradictory terminal event must not capture or refund again.
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1 FOR UPDATE`).WithArgs(c.ID).WillReturnRows(canvasRows(*got))
			m.ExpectRollback()
			again, err := r.FinishCanvasCharge(context.Background(), c.ID, !tt.success)
			require.NoError(t, err)
			require.Equal(t, tt.status, again.Status)
			require.Equal(t, tt.target, again.ChargedCost)
		})
	}
}
func TestCanvasSettlementRollbackKeepsHoldWhenMoneyUpdateFails(t *testing.T) {
	r, m := canvasTestRepo(t)
	c := canvasTestCharge()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1 FOR UPDATE`).WillReturnRows(canvasRows(c))
	m.ExpectExec(canvasMoneySQL).WillReturnError(errors.New("db unavailable"))
	m.ExpectRollback()
	_, err := r.FinishCanvasCharge(context.Background(), c.ID, false)
	require.Error(t, err)
}
func TestCanvasLateUsageCannotRechargeRefundedRun(t *testing.T) {
	for _, status := range []string{"reserved", "succeeded", "failed"} {
		t.Run(status, func(t *testing.T) {
			r, m := canvasTestRepo(t)
			c := canvasTestCharge()
			c.Status = status
			if status == "succeeded" {
				c.ChargedCost = 3
			}
			cmd := &service.UsageBillingCommand{CanvasChargeID: c.ID, RequestID: "usage", UserID: c.UserID, APIKeyID: c.APIKeyID, BalanceCost: 1.25}
			cmd.Normalize()
			m.ExpectBegin()
			m.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup .*RETURNING id`).WithArgs(cmd.RequestID, cmd.APIKeyID, cmd.RequestFingerprint).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			m.ExpectQuery(`(?s)SELECT request_fingerprint.*FROM usage_billing_dedup_archive`).WillReturnError(sql.ErrNoRows)
			m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1 FOR UPDATE`).WithArgs(c.ID).WillReturnRows(canvasRows(c))
			if status == "succeeded" {
				m.ExpectExec(canvasMoneySQL).WithArgs(-1.75, 0.0, c.UserID).WillReturnResult(sqlmock.NewResult(0, 1))
				c.ChargedCost = 1.25
			}
			m.ExpectExec(`UPDATE canvas_run_charges SET actual_cost=\$2,charged_cost=\$3,usage_recorded=TRUE,updated_at=NOW\(\) WHERE id=\$1`).WithArgs(c.ID, 1.25, c.ChargedCost).WillReturnResult(sqlmock.NewResult(0, 1))
			// No standard wallet deduction SQL is expected: ledger is sole money owner.
			m.ExpectCommit()
			result, err := r.Apply(context.Background(), cmd)
			require.NoError(t, err)
			require.True(t, result.Applied)
			require.True(t, result.CanvasCharge.UsageRecorded)
			if status == "failed" {
				require.Zero(t, result.CanvasCharge.ChargedCost)
			}
			m.ExpectBegin()
			m.ExpectQuery(`(?s)INSERT INTO usage_billing_dedup .*RETURNING id`).WillReturnError(sql.ErrNoRows)
			m.ExpectQuery(`(?s)SELECT request_fingerprint.*FROM usage_billing_dedup\s+WHERE`).WillReturnRows(sqlmock.NewRows([]string{"request_fingerprint"}).AddRow(cmd.RequestFingerprint))
			m.ExpectRollback()
			again, err := r.Apply(context.Background(), cmd)
			require.NoError(t, err)
			require.False(t, again.Applied)
		})
	}
}
func TestCanvasSubscriptionUsesExactIDAndOriginalWindows(t *testing.T) {
	r, m := canvasTestRepo(t)
	c := canvasTestCharge()
	id := int64(77)
	now := time.Now()
	c.SubscriptionID = &id
	c.DailyWindowStart = &now
	c.WeeklyWindowStart = &now
	c.MonthlyWindowStart = &now
	m.ExpectBegin()
	expectCanvasInsert(m, c).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(c.ID))
	m.ExpectQuery(`(?s)UPDATE user_subscriptions us.*WHERE us.id=\$2 AND us.user_id=\$3 AND us.group_id=\$4.*daily_usage_usd\+\$1<=g.daily_limit_usd.*daily_window_start IS NOT DISTINCT FROM \$5::timestamptz.*weekly_window_start IS NOT DISTINCT FROM \$6::timestamptz.*monthly_window_start IS NOT DISTINCT FROM \$7::timestamptz`).WithArgs(3.0, id, c.UserID, c.GroupID, now, now, now).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
	m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1`).WillReturnRows(canvasRows(c))
	m.ExpectCommit()
	_, created, err := r.ReserveCanvasCharge(context.Background(), &service.CanvasChargeReservation{CanvasCharge: c, TokenHash: "hash"})
	require.NoError(t, err)
	require.True(t, created)
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE id=\$1 FOR UPDATE`).WillReturnRows(canvasRows(c))
	m.ExpectExec(`(?s)UPDATE user_subscriptions SET.*daily_window_start IS NOT DISTINCT FROM \$3::timestamptz.*weekly_window_start IS NOT DISTINCT FROM \$4::timestamptz.*monthly_window_start IS NOT DISTINCT FROM \$5::timestamptz.*WHERE id=\$2`).WithArgs(-3.0, id, now, now, now).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`UPDATE canvas_run_charges SET status=`).WithArgs(c.ID, "failed", 0.0).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	_, err = r.FinishCanvasCharge(context.Background(), c.ID, false)
	require.NoError(t, err)
}
func TestCanvasRecoveryRechecksLeaseUnderLock(t *testing.T) {
	r, m := canvasTestRepo(t)
	m.ExpectBegin()
	m.ExpectQuery(regexp.QuoteMeta(`SELECT ` + canvasChargeColumns + ` FROM canvas_run_charges WHERE id=$1 AND status='reserved' AND lease_until <= NOW() FOR UPDATE`)).WithArgs("renewed").WillReturnError(sql.ErrNoRows)
	m.ExpectRollback()
	got, err := r.FailExpiredCanvasCharge(context.Background(), "renewed")
	require.NoError(t, err)
	require.Nil(t, got)
	m.ExpectQuery(`SELECT .* FROM canvas_run_charges WHERE token_hash=\$1 AND api_key_id=\$2 AND status='reserved' AND lease_until > NOW\(\)`).WithArgs("hash", int64(9)).WillReturnError(sql.ErrNoRows)
	got, err = r.AuthenticateCanvasCharge(context.Background(), "hash", 9)
	require.NoError(t, err)
	require.Nil(t, got)
}
