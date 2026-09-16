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
		{Dimension: "day", Consumption: 10, Cost: 5, Recharge: 100, Requests: 1},
		{Dimension: "day", Consumption: 90, Cost: 85, Recharge: 200, Subscription: 200, Requests: 2},
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
	require.Equal(t, 300.0, result.Summary.Recharge)
	require.Equal(t, 200.0, result.Summary.Subscription)
}
