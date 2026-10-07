package repository

import (
	"context"
	"database/sql"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/userbalancesnapshot"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userBalanceSnapshotRepository struct {
	client *dbent.Client
	db     *sql.DB
}

func NewUserBalanceSnapshotRepository(client *dbent.Client, db *sql.DB) service.UserBalanceSnapshotRepository {
	return &userBalanceSnapshotRepository{client: client, db: db}
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
	_, err := repo.db.ExecContext(ctx, `
		INSERT INTO user_balance_snapshots (user_id, period, balance, created_at)
		SELECT id, $1, balance, $2 FROM users
		WHERE deleted_at IS NULL AND created_at < $3
		ON CONFLICT (user_id, period) DO UPDATE
		SET balance = EXCLUDED.balance, created_at = EXCLUDED.created_at
		WHERE user_balance_snapshots.created_at < $3`, monthStart.AddDate(0, -1, 0).Format("2006-01"), now, monthStart)
	return err
}

func (repo *userBalanceSnapshotRepository) ListHistory(ctx context.Context, userID int64, fromPeriod, toPeriod string) ([]service.BalanceHistoryPoint, error) {
	snapshots, err := repo.client.UserBalanceSnapshot.Query().Where(
		userbalancesnapshot.UserIDEQ(userID),
		userbalancesnapshot.PeriodGTE(fromPeriod),
		userbalancesnapshot.PeriodLTE(toPeriod),
	).Order(dbent.Asc(userbalancesnapshot.FieldPeriod)).All(ctx)
	if err != nil {
		return nil, err
	}
	points := make([]service.BalanceHistoryPoint, 0, len(snapshots))
	for _, snapshot := range snapshots {
		points = append(points, service.BalanceHistoryPoint{Period: snapshot.Period, Balance: snapshot.Balance})
	}
	return points, nil
}

func (repo *userBalanceSnapshotRepository) GetCurrentBalance(ctx context.Context, userID int64) (float64, error) {
	currentUser, err := repo.client.User.Query().Where(user.IDEQ(userID), user.DeletedAtIsNil()).Select(user.FieldBalance).Only(ctx)
	if dbent.IsNotFound(err) {
		return 0, service.ErrUserNotFound
	}
	if err != nil {
		return 0, err
	}
	return currentUser.Balance, nil
}
