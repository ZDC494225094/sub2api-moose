//go:build unit

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type balanceSnapshotRepoStub struct {
	points       []BalanceHistoryPoint
	balance      float64
	err          error
	balanceCalls int
	userID       int64
	fromPeriod   string
	toPeriod     string
	ensuredAt    time.Time
}

func (repo *balanceSnapshotRepoStub) EnsureCurrentMonthSnapshot(_ context.Context, userID int64, now time.Time) error {
	repo.userID = userID
	repo.ensuredAt = now
	return repo.err
}

func (repo *balanceSnapshotRepoStub) SnapshotPreviousMonth(context.Context, time.Time) error {
	return repo.err
}

func (repo *balanceSnapshotRepoStub) ListHistory(_ context.Context, userID int64, fromPeriod, toPeriod string) ([]BalanceHistoryPoint, error) {
	repo.userID, repo.fromPeriod, repo.toPeriod = userID, fromPeriod, toPeriod
	return append([]BalanceHistoryPoint(nil), repo.points...), repo.err
}

func (repo *balanceSnapshotRepoStub) GetCurrentBalance(context.Context, int64) (float64, error) {
	repo.balanceCalls++
	return repo.balance, repo.err
}

func TestBalanceHistoryMonthRanges(t *testing.T) {
	for _, test := range []struct {
		months int
		from   string
	}{
		{3, "2026-01"}, {6, "2025-10"}, {12, "2025-04"},
		{0, "2025-04"}, {1, "2025-04"}, {-3, "2025-04"}, {24, "2025-04"},
	} {
		t.Run(strconv.Itoa(test.months), func(t *testing.T) {
			repo := &balanceSnapshotRepoStub{balance: 12.5}
			svc := NewUserBalanceSnapshotService(repo)
			svc.now = func() time.Time { return time.Date(2026, 3, 31, 23, 0, 0, 0, time.FixedZone("test", 8*3600)) }
			points, err := svc.GetHistory(context.Background(), 42, test.months)
			require.NoError(t, err)
			require.Equal(t, int64(42), repo.userID)
			require.Equal(t, test.from, repo.fromPeriod)
			require.Equal(t, "2026-03", repo.toPeriod)
			require.Equal(t, []BalanceHistoryPoint{{Period: "2026-03", Balance: 12.5, Realtime: true}}, points)
		})
	}
}

func TestBalanceHistorySkipsMissingMonthsAndPreservesCurrentSnapshot(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name         string
		stored       []BalanceHistoryPoint
		want         []BalanceHistoryPoint
		balanceCalls int
	}{
		{
			name:         "missing months are not filled",
			stored:       []BalanceHistoryPoint{{Period: "2026-08", Balance: 10}},
			want:         []BalanceHistoryPoint{{Period: "2026-08", Balance: 10}, {Period: "2026-10", Balance: -2.5, Realtime: true}},
			balanceCalls: 1,
		},
		{
			name:   "current snapshot is not replaced with live balance",
			stored: []BalanceHistoryPoint{{Period: "2026-08", Balance: 10}, {Period: "2026-10", Balance: 5}},
			want:   []BalanceHistoryPoint{{Period: "2026-08", Balance: 10}, {Period: "2026-10", Balance: 5}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &balanceSnapshotRepoStub{points: test.stored, balance: -2.5}
			svc := NewUserBalanceSnapshotService(repo)
			svc.now = func() time.Time { return now }
			points, err := svc.GetHistory(context.Background(), 42, 3)
			require.NoError(t, err)
			require.Equal(t, test.want, points)
			require.Equal(t, test.balanceCalls, repo.balanceCalls)
		})
	}
}

func TestBalanceHistoryPropagatesRepositoryError(t *testing.T) {
	repo := &balanceSnapshotRepoStub{err: errors.New("database unavailable")}
	_, err := NewUserBalanceSnapshotService(repo).GetHistory(context.Background(), 42, 12)
	require.ErrorIs(t, err, repo.err)
	require.Zero(t, repo.balanceCalls)
}

func TestEnsureCurrentMonthSnapshotUsesConfiguredTime(t *testing.T) {
	repo := &balanceSnapshotRepoStub{}
	svc := NewUserBalanceSnapshotService(repo)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("test", 8*3600))
	svc.now = func() time.Time { return now }
	require.NoError(t, svc.EnsureCurrentMonthSnapshot(context.Background(), 42))
	require.Equal(t, int64(42), repo.userID)
	require.Equal(t, now, repo.ensuredAt)
}

func TestBalanceSnapshotCronRunsOnFirstAt0005(t *testing.T) {
	svc := NewUserBalanceSnapshotService(&balanceSnapshotRepoStub{})
	svc.Start()
	t.Cleanup(svc.Stop)
	entries := svc.cronSched.Entries()
	require.Len(t, entries, 1)
	now := time.Date(2026, 10, 31, 23, 59, 59, 0, timezone.Location())
	require.Equal(t, time.Date(2026, 11, 1, 0, 5, 0, 0, timezone.Location()), entries[0].Schedule.Next(now))
}
