package service

import (
	"context"
	"errors"
	"time"
)

type OperationsCustomerFilter struct {
	Start, End, AsOf          time.Time
	ChurnDays, Page, PageSize int
	Segment, Search           string
}

type OperationsCustomerSummary struct {
	Total          int64    `json:"total"`
	Paying         int64    `json:"paying"`
	Repeat         int64    `json:"repeat"`
	NewPaying      int64    `json:"new_paying"`
	Active         int64    `json:"active"`
	Churned        int64    `json:"churned"`
	PreviousActive int64    `json:"previous_active"`
	BalanceUsers   int64    `json:"balance_users"`
	RepeatRate     *float64 `json:"repeat_rate"`
	ChurnRate      *float64 `json:"churn_rate"`
}

type OperationsCustomer struct {
	ID           int64      `json:"id"`
	Email        string     `json:"email"`
	Username     string     `json:"username"`
	Balance      float64    `json:"balance"`
	PeriodOrders int64      `json:"period_orders"`
	TotalOrders  int64      `json:"total_orders"`
	PeriodAmount float64    `json:"period_amount"`
	Consumption  float64    `json:"consumption"`
	Cost         float64    `json:"cost"`
	Profit       float64    `json:"profit"`
	Margin       *float64   `json:"margin"`
	LastUsedAt   *time.Time `json:"last_used_at"`
	LastPaidAt   *time.Time `json:"last_paid_at"`
	Repeat       bool       `json:"repeat"`
	Churned      bool       `json:"churned"`
}

type OperationsCustomersResponse struct {
	Summary   OperationsCustomerSummary `json:"summary"`
	Items     []OperationsCustomer      `json:"items"`
	Total     int64                     `json:"total"`
	AsOf      time.Time                 `json:"as_of"`
	ChurnDays int                       `json:"churn_days"`
}

type operationsCustomersReader interface {
	GetOperationsCustomers(context.Context, OperationsCustomerFilter) (*OperationsCustomersResponse, error)
}

func (s *DashboardService) GetOperationsCustomers(ctx context.Context, f OperationsCustomerFilter) (*OperationsCustomersResponse, error) {
	switch f.Segment {
	case "all", "balance", "paying", "repeat", "new_paying", "active", "churned":
	default:
		return nil, errors.New("invalid customer segment")
	}
	switch f.ChurnDays {
	case 7, 30, 60, 90:
	default:
		return nil, errors.New("invalid churn period")
	}
	if f.Page < 1 || f.PageSize < 1 || f.PageSize > 100 {
		return nil, errors.New("invalid pagination")
	}
	f.AsOf = f.End
	if now := time.Now().In(f.End.Location()); f.AsOf.After(now) {
		f.AsOf = now
	}
	r, ok := s.usageRepo.(operationsCustomersReader)
	if !ok {
		return nil, ErrOperationsFunnelUnsupported
	}
	result, err := r.GetOperationsCustomers(ctx, f)
	if err != nil {
		return nil, err
	}
	result.AsOf, result.ChurnDays = f.AsOf, f.ChurnDays
	result.Summary.RepeatRate = operationsCustomerRate(result.Summary.Repeat, result.Summary.Paying)
	result.Summary.ChurnRate = operationsCustomerRate(result.Summary.Churned, result.Summary.PreviousActive)
	for i := range result.Items {
		row := &result.Items[i]
		financial := OperationsFinanceRow{Consumption: row.Consumption, Cost: row.Cost}
		calculateOperationsProfit(&financial)
		row.Profit, row.Margin = financial.Profit, financial.Margin
	}
	return result, nil
}

func operationsCustomerRate(numerator, denominator int64) *float64 {
	if denominator == 0 {
		return nil
	}
	value := float64(numerator) / float64(denominator) * 100
	return &value
}
