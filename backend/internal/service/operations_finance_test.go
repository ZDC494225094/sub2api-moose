package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type financeRepositoryStub struct {
	UsageLogRepository
	result *OperationsFinanceResponse
}

func (r financeRepositoryStub) GetOperationsFinance(context.Context, time.Time, time.Time) (*OperationsFinanceResponse, error) {
	return r.result, nil
}

func TestOperationsFinanceProfit(t *testing.T) {
	for _, test := range []struct {
		name                      string
		consumption, cost, profit float64
		margin                    *float64
	}{
		{"zero revenue with upstream cost", 0, 3, -3, nil},
		{"empty period", 0, 0, 0, nil},
		{"loss", 10, 15, -5, func() *float64 { v := -50.0; return &v }()},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := OperationsFinanceRow{Consumption: test.consumption, Cost: test.cost}
			calculateOperationsProfit(&row)
			require.Equal(t, test.profit, row.Profit)
			require.Equal(t, test.margin, row.Margin)
		})
	}
}

func TestOperationsFinanceSummaryDoesNotDoubleCountDimensions(t *testing.T) {
	s := &DashboardService{usageRepo: financeRepositoryStub{result: &OperationsFinanceResponse{Rows: []OperationsFinanceRow{
		{Dimension: "day", Consumption: 10, Cost: 5, Recharge: 100, Requests: 1, TotalOrders: 5, PaidOrders: 2, ExcludedRecharge: 99},
		{Dimension: "day", Consumption: 90, Cost: 85, Recharge: 200, Subscription: 200, Requests: 2, TotalOrders: 3, PaidOrders: 1, ExcludedRecharge: 77},
		{Dimension: "upstream", Consumption: 100, Cost: 90, Requests: 3},
		{Dimension: "account", Consumption: 100, Cost: 90, Requests: 3},
	}}}}
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	start := time.Date(2026, 3, 8, 0, 0, 0, 0, loc)
	result, err := s.GetOperationsFinance(context.Background(), start, start.AddDate(0, 0, 1))
	require.NoError(t, err)
	require.Equal(t, "2026-03-08", result.EndDate)
	require.Equal(t, int64(3), result.Summary.Requests)
	require.Equal(t, 100.0, result.Summary.Consumption)
	require.Equal(t, 10.0, result.Summary.Profit)
	require.Equal(t, 10.0, *result.Summary.Margin)
	require.Equal(t, int64(8), result.Summary.TotalOrders)
	require.Equal(t, int64(3), result.Summary.PaidOrders)
	require.Equal(t, 176.0, result.Summary.ExcludedRecharge)
	require.Equal(t, 300.0, result.Summary.Recharge)
	require.Equal(t, 200.0, result.Summary.Subscription)
}

func TestOperationsInventoryRemainingQuota(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	daily, weekly, monthly := 10.0, 50.0, 100.0
	current := now.Add(-time.Hour)
	sub := UserSubscription{Status: SubscriptionStatusActive, StartsAt: now.Add(-10 * 24 * time.Hour), ExpiresAt: now.Add(60 * 24 * time.Hour),
		DailyWindowStart: &current, WeeklyWindowStart: &current, MonthlyWindowStart: &current, DailyUsageUSD: 3, WeeklyUsageUSD: 48, MonthlyUsageUSD: 20}
	group := Group{DailyLimitUSD: &daily, WeeklyLimitUSD: &weekly, MonthlyLimitUSD: &monthly}
	var v OperationsInventory
	v.AddSubscription(sub, group, now)
	require.Equal(t, 2.0, v.SubscriptionRemaining) // min(7,2,80), not their sum
	require.Equal(t, int64(1), v.LimitedSubscriptions)
	require.Nil(t, v.GiftRemaining)
	v.AddSubscription(sub, Group{}, now)
	require.Equal(t, int64(1), v.UnlimitedSubscriptions)
	expired := sub
	expired.ExpiresAt = now
	v.AddSubscription(expired, group, now)
	future := sub
	future.StartsAt = now.Add(time.Hour)
	v.AddSubscription(future, group, now)
	require.Equal(t, int64(1), v.LimitedSubscriptions)
	old := now.Add(-40 * 24 * time.Hour)
	sub.StartsAt = now.Add(-60 * 24 * time.Hour)
	sub.DailyWindowStart = &old
	sub.WeeklyWindowStart = &old
	sub.MonthlyWindowStart = &old
	sub.DailyUsageUSD = 100
	sub.WeeklyUsageUSD = 100
	sub.MonthlyUsageUSD = 100
	var reset OperationsInventory
	reset.AddSubscription(sub, group, now)
	require.Equal(t, 10.0, reset.SubscriptionRemaining) // all stale windows normalized without DB writes
	require.Equal(t, 100.0, sub.DailyUsageUSD)          // read-only copy
	sub.DailyWindowStart = &current
	sub.WeeklyWindowStart = &current
	sub.MonthlyWindowStart = &current
	var exhausted OperationsInventory
	exhausted.AddSubscription(sub, group, now)
	require.Zero(t, exhausted.SubscriptionRemaining)
}
