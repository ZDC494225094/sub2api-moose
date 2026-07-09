package service

import (
	"context"
	"errors"
	"math"
	"time"
)

var ErrOperationsFunnelUnsupported = errors.New("operations funnel repository is not configured")

type OperationsFunnelStats struct {
	RegisteredUsers       int64
	CreatedKeyUsers       int64
	ActiveUsers           int64
	PayingUsers           int64
	PeriodPayingUsers     int64
	PaidOrders            int64
	TotalRevenue          float64
	BalanceRevenue        float64
	BalanceOrders         int64
	SubscriptionRevenue   float64
	SubscriptionOrders    int64
	PendingOrders         int64
	FailedOrders          int64
	RefundRequestedOrders int64
	ExpiredOrders         int64
	CancelledOrders       int64
}

type OperationsFunnelStep struct {
	Key                   string  `json:"key"`
	Label                 string  `json:"label"`
	Count                 int64   `json:"count"`
	ConversionRate        float64 `json:"conversion_rate"`
	OverallConversionRate float64 `json:"overall_conversion_rate"`
	DropoffFromPrevious   int64   `json:"dropoff_from_previous"`
}

type OperationsRevenueSummary struct {
	TotalRevenue        float64 `json:"total_revenue"`
	PaidOrders          int64   `json:"paid_orders"`
	PayingUsers         int64   `json:"paying_users"`
	AverageOrderAmount  float64 `json:"average_order_amount"`
	BalanceRevenue      float64 `json:"balance_revenue"`
	BalanceOrders       int64   `json:"balance_orders"`
	SubscriptionRevenue float64 `json:"subscription_revenue"`
	SubscriptionOrders  int64   `json:"subscription_orders"`
}

type OperationsOrderSignals struct {
	PendingOrders         int64 `json:"pending_orders"`
	FailedOrders          int64 `json:"failed_orders"`
	RefundRequestedOrders int64 `json:"refund_requested_orders"`
	ExpiredOrders         int64 `json:"expired_orders"`
	CancelledOrders       int64 `json:"cancelled_orders"`
}

type OperationsFunnelResponse struct {
	StartDate   string                   `json:"start_date"`
	EndDate     string                   `json:"end_date"`
	RangeDays   int                      `json:"range_days"`
	GeneratedAt string                   `json:"generated_at"`
	Steps       []OperationsFunnelStep   `json:"steps"`
	Revenue     OperationsRevenueSummary `json:"revenue"`
	Signals     OperationsOrderSignals   `json:"signals"`
}

type operationsFunnelReader interface {
	GetOperationsFunnel(ctx context.Context, startTime, endTime time.Time) (*OperationsFunnelStats, error)
}

func (s *DashboardService) GetOperationsFunnel(ctx context.Context, startTime, endTime time.Time) (*OperationsFunnelResponse, error) {
	reader, ok := s.usageRepo.(operationsFunnelReader)
	if !ok {
		return nil, ErrOperationsFunnelUnsupported
	}
	stats, err := reader.GetOperationsFunnel(ctx, startTime, endTime)
	if err != nil {
		return nil, err
	}
	if stats == nil {
		stats = &OperationsFunnelStats{}
	}

	return &OperationsFunnelResponse{
		StartDate:   startTime.Format("2006-01-02"),
		EndDate:     endTime.Add(-time.Nanosecond).Format("2006-01-02"),
		RangeDays:   rangeDays(startTime, endTime),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:       buildOperationsFunnelSteps(stats),
		Revenue: OperationsRevenueSummary{
			TotalRevenue:        round2(stats.TotalRevenue),
			PaidOrders:          stats.PaidOrders,
			PayingUsers:         stats.PeriodPayingUsers,
			AverageOrderAmount:  averageOrderAmount(stats.TotalRevenue, stats.PaidOrders),
			BalanceRevenue:      round2(stats.BalanceRevenue),
			BalanceOrders:       stats.BalanceOrders,
			SubscriptionRevenue: round2(stats.SubscriptionRevenue),
			SubscriptionOrders:  stats.SubscriptionOrders,
		},
		Signals: OperationsOrderSignals{
			PendingOrders:         stats.PendingOrders,
			FailedOrders:          stats.FailedOrders,
			RefundRequestedOrders: stats.RefundRequestedOrders,
			ExpiredOrders:         stats.ExpiredOrders,
			CancelledOrders:       stats.CancelledOrders,
		},
	}, nil
}

func buildOperationsFunnelSteps(stats *OperationsFunnelStats) []OperationsFunnelStep {
	raw := []struct {
		key   string
		label string
		count int64
	}{
		{key: "registered", label: "新注册用户", count: stats.RegisteredUsers},
		{key: "created_key", label: "已创建 API Key", count: stats.CreatedKeyUsers},
		{key: "active", label: "已产生调用", count: stats.ActiveUsers},
		{key: "paying", label: "已完成支付", count: stats.PayingUsers},
	}

	steps := make([]OperationsFunnelStep, 0, len(raw))
	var previous int64
	for i, item := range raw {
		conversionBase := previous
		if i == 0 {
			conversionBase = item.count
		}
		dropoff := int64(0)
		if i > 0 && previous > item.count {
			dropoff = previous - item.count
		}
		steps = append(steps, OperationsFunnelStep{
			Key:                   item.key,
			Label:                 item.label,
			Count:                 item.count,
			ConversionRate:        percent(item.count, conversionBase),
			OverallConversionRate: percent(item.count, stats.RegisteredUsers),
			DropoffFromPrevious:   dropoff,
		})
		previous = item.count
	}
	return steps
}

func percent(value, base int64) float64 {
	if base <= 0 {
		return 0
	}
	return round2(float64(value) * 100 / float64(base))
}

func averageOrderAmount(total float64, count int64) float64 {
	if count <= 0 {
		return 0
	}
	return round2(total / float64(count))
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func rangeDays(startTime, endTime time.Time) int {
	if !endTime.After(startTime) {
		return 0
	}
	days := int(math.Ceil(endTime.Sub(startTime).Hours() / 24))
	if days < 1 {
		return 1
	}
	return days
}
