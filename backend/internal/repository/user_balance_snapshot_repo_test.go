//go:build unit

package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newBalanceSnapshotTestRepository(t *testing.T) (service.UserBalanceSnapshotRepository, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	_, err = db.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY, balance REAL NOT NULL, created_at DATETIME NOT NULL, deleted_at DATETIME);
		CREATE TABLE user_balance_snapshots (
			id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, period TEXT NOT NULL,
			balance REAL NOT NULL, created_at DATETIME NOT NULL, UNIQUE(user_id, period)
		)`)
	require.NoError(t, err)
	return NewUserBalanceSnapshotRepository(client, db), db
}

func TestEnsureCurrentMonthBalanceSnapshotUpsert(t *testing.T) {
	repo, db := newBalanceSnapshotTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 10, 31, 23, 59, 0, 0, time.UTC)
	_, err := db.Exec(`INSERT INTO users (id, balance, created_at) VALUES (42, 12.5, $1)`, now.AddDate(0, -1, 0))
	require.NoError(t, err)
	require.NoError(t, repo.EnsureCurrentMonthSnapshot(ctx, 42, now))
	_, err = db.Exec(`UPDATE users SET balance = -2.25 WHERE id = 42`)
	require.NoError(t, err)
	require.NoError(t, repo.EnsureCurrentMonthSnapshot(ctx, 42, now))
	require.NoError(t, repo.EnsureCurrentMonthSnapshot(ctx, 42, now))
	points, err := repo.ListHistory(ctx, 42, "2026-10", "2026-10")
	require.NoError(t, err)
	require.Equal(t, []service.BalanceHistoryPoint{{Period: "2026-10", Balance: -2.25}}, points)
	balance, err := repo.GetCurrentBalance(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, -2.25, balance)
	require.ErrorIs(t, repo.EnsureCurrentMonthSnapshot(ctx, 999, now), service.ErrUserNotFound)
	_, err = db.Exec(`UPDATE users SET deleted_at = $1 WHERE id = 42`, now)
	require.NoError(t, err)
	require.ErrorIs(t, repo.EnsureCurrentMonthSnapshot(ctx, 42, now), service.ErrUserNotFound)
	_, err = repo.GetCurrentBalance(ctx, 42)
	require.ErrorIs(t, err, service.ErrUserNotFound)
}

func TestSnapshotPreviousMonthFinalizesOnceAndPreservesBoundaryCapture(t *testing.T) {
	for _, test := range []struct {
		name       string
		capturedAt time.Time
		want       float64
	}{
		{"updates provisional snapshot", time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC), 9},
		{"preserves month-end balance before new-month spending", time.Date(2027, 1, 1, 0, 1, 0, 0, time.UTC), 17},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo, db := newBalanceSnapshotTestRepository(t)
			ctx := context.Background()
			now := time.Date(2027, 1, 1, 0, 5, 0, 0, time.UTC)
			_, err := db.Exec(`INSERT INTO users (id, balance, created_at) VALUES (42, 9, $1), (43, 3, $1), (44, 100, $2)`, now.AddDate(0, -1, 0), now)
			require.NoError(t, err)
			_, err = db.Exec(`INSERT INTO user_balance_snapshots (user_id, period, balance, created_at) VALUES (42, '2026-12', 17, $1)`, test.capturedAt)
			require.NoError(t, err)
			require.NoError(t, repo.SnapshotPreviousMonth(ctx, now))
			_, err = db.Exec(`UPDATE users SET balance = 1 WHERE id = 42`)
			require.NoError(t, err)
			require.NoError(t, repo.SnapshotPreviousMonth(ctx, now.Add(time.Minute)))
			points, err := repo.ListHistory(ctx, 42, "2026-12", "2027-01")
			require.NoError(t, err)
			require.Equal(t, []service.BalanceHistoryPoint{{Period: "2026-12", Balance: test.want}}, points)
			points, err = repo.ListHistory(ctx, 43, "2026-12", "2027-01")
			require.NoError(t, err)
			require.Equal(t, []service.BalanceHistoryPoint{{Period: "2026-12", Balance: 3}}, points)
			points, err = repo.ListHistory(ctx, 44, "2026-12", "2027-01")
			require.NoError(t, err)
			require.Empty(t, points)
		})
	}
}

func TestBalanceSnapshotHistoryFiltersUserAndRangeAndSorts(t *testing.T) {
	repo, db := newBalanceSnapshotTestRepository(t)
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	_, err := db.Exec(`INSERT INTO user_balance_snapshots (user_id, period, balance, created_at) VALUES
		(42, '2026-10', 10, $1), (42, '2026-08', 8, $1), (42, '2026-07', 7, $1),
		(42, '2026-11', 11, $1), (43, '2026-09', 99, $1)`, now)
	require.NoError(t, err)
	points, err := repo.ListHistory(context.Background(), 42, "2026-08", "2026-10")
	require.NoError(t, err)
	require.Equal(t, []service.BalanceHistoryPoint{{Period: "2026-08", Balance: 8}, {Period: "2026-10", Balance: 10}}, points)
}
