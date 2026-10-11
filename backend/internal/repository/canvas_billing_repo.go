package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func NewCanvasBillingRepository(db *sql.DB) service.CanvasBillingRepository {
	return &usageBillingRepository{db: db}
}

const canvasChargeColumns = `id,user_id,api_key_id,group_id,subscription_id,run_id,payload_hash,mode,model,status,reserved_cost,actual_cost,charged_cost,usage_recorded,daily_window_start,weekly_window_start,monthly_window_start,created_at`

type canvasRowScanner interface{ Scan(...any) error }

func scanCanvasCharge(row canvasRowScanner) (*service.CanvasCharge, error) {
	c := &service.CanvasCharge{}
	err := row.Scan(&c.ID, &c.UserID, &c.APIKeyID, &c.GroupID, &c.SubscriptionID, &c.RunID, &c.PayloadHash, &c.Mode, &c.Model, &c.Status, &c.ReservedCost, &c.ActualCost, &c.ChargedCost, &c.UsageRecorded, &c.DailyWindowStart, &c.WeeklyWindowStart, &c.MonthlyWindowStart, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return c, err
}
func (r *usageBillingRepository) GetCanvasCharge(ctx context.Context, userID int64, runID string) (*service.CanvasCharge, error) {
	return scanCanvasCharge(r.db.QueryRowContext(ctx, `SELECT `+canvasChargeColumns+` FROM canvas_run_charges WHERE user_id=$1 AND run_id=$2`, userID, runID))
}
func (r *usageBillingRepository) AuthenticateCanvasCharge(ctx context.Context, hash string, keyID int64) (*service.CanvasCharge, error) {
	return scanCanvasCharge(r.db.QueryRowContext(ctx, `SELECT `+canvasChargeColumns+` FROM canvas_run_charges WHERE token_hash=$1 AND api_key_id=$2 AND status='reserved' AND lease_until > NOW()`, hash, keyID))
}
func (r *usageBillingRepository) ReserveCanvasCharge(ctx context.Context, in *service.CanvasChargeReservation) (*service.CanvasCharge, bool, error) {
	if in == nil || in.ReservedCost < 0 || math.IsNaN(in.ReservedCost) || math.IsInf(in.ReservedCost, 0) {
		return nil, false, service.ErrCanvasChargeUnavailable
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()
	var id string
	err = tx.QueryRowContext(ctx, `INSERT INTO canvas_run_charges
        (id,token_hash,user_id,api_key_id,group_id,subscription_id,run_id,payload_hash,mode,model,reserved_cost,daily_window_start,weekly_window_start,monthly_window_start)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
        ON CONFLICT (user_id,run_id) DO NOTHING RETURNING id`, in.ID, in.TokenHash, in.UserID, in.APIKeyID, in.GroupID, in.SubscriptionID, in.RunID, in.PayloadHash, in.Mode, in.Model, in.ReservedCost, in.DailyWindowStart, in.WeeklyWindowStart, in.MonthlyWindowStart).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		c, err := scanCanvasCharge(tx.QueryRowContext(ctx, `SELECT `+canvasChargeColumns+` FROM canvas_run_charges WHERE user_id=$1 AND run_id=$2`, in.UserID, in.RunID))
		if err != nil {
			return nil, false, err
		}
		if c == nil || c.PayloadHash != in.PayloadHash || c.APIKeyID != in.APIKeyID {
			return nil, false, service.ErrCanvasChargeConflict
		}
		return c, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if in.SubscriptionID == nil {
		var balance float64
		err = tx.QueryRowContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)+$1,updated_at=NOW()
            WHERE id=$2 AND deleted_at IS NULL AND status='active' AND ($1=0 OR balance >= $1) RETURNING balance`, in.ReservedCost, in.UserID).Scan(&balance)
	} else {
		// Window identities are checked as well as limits. A concurrent reset or
		// subscription replacement cannot reserve from a stale snapshot.
		var subID int64
		err = tx.QueryRowContext(ctx, `UPDATE user_subscriptions us SET daily_usage_usd=us.daily_usage_usd+$1,
            weekly_usage_usd=us.weekly_usage_usd+$1,monthly_usage_usd=us.monthly_usage_usd+$1,updated_at=NOW()
            FROM groups g WHERE us.id=$2 AND us.user_id=$3 AND us.group_id=$4 AND us.deleted_at IS NULL
            AND us.status='active' AND us.expires_at>NOW() AND us.starts_at<=NOW()
            AND g.id=us.group_id AND g.deleted_at IS NULL AND g.status='active'
            AND (COALESCE(g.daily_limit_usd,0)<=0 OR us.daily_usage_usd+$1<=g.daily_limit_usd)
            AND (COALESCE(g.weekly_limit_usd,0)<=0 OR us.weekly_usage_usd+$1<=g.weekly_limit_usd)
            AND (COALESCE(g.monthly_limit_usd,0)<=0 OR us.monthly_usage_usd+$1<=g.monthly_limit_usd)
            AND us.daily_window_start IS NOT DISTINCT FROM $5::timestamptz
            AND us.weekly_window_start IS NOT DISTINCT FROM $6::timestamptz
            AND us.monthly_window_start IS NOT DISTINCT FROM $7::timestamptz RETURNING us.id`, in.ReservedCost, in.SubscriptionID, in.UserID, in.GroupID, in.DailyWindowStart, in.WeeklyWindowStart, in.MonthlyWindowStart).Scan(&subID)
	}
	if errors.Is(err, sql.ErrNoRows) {
		if in.SubscriptionID != nil {
			return nil, false, service.ErrCanvasQuotaUnavailable
		}
		return nil, false, service.ErrInsufficientBalance
	}
	if err != nil {
		return nil, false, err
	}
	c, err := scanCanvasCharge(tx.QueryRowContext(ctx, `SELECT `+canvasChargeColumns+` FROM canvas_run_charges WHERE id=$1`, id))
	if err != nil {
		return nil, false, err
	}
	if err = tx.Commit(); err != nil {
		return nil, false, err
	}
	return c, true, nil
}

// adjustCanvasMoney changes only the difference from the current debit. All
// callers hold the ledger row lock; reserve/refund/capture can therefore happen
// exactly once even across multiple application instances and late usage jobs.
func adjustCanvasMoney(ctx context.Context, tx *sql.Tx, c *service.CanvasCharge, target float64, release bool) error {
	previous := c.ChargedCost
	if release {
		previous = c.ReservedCost
	}
	delta := service.QuantizeUsageBillingAmount(target - previous)
	if c.SubscriptionID == nil {
		frozen := 0.0
		if release {
			frozen = c.ReservedCost
		}
		result, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance-$1,frozen_balance=COALESCE(frozen_balance,0)-$2,updated_at=NOW()
            WHERE id=$3 AND COALESCE(frozen_balance,0)>=$2`, delta, frozen, c.UserID)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("canvas frozen balance mismatch")
		}
	} else {
		// Refund/delta belongs to the original subscription and original windows,
		// never a replacement subscription or a window already reset by cron.
		result, err := tx.ExecContext(ctx, `UPDATE user_subscriptions SET
            daily_usage_usd=CASE WHEN daily_window_start IS NOT DISTINCT FROM $3::timestamptz THEN GREATEST(0,daily_usage_usd+$1) ELSE daily_usage_usd END,
            weekly_usage_usd=CASE WHEN weekly_window_start IS NOT DISTINCT FROM $4::timestamptz THEN GREATEST(0,weekly_usage_usd+$1) ELSE weekly_usage_usd END,
            monthly_usage_usd=CASE WHEN monthly_window_start IS NOT DISTINCT FROM $5::timestamptz THEN GREATEST(0,monthly_usage_usd+$1) ELSE monthly_usage_usd END,
            updated_at=NOW() WHERE id=$2`, delta, c.SubscriptionID, c.DailyWindowStart, c.WeeklyWindowStart, c.MonthlyWindowStart)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return service.ErrSubscriptionNotFound
		}
	}
	c.ChargedCost = target
	return nil
}
func canvasSettlementCost(c *service.CanvasCharge) float64 {
	// A successful upstream without a usage callback is not a free request. The
	// published precharge is retained, and any late metering reconciles it.
	if c.UsageRecorded {
		return c.ActualCost
	}
	return c.ReservedCost
}
func (r *usageBillingRepository) FinishCanvasCharge(ctx context.Context, id string, success bool) (*service.CanvasCharge, error) {
	return r.finishCanvasCharge(ctx, id, success, false)
}
func (r *usageBillingRepository) FailExpiredCanvasCharge(ctx context.Context, id string) (*service.CanvasCharge, error) {
	return r.finishCanvasCharge(ctx, id, false, true)
}
func (r *usageBillingRepository) finishCanvasCharge(ctx context.Context, id string, success, expiredOnly bool) (*service.CanvasCharge, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	query := `SELECT ` + canvasChargeColumns + ` FROM canvas_run_charges WHERE id=$1`
	if expiredOnly {
		query += ` AND status='reserved' AND lease_until <= NOW()`
	}
	c, err := scanCanvasCharge(tx.QueryRowContext(ctx, query+` FOR UPDATE`, id))
	if err != nil {
		return nil, err
	}
	if c == nil {
		if expiredOnly {
			return nil, nil
		}
		return nil, service.ErrCanvasChargeUnavailable
	}
	if c.Status != "reserved" {
		return c, nil
	}
	target := 0.0
	c.Status = "failed"
	if success {
		target = canvasSettlementCost(c)
		c.Status = "succeeded"
	}
	if err = adjustCanvasMoney(ctx, tx, c, target, true); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE canvas_run_charges SET status=$2,charged_cost=$3,updated_at=NOW() WHERE id=$1`, id, c.Status, c.ChargedCost); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return c, nil
}
func (r *usageBillingRepository) RenewCanvasCharge(ctx context.Context, id string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE canvas_run_charges SET lease_until=NOW()+INTERVAL '3 minutes',updated_at=NOW() WHERE id=$1 AND status='reserved' AND lease_until>NOW()`, id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}
func (r *usageBillingRepository) ExpiredCanvasCharges(ctx context.Context) ([]service.CanvasCharge, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+canvasChargeColumns+` FROM canvas_run_charges WHERE status='reserved' AND lease_until<=NOW() ORDER BY lease_until LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []service.CanvasCharge
	for rows.Next() {
		c, err := scanCanvasCharge(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *c)
	}
	return result, rows.Err()
}

// Called inside the same transaction as usage_billing_dedup. Failed canvas runs
// remain refunded even when a queue worker reports metering after the failure.
func (r *usageBillingRepository) recordCanvasUsage(ctx context.Context, tx *sql.Tx, cmd *service.UsageBillingCommand) (*service.CanvasCharge, error) {
	c, err := scanCanvasCharge(tx.QueryRowContext(ctx, `SELECT `+canvasChargeColumns+` FROM canvas_run_charges WHERE id=$1 FOR UPDATE`, cmd.CanvasChargeID))
	if err != nil {
		return nil, err
	}
	if c == nil || c.UserID != cmd.UserID || c.APIKeyID != cmd.APIKeyID {
		return nil, service.ErrCanvasChargeUnavailable
	}
	if cmd.SubscriptionCost > 0 && (c.SubscriptionID == nil || cmd.SubscriptionID == nil || *c.SubscriptionID != *cmd.SubscriptionID) {
		return nil, fmt.Errorf("canvas subscription identity mismatch")
	}
	if cmd.BalanceCost > 0 && c.SubscriptionID != nil {
		return nil, fmt.Errorf("canvas billing mode mismatch")
	}
	amount := cmd.BalanceCost + cmd.SubscriptionCost
	if amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, service.ErrCanvasChargeUnavailable
	}
	c.ActualCost = service.QuantizeUsageBillingAmount(c.ActualCost + amount)
	c.UsageRecorded = true
	if c.Status == "succeeded" {
		if err = adjustCanvasMoney(ctx, tx, c, canvasSettlementCost(c), false); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE canvas_run_charges SET actual_cost=$2,charged_cost=$3,usage_recorded=TRUE,updated_at=NOW() WHERE id=$1`, c.ID, c.ActualCost, c.ChargedCost)
	return c, err
}
