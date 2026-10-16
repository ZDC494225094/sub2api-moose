// Package redeemmarketing owns marketing-code eligibility and conversion reporting.
package redeemmarketing

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const ID = "redeem-marketing"

type Database interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type Options struct {
	NewUsersOnly bool `json:"new_users_only"`
}

// Missing or corrupt state fails closed, without gating historical eligibility.
func RequireEnabled(ctx context.Context, db Database) error {
	rows, err := db.QueryContext(ctx, "SELECT value FROM settings WHERE key = $1", customize.Key(ID))
	if err != nil {
		return err
	}
	defer rows.Close()
	var value string
	if rows.Next() {
		if err := rows.Scan(&value); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if value != "true" {
		return customize.Disabled(ID)
	}
	return nil
}

func SavePolicy(ctx context.Context, db Database, batch string, cutoff time.Time) error {
	_, err := db.ExecContext(ctx, `INSERT INTO custom_redeem_marketing_batches (batch_id, registered_after) VALUES ($1, $2)`, batch, cutoff)
	return err
}

// The batch policy is immutable and must survive disabling the extension.
func CheckEligibility(ctx context.Context, db Database, batch string, registeredAt time.Time) error {
	rows, err := db.QueryContext(ctx, `SELECT registered_after FROM custom_redeem_marketing_batches WHERE batch_id = $1`, batch)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		var cutoff time.Time
		if err := rows.Scan(&cutoff); err != nil {
			return err
		}
		if !Eligible(registeredAt, cutoff) {
			return infraerrors.Forbidden("MARKETING_REDEEM_NEW_USERS_ONLY", "该营销兑换码仅限本批次生成后新注册的用户使用")
		}
	}
	return rows.Err()
}

func Eligible(registeredAt, cutoff time.Time) bool { return !registeredAt.Before(cutoff) }

type Batch struct {
	BatchID         string             `json:"batch_id"`
	CreatedAt       time.Time          `json:"created_at"`
	RegisteredAfter *time.Time         `json:"registered_after"`
	CodeCount       int64              `json:"code_count"`
	RedeemedUsers   int64              `json:"redeemed_users"`
	RedeemedValue   float64            `json:"redeemed_value"`
	PayingUsers     int64              `json:"paying_users"`
	OrderCount      int64              `json:"order_count"`
	PayAmounts      map[string]float64 `json:"pay_amounts"`
}

// Aggregate codes and cohort orders separately: joining raw codes to orders
// would multiply both the issued-code count and revenue. Each batch is an
// independent cohort; the same buyer can belong to more than one batch.
const batchQuery = `WITH batches AS (
 SELECT batch_id, MIN(created_at) created_at, COUNT(*) code_count
 FROM redeem_codes WHERE type = 'marketing' AND batch_id IS NOT NULL
 AND ($1 = '' OR batch_id = $1) GROUP BY batch_id
 ORDER BY MIN(created_at) DESC, batch_id DESC LIMIT $2 OFFSET $3
), cohort AS (
 SELECT u.batch_id, u.user_id, MIN(u.used_at) used_at
 FROM redeem_code_batch_usages u JOIN batches b ON b.batch_id = u.batch_id GROUP BY u.batch_id, u.user_id
), redemptions AS (
 SELECT c.batch_id, COUNT(*) redeemed_users, COALESCE(SUM(r.value), 0) redeemed_value
 FROM cohort c JOIN redeem_code_batch_usages u ON u.batch_id = c.batch_id AND u.user_id = c.user_id
 JOIN redeem_codes r ON r.id = u.redeem_code_id GROUP BY c.batch_id
), eligible_orders AS (
 SELECT c.batch_id, c.user_id, o.pay_amount,
 CASE WHEN upper(btrim(o.provider_snapshot->>'currency')) ~ '^[A-Z]{3}$'
 THEN upper(btrim(o.provider_snapshot->>'currency')) ELSE 'CNY' END currency
 FROM cohort c JOIN payment_orders o ON o.user_id = c.user_id
 AND o.paid_at > c.used_at AND o.created_at >= c.used_at
 AND o.status IN ('PAID', 'RECHARGING', 'COMPLETED')
 AND o.pay_amount > 0 AND o.refund_amount = 0
), payments AS (
 SELECT batch_id, COUNT(DISTINCT user_id) paying_users, COUNT(*) order_count
 FROM eligible_orders GROUP BY batch_id
), currency_totals AS (
 SELECT batch_id, currency, SUM(pay_amount) pay_amount FROM eligible_orders GROUP BY batch_id, currency
), amounts AS (
 SELECT batch_id, jsonb_object_agg(currency, pay_amount) pay_amounts FROM currency_totals GROUP BY batch_id
)
SELECT b.batch_id, b.created_at, p.registered_after, b.code_count,
 COALESCE(r.redeemed_users, 0), COALESCE(r.redeemed_value, 0),
 COALESCE(o.paying_users, 0), COALESCE(o.order_count, 0), COALESCE(a.pay_amounts, '{}'::jsonb)
FROM batches b LEFT JOIN custom_redeem_marketing_batches p USING (batch_id)
LEFT JOIN redemptions r USING (batch_id) LEFT JOIN payments o USING (batch_id)
LEFT JOIN amounts a USING (batch_id)
ORDER BY b.created_at DESC, b.batch_id DESC`

func ListBatches(ctx context.Context, db Database, batch string, page, size int) ([]Batch, error) {
	if page < 1 || size < 1 || size > 100 || page-1 > int(^uint(0)>>1)/size {
		return nil, infraerrors.BadRequest("MARKETING_BATCH_PAGINATION_INVALID", "invalid marketing batch pagination")
	}
	if err := RequireEnabled(ctx, db); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, batchQuery, batch, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Batch{}
	for rows.Next() {
		var b Batch
		var amounts []byte
		if err := rows.Scan(&b.BatchID, &b.CreatedAt, &b.RegisteredAfter, &b.CodeCount, &b.RedeemedUsers, &b.RedeemedValue, &b.PayingUsers, &b.OrderCount, &amounts); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(amounts, &b.PayAmounts); err != nil {
			return nil, err
		}
		items = append(items, b)
	}
	return items, rows.Err()
}
