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
	// The subscription quota may already have reset at midnight. Only copy
	// a closing observation from the previous day into the legacy summary.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO balance_monthly_stats (period, total_balance, total_subscription_remaining, created_at)
		SELECT $1, total_balance, total_subscription_remaining, captured_at FROM balance_daily_stats
		WHERE day = $2::date AND captured_at >= $3 AND captured_at < $4
		ON CONFLICT (period) DO NOTHING`, period, monthStart.AddDate(0, 0, -1).Format(time.DateOnly), monthStart.Add(-time.Minute), monthStart)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (repo *userBalanceSnapshotRepository) CaptureDailyBalance(ctx context.Context, loc *time.Location) error {
	tx, err := repo.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var locked bool
	if err := tx.QueryRowContext(ctx, `SELECT pg_try_advisory_xact_lock(hashtext('balance_daily_stats:capture'))`).Scan(&locked); err != nil || !locked {
		return err
	}
	totals, err := readOperationsBalanceTotals(ctx, tx)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO balance_daily_stats (day, captured_at, total_balance, total_subscription_remaining)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (day) DO UPDATE SET captured_at = EXCLUDED.captured_at,
			total_balance = EXCLUDED.total_balance, total_subscription_remaining = EXCLUDED.total_subscription_remaining
		WHERE balance_daily_stats.captured_at < EXCLUDED.captured_at`,
		totals.AsOf.In(loc).Format(time.DateOnly), totals.AsOf, totals.TotalBalance, totals.TotalSubscriptionRemaining)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (repo *userBalanceSnapshotRepository) ListBalanceObservations(ctx context.Context, start, end time.Time) ([]service.BalanceObservation, error) {
	// Older deployments may have only the first-day user snapshots from
	// migration 242. A wallet unchanged since before the cutoff also has a
	// known closing balance. Require evidence for every user alive then.
	const query = `WITH months AS (
		SELECT to_char(m, 'YYYY-MM') AS period, (m + interval '1 month')::date - 1 AS day,
			(m + interval '1 month') AT TIME ZONE $3 AS cutoff,
			(m + interval '1 month 1 day') AT TIME ZONE $3 AS next_day
		FROM generate_series(date_trunc('month', $1::date::timestamp),
			date_trunc('month', $2::date::timestamp), interval '1 month') AS m
	), legacy AS (
		SELECT m.day, s.total_balance, s.total_subscription_remaining
		FROM months m JOIN balance_monthly_stats s USING (period)
	), recoverable_months AS (
		SELECT m.* FROM months m
		WHERE m.day BETWEEN $1::date AND $2::date
		  AND m.cutoff <= now()
		  AND NOT EXISTS (SELECT 1 FROM legacy WHERE day = m.day)
		  AND NOT EXISTS (SELECT 1 FROM balance_daily_stats d WHERE d.day = m.day)
		  AND NOT EXISTS (
			SELECT 1 FROM users u WHERE u.created_at < m.cutoff AND (u.deleted_at IS NULL OR u.deleted_at >= m.cutoff)
			AND u.updated_at >= m.cutoff AND NOT EXISTS (
				SELECT 1 FROM user_balance_snapshots s WHERE s.user_id = u.id AND s.period = m.period
				AND s.created_at >= m.cutoff AND s.created_at < m.next_day
			)
		  )
	), recovered AS (
		SELECT m.day, SUM(COALESCE(s.balance, u.balance)) AS total_balance
		FROM recoverable_months m
		JOIN users u ON u.created_at < m.cutoff AND (u.deleted_at IS NULL OR u.deleted_at >= m.cutoff)
		LEFT JOIN user_balance_snapshots s ON s.user_id = u.id AND s.period = m.period AND s.created_at >= m.cutoff AND s.created_at < m.next_day
		GROUP BY m.day
	), observations AS (
		SELECT day, total_balance, total_subscription_remaining, captured_at, 'daily' AS source,
			CASE WHEN captured_at >= ((day + 1)::timestamp AT TIME ZONE $3) - interval '1 minute' THEN 0 ELSE 2 END AS priority
		FROM balance_daily_stats WHERE day BETWEEN $1::date AND $2::date
		UNION ALL
		SELECT day, total_balance, total_subscription_remaining, NULL, 'monthly', 1 FROM legacy
		UNION ALL
		SELECT day, total_balance, NULL, NULL, 'wallet_snapshot', 3 FROM recovered
	)
	SELECT DISTINCT ON (day) to_char(day, 'YYYY-MM-DD'), total_balance, total_subscription_remaining, captured_at, source
	FROM observations WHERE day BETWEEN $1::date AND $2::date ORDER BY day, priority`
	rows, err := repo.db.QueryContext(ctx, query, start.Format(time.DateOnly), end.Format(time.DateOnly), start.Location().String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := make([]service.BalanceObservation, 0)
	for rows.Next() {
		var point service.BalanceObservation
		if err := rows.Scan(&point.ObservedOn, &point.TotalBalance, &point.TotalSubscriptionRemaining, &point.AsOf, &point.Source); err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, rows.Err()
}

func (repo *userBalanceSnapshotRepository) CurrentBalanceTotals(ctx context.Context) (*service.BalanceTotals, error) {
	return readOperationsBalanceTotals(ctx, repo.db)
}

func readOperationsBalanceTotals(ctx context.Context, executor sqlExecutor) (*service.BalanceTotals, error) {
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
	var totals service.BalanceTotals
	var subscriptions []service.UserSubscription
	var metadata []byte
	if err := rows.Scan(&totals.TotalBalance, &metadata, &totals.AsOf); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(metadata, &subscriptions); err != nil {
		return nil, err
	}
	var inventory service.OperationsInventory
	for _, sub := range subscriptions {
		if sub.Group != nil {
			inventory.AddSubscription(sub, *sub.Group, totals.AsOf)
		}
	}
	totals.TotalSubscriptionRemaining = inventory.SubscriptionRemaining
	return &totals, rows.Err()
}
