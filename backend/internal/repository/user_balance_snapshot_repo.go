package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userBalanceSnapshotRepository struct {
	db *sql.DB
}

func NewUserBalanceSnapshotRepository(db *sql.DB) service.UserBalanceSnapshotRepository {
	return &userBalanceSnapshotRepository{db: db}
}

func (repo *userBalanceSnapshotRepository) EnsureCurrentMonthSnapshot(ctx context.Context, userID int64, now time.Time) error {
	result, err := repo.db.ExecContext(ctx, `
		INSERT INTO user_balance_snapshots (user_id, period, balance, created_at)
		SELECT id, $2, balance, $3 FROM users WHERE id = $1 AND deleted_at IS NULL
		ON CONFLICT (user_id, period) DO UPDATE
		SET balance = EXCLUDED.balance, created_at = EXCLUDED.created_at`, userID, now.Format("2006-01"), now)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func (repo *userBalanceSnapshotRepository) SnapshotPreviousMonth(ctx context.Context, now time.Time) error {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	period := monthStart.AddDate(0, -1, 0).Format("2006-01")
	tx, err := repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO user_balance_snapshots (user_id, period, balance, created_at)
		SELECT id, $1, balance, $2 FROM users
		WHERE deleted_at IS NULL AND created_at < $3
		ON CONFLICT (user_id, period) DO UPDATE
		SET balance = EXCLUDED.balance, created_at = EXCLUDED.created_at
		WHERE user_balance_snapshots.created_at < $3`, period, now, monthStart)
	if err != nil {
		return err
	}
	totals, err := readOperationsBalanceTotals(ctx, tx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO balance_monthly_stats (period, total_balance, total_subscription_remaining, created_at)
		SELECT $1, COALESCE(SUM(balance), 0), $2, $3 FROM user_balance_snapshots WHERE period = $1
		ON CONFLICT (period) DO NOTHING`, period, totals.TotalSubscriptionRemaining, now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (repo *userBalanceSnapshotRepository) ListMonthlyStats(ctx context.Context, fromPeriod, toPeriod string) ([]service.BalanceMonthlyStats, error) {
	rows, err := repo.db.QueryContext(ctx, `
		SELECT period, total_balance, total_subscription_remaining FROM balance_monthly_stats
		WHERE period >= $1 AND period <= $2 ORDER BY period ASC`, fromPeriod, toPeriod)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := make([]service.BalanceMonthlyStats, 0)
	for rows.Next() {
		var point service.BalanceMonthlyStats
		if err := rows.Scan(&point.Period, &point.TotalBalance, &point.TotalSubscriptionRemaining); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (repo *userBalanceSnapshotRepository) GetCurrentMonthlyStats(ctx context.Context) (*service.BalanceMonthlyStats, error) {
	return readOperationsBalanceTotals(ctx, repo.db)
}

func readOperationsBalanceTotals(ctx context.Context, executor sqlExecutor) (*service.BalanceMonthlyStats, error) {
	const query = `WITH inventory_subscriptions AS (` + operationsInventorySubscriptionsQuery + `)
		SELECT (SELECT COALESCE(SUM(balance),0) FROM users WHERE deleted_at IS NULL),
		COALESCE((SELECT jsonb_agg(data) FROM inventory_subscriptions),'[]'::jsonb), now()`
	rows, err := executor.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}
	var totals service.BalanceMonthlyStats
	var subscriptions []service.UserSubscription
	var metadata []byte
	var asOf time.Time
	if err := rows.Scan(&totals.TotalBalance, &metadata, &asOf); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(metadata, &subscriptions); err != nil {
		return nil, err
	}
	var inventory service.OperationsInventory
	for _, sub := range subscriptions {
		if sub.Group != nil {
			inventory.AddSubscription(sub, *sub.Group, asOf)
		}
	}
	totals.TotalSubscriptionRemaining = inventory.SubscriptionRemaining
	return &totals, rows.Err()
}
