package rechargecampaigns

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"github.com/shopspring/decimal"
)

// SQLInviterLookup is shared by the checkout adapter and isolated SQL tests.
// The query preserves historical eligibility, including unknown binding times.
type SQLInviterLookup struct{ db Queryer }

func NewSQLInviterLookup(db Queryer) *SQLInviterLookup { return &SQLInviterLookup{db: db} }

func (r *SQLInviterLookup) EligibleInviter(ctx context.Context, userID int64, a Campaign) (int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT af.inviter_id FROM user_affiliates af JOIN users u ON u.id=af.inviter_id WHERE af.user_id=$1 AND af.inviter_id<>$1 AND u.status='active' AND u.deleted_at IS NULL AND ($2=FALSE OR (af.invited_at >= $3 AND af.invited_at < $4))`, userID, a.NewInviteesOnly, a.StartsAt, a.EndsAt)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var inviterID int64
	if rows.Next() {
		err = rows.Scan(&inviterID)
	}
	if err != nil {
		return 0, err
	}
	return inviterID, rows.Err()
}

// RewardAccruer receives the host transaction context. The host retains the
// order/audit idempotency claim and commit/rollback; this module never commits.
type RewardAccruer interface {
	AccrueQuota(context.Context, int64, int64, float64, int, *int64) (bool, error)
}

func AccrueReward(ctx context.Context, repo RewardAccruer, snap *Snapshot, orderID, userID int64) (float64, error) {
	if snap == nil || snap.InviterID <= 0 || snap.Reward <= 0 {
		return 0, nil
	}
	if repo == nil {
		return 0, fmt.Errorf("campaign reward repository unavailable")
	}
	applied, err := repo.AccrueQuota(ctx, snap.InviterID, userID, snap.Reward, snap.Campaign.FreezeHours, &orderID)
	if err != nil {
		return 0, err
	}
	if !applied {
		return 0, fmt.Errorf("campaign inviter %d unavailable", snap.InviterID)
	}
	return snap.Reward, nil
}

// RewardTransaction must be the SAME transaction that changes the order status.
// It intentionally has no Begin/Commit API. The refund host is responsible for
// serialization and duplicate-finalization guards, not a second plugin lock.
type RewardTransaction interface {
	Queryer
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type Refund struct {
	Snapshot     *Snapshot
	OrderID      int64
	UserID       int64
	OrderAmount  float64
	RefundAmount float64
}

// ReverseReward consumes an immutable snapshot even when new campaigns or
// affiliate rewards are disabled. Negative available quota records transferred
// reward debt; frozen rewards keep the ledger timestamp for native thaw logic.
func ReverseReward(ctx context.Context, tx RewardTransaction, refund Refund) error {
	if refund.Snapshot == nil || refund.Snapshot.Reward <= 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, "UPDATE user_affiliates SET updated_at=updated_at WHERE user_id=$1", refund.Snapshot.InviterID); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,user_id,amount,frozen_until FROM user_affiliate_ledger WHERE source_order_id=$1 AND action='accrue' FOR UPDATE`, refund.OrderID)
	if err != nil {
		return err
	}
	var id, uid int64
	var amount float64
	var frozenUntil sql.NullTime
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		return err
	}
	err = rows.Scan(&id, &uid, &amount, &frozenUntil)
	rows.Close()
	if err != nil {
		return err
	}
	reversal := decimal.NewFromFloat(amount).Mul(decimal.NewFromFloat(math.Min(1, refund.RefundAmount/refund.OrderAmount))).Round(8).InexactFloat64()
	if reversal <= 0 {
		return nil
	}
	column := "aff_quota"
	if frozenUntil.Valid {
		column = "aff_frozen_quota"
	}
	if _, err = tx.ExecContext(ctx, fmt.Sprintf("UPDATE user_affiliates SET %s=%s-$1,aff_history_quota=aff_history_quota-$1,updated_at=NOW() WHERE user_id=$2", column, column), reversal, uid); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,source_order_id,frozen_until,created_at,updated_at) VALUES($1,'campaign_refund',$2,$3,$4,$5,NOW(),NOW())`, uid, -reversal, refund.UserID, refund.OrderID, frozenUntil)
	return err
}
