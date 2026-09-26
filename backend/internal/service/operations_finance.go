package service

import (
	"context"
	"math"
	"time"
)

// Usage and recharge credits are USD. Cash receipts are separately grouped by currency.
// Subscription is the legacy nominal plan price, not balance credits.
type OperationsFinanceRow struct {
	TotalOrders      int64    `json:"total_orders"`
	PaidOrders       int64    `json:"paid_orders"`
	ExcludedRecharge float64  `json:"excluded_recharge"`
	Dimension        string   `json:"dimension"`
	Key              string   `json:"key"`
	Label            string   `json:"label"`
	Upstream         string   `json:"upstream"`
	Requests         int64    `json:"requests"`
	Consumption      float64  `json:"consumption"`
	ListCost         float64  `json:"list_cost"`
	Cost             float64  `json:"cost"`
	Recharge         float64  `json:"recharge"`
	Subscription     float64  `json:"subscription"`
	Profit           float64  `json:"profit"`
	Margin           *float64 `json:"margin"`
}

type OperationsPaymentDay struct {
	Day              string  `json:"day"`
	Currency         string  `json:"currency"`
	RechargePaid     float64 `json:"recharge_paid"`
	SubscriptionPaid float64 `json:"subscription_paid"`
	Credited         float64 `json:"credited"`
	PendingCredit    float64 `json:"pending_credit"`
}

type OperationsInventory struct {
	FrozenBalance          float64  `json:"frozen_balance"`
	SubscriptionRemaining  float64  `json:"subscription_remaining"`
	LimitedSubscriptions   int64    `json:"limited_subscriptions"`
	UnlimitedSubscriptions int64    `json:"unlimited_subscriptions"`
	GiftRemaining          *float64 `json:"gift_remaining"` // Not separately tracked by the balance ledger.
}

// AddSubscription uses the billing service's actual calendar/rolling window rules.
// Each subscription contributes its tightest remaining limit, not day+week+month.
func (v *OperationsInventory) AddSubscription(sub UserSubscription, group Group, now time.Time) {
	if sub.Status != SubscriptionStatusActive || sub.StartsAt.After(now) || sub.IsExpiredAt(now) {
		return
	}
	subs := []UserSubscription{sub}
	normalizeExpiredWindowsAt(subs, now)
	sub = subs[0]
	remaining := -1.0
	for _, window := range []struct {
		limit *float64
		used  float64
	}{
		{group.DailyLimitUSD, sub.DailyUsageUSD}, {group.WeeklyLimitUSD, sub.WeeklyUsageUSD}, {group.MonthlyLimitUSD, sub.MonthlyUsageUSD},
	} {
		if window.limit == nil || *window.limit <= 0 {
			continue
		}
		value := math.Max(0, *window.limit-window.used)
		if remaining < 0 || value < remaining {
			remaining = value
		}
	}
	if remaining < 0 {
		v.UnlimitedSubscriptions++
	} else {
		v.LimitedSubscriptions++
		v.SubscriptionRemaining += remaining
	}
}

type OperationsFinanceResponse struct {
	Payments       []OperationsPaymentDay `json:"payments"`
	Inventory      OperationsInventory    `json:"inventory"`
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
			result.Summary.TotalOrders += row.TotalOrders
			result.Summary.PaidOrders += row.PaidOrders
			result.Summary.ExcludedRecharge += row.ExcludedRecharge
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
