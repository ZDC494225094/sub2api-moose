package service

import (
	"context"
	"time"
)

// Finance amounts are billing credits (USD), never mixed with payment currencies.
type OperationsFinanceRow struct {
	Dimension    string   `json:"dimension"`
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Upstream     string   `json:"upstream"`
	Requests     int64    `json:"requests"`
	Consumption  float64  `json:"consumption"`
	ListCost     float64  `json:"list_cost"`
	Cost         float64  `json:"cost"`
	Recharge     float64  `json:"recharge"`
	Subscription float64  `json:"subscription"`
	Profit       float64  `json:"profit"`
	Margin       *float64 `json:"margin"`
}

type OperationsFinanceResponse struct {
	StartDate      string                 `json:"start_date"`
	EndDate        string                 `json:"end_date"`
	GeneratedAt    string                 `json:"generated_at"`
	Timezone       string                 `json:"timezone"`
	CurrentBalance float64                `json:"current_balance"`
	Summary        OperationsFinanceRow   `json:"summary"`
	Rows           []OperationsFinanceRow `json:"rows"`
}

type operationsFinanceReader interface {
	GetOperationsFinance(context.Context, time.Time, time.Time) (*OperationsFinanceResponse, error)
}

func (s *DashboardService) GetOperationsFinance(ctx context.Context, start, end time.Time) (*OperationsFinanceResponse, error) {
	r, ok := s.usageRepo.(operationsFinanceReader)
	if !ok {
		return nil, ErrOperationsFunnelUnsupported
	}
	result, err := r.GetOperationsFinance(ctx, start, end)
	if err != nil {
		return nil, err
	}
	result.StartDate = start.Format("2006-01-02")
	result.EndDate = end.AddDate(0, 0, -1).Format("2006-01-02")
	result.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	result.Timezone = start.Location().String()
	for i := range result.Rows {
		row := &result.Rows[i]
		calculateOperationsProfit(row)
		if row.Dimension == "day" {
			result.Summary.Requests += row.Requests
			result.Summary.Consumption += row.Consumption
			result.Summary.ListCost += row.ListCost
			result.Summary.Cost += row.Cost
			result.Summary.Recharge += row.Recharge
			result.Summary.Subscription += row.Subscription
		}
	}
	calculateOperationsProfit(&result.Summary)
	return result, nil
}

func calculateOperationsProfit(row *OperationsFinanceRow) {
	row.Profit = row.Consumption - row.Cost
	row.Margin = nil
	if row.Consumption > 0 {
		margin := row.Profit / row.Consumption * 100
		row.Margin = &margin
	}
}
