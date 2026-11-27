package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type balanceHistoryRepo struct {
	UserBalanceSnapshotRepository
	samples []BalanceObservation
	live    BalanceTotals
	reads   int
}

func (r *balanceHistoryRepo) ListBalanceObservations(_ context.Context, start, end time.Time) ([]BalanceObservation, error) {
	var points []BalanceObservation
	for _, p := range r.samples {
		if p.ObservedOn >= start.Format(time.DateOnly) && p.ObservedOn <= end.Format(time.DateOnly) {
			points = append(points, p)
		}
	}
	return points, nil
}

func (r *balanceHistoryRepo) CurrentBalanceTotals(context.Context) (*BalanceTotals, error) {
	r.reads++
	return &r.live, nil
}

func TestBalanceHistoryUsesLastInRangeAndPreservesGaps(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	date := func(month time.Month, day int) time.Time { return time.Date(2026, month, day, 0, 0, 0, 0, loc) }
	sample := func(day string, balance float64) BalanceObservation {
		return BalanceObservation{ObservedOn: day, Source: "daily", TotalBalance: &balance}
	}
	repo := &balanceHistoryRepo{samples: []BalanceObservation{
		sample("2026-08-28", 99), sample("2026-08-29", 12), sample("2026-08-31", 0),
		sample("2026-09-02", 20), sample("2026-09-07", 31),
	}, live: BalanceTotals{TotalBalance: 35, TotalSubscriptionRemaining: 6, AsOf: date(9, 7).Add(4 * time.Hour).UTC()}}
	svc := NewUserBalanceSnapshotService(repo)
	svc.now = func() time.Time { return repo.live.AsOf }
	start, end := date(8, 29), date(9, 7) // Saturday through the following Monday
	ctx := context.Background()
	days, err := svc.GetHistory(ctx, start, end, "day")
	require.NoError(t, err)
	require.Len(t, days, 10)
	require.Equal(t, "missing", days[1].Source)
	require.Nil(t, days[1].TotalBalance)
	require.Zero(t, *days[2].TotalBalance)
	require.Nil(t, days[2].TotalSubscriptionRemaining)
	require.True(t, days[9].Realtime)
	require.Equal(t, 6.0, *days[9].TotalSubscriptionRemaining)
	require.Equal(t, 35.0, *days[9].TotalBalance)

	weeks, err := svc.GetHistory(ctx, start, end, "week")
	require.NoError(t, err)
	require.Len(t, weeks, 3)
	require.Equal(t, "2026-08-30", weeks[0].EndDate)
	require.Equal(t, "2026-08-29", weeks[0].ObservedOn)
	require.Equal(t, "2026-08-31", weeks[1].StartDate)
	require.Equal(t, "2026-09-06", weeks[1].EndDate)
	require.Equal(t, 20.0, *weeks[1].TotalBalance)
	require.True(t, weeks[2].Realtime)

	reads := repo.reads
	months, err := svc.GetHistory(ctx, start, date(9, 5), "month")
	require.NoError(t, err)
	require.Len(t, months, 2)
	require.Equal(t, "2026-08", months[0].Period)
	require.Zero(t, *months[0].TotalBalance)
	require.Equal(t, 20.0, *months[1].TotalBalance)
	period, err := svc.GetHistory(ctx, start, date(9, 1), "range")
	require.NoError(t, err)
	require.Len(t, period, 1)
	require.Equal(t, "2026-08-31", period[0].ObservedOn)
	require.Zero(t, *period[0].TotalBalance)
	require.False(t, period[0].Realtime)
	require.Equal(t, reads, repo.reads, "a purely historical range must not consult current balances")
}
