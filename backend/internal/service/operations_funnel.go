package service

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var ErrOperationsFunnelUnsupported = errors.New("operations funnel repository is not configured")

type OperationsFunnelStats struct {
	RegisteredUsers              int64
	CreatedKeyUsers              int64
	ActiveUsers                  int64
	PayingUsers                  int64
	TotalUsers                   int64
	AllActiveUsers               int64
	AllInactiveUsers             int64
	PeriodPayingUsers            int64
	PaidOrders                   int64
	TotalRevenue                 float64
	BalanceRevenue               float64
	BalanceOrders                int64
	SubscriptionRevenue          float64
	SubscriptionOrders           int64
	TotalRechargeAmount          float64
	RemainingBalance             float64
	GiftedAmount                 float64
	ActiveSubscriptions          int64
	ActiveSubscriptionUsers      int64
	LimitedSubscriptions         int64
	SubscriptionDailyRemaining   float64
	SubscriptionWeeklyRemaining  float64
	SubscriptionMonthlyRemaining float64
	PendingOrders                int64
	FailedOrders                 int64
	RefundRequestedOrders        int64
	ExpiredOrders                int64
	CancelledOrders              int64
	ActiveStatusUsers            int64
	DisabledStatusUsers          int64
	RechargedUsers               int64
	BalanceRechargeUsers         int64
	SubscriptionPurchaseUsers    int64
	SubscriptionDailyLimit       float64
	SubscriptionWeeklyLimit      float64
	SubscriptionMonthlyLimit     float64
	SubscriptionDailyUsed        float64
	SubscriptionWeeklyUsed       float64
	SubscriptionMonthlyUsed      float64
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

type OperationsUserSummary struct {
	TotalUsers    int64   `json:"total_users"`
	ActiveUsers   int64   `json:"active_users"`
	InactiveUsers int64   `json:"inactive_users"`
	ActiveRate    float64 `json:"active_rate"`
}

type OperationsCreditSummary struct {
	TotalRechargeAmount float64 `json:"total_recharge_amount"`
	RemainingBalance    float64 `json:"remaining_balance"`
	GiftedAmount        float64 `json:"gifted_amount"`
}

type OperationsSubscriptionSummary struct {
	ActiveSubscriptions     int64   `json:"active_subscriptions"`
	ActiveSubscriptionUsers int64   `json:"active_subscription_users"`
	LimitedSubscriptions    int64   `json:"limited_subscriptions"`
	DailyRemainingUSD       float64 `json:"daily_remaining_usd"`
	WeeklyRemainingUSD      float64 `json:"weekly_remaining_usd"`
	MonthlyRemainingUSD     float64 `json:"monthly_remaining_usd"`
}

type OperationsBreakdownItem struct {
	Key     string  `json:"key"`
	Label   string  `json:"label"`
	Count   int64   `json:"count"`
	Amount  float64 `json:"amount"`
	Percent float64 `json:"percent"`
}

type OperationsSubscriptionQuotaBreakdown struct {
	Key             string  `json:"key"`
	Label           string  `json:"label"`
	LimitUSD        float64 `json:"limit_usd"`
	UsedUSD         float64 `json:"used_usd"`
	RemainingUSD    float64 `json:"remaining_usd"`
	UtilizationRate float64 `json:"utilization_rate"`
}

type OperationsBreakdownSummary struct {
	Users         []OperationsBreakdownItem              `json:"users"`
	Credits       []OperationsBreakdownItem              `json:"credits"`
	Revenue       []OperationsBreakdownItem              `json:"revenue"`
	Subscriptions []OperationsSubscriptionQuotaBreakdown `json:"subscriptions"`
}

type OperationsFunnelResponse struct {
	StartDate     string                        `json:"start_date"`
	EndDate       string                        `json:"end_date"`
	RangeDays     int                           `json:"range_days"`
	GeneratedAt   string                        `json:"generated_at"`
	Steps         []OperationsFunnelStep        `json:"steps"`
	Revenue       OperationsRevenueSummary      `json:"revenue"`
	Signals       OperationsOrderSignals        `json:"signals"`
	Users         OperationsUserSummary         `json:"users"`
	Credits       OperationsCreditSummary       `json:"credits"`
	Subscriptions OperationsSubscriptionSummary `json:"subscriptions"`
	Breakdown     OperationsBreakdownSummary    `json:"breakdown"`
}

type OperationsUserDetailFilter struct {
	Segment    string
	StartTime  time.Time
	EndTime    time.Time
	Pagination pagination.PaginationParams
}

type OperationsUserDetail struct {
	UserID                          int64      `json:"user_id"`
	Email                           string     `json:"email"`
	UserName                        string     `json:"user_name,omitempty"`
	Status                          string     `json:"status"`
	Balance                         float64    `json:"balance"`
	TotalRecharged                  float64    `json:"total_recharged"`
	GiftedAmountEstimate            float64    `json:"gifted_amount_estimate"`
	LastActiveAt                    *time.Time `json:"last_active_at,omitempty"`
	CreatedAt                       time.Time  `json:"created_at"`
	PeriodRequests                  int64      `json:"period_requests"`
	PeriodUsageCost                 float64    `json:"period_usage_cost"`
	PaidOrderCount                  int64      `json:"paid_order_count"`
	PaidOrderAmount                 float64    `json:"paid_order_amount"`
	BalanceOrderAmount              float64    `json:"balance_order_amount"`
	SubscriptionOrderAmount         float64    `json:"subscription_order_amount"`
	ActiveSubscriptionCount         int64      `json:"active_subscription_count"`
	SubscriptionDailyRemainingUSD   float64    `json:"subscription_daily_remaining_usd"`
	SubscriptionWeeklyRemainingUSD  float64    `json:"subscription_weekly_remaining_usd"`
	SubscriptionMonthlyRemainingUSD float64    `json:"subscription_monthly_remaining_usd"`
}

type operationsFunnelReader interface {
	GetOperationsFunnel(ctx context.Context, startTime, endTime time.Time) (*OperationsFunnelStats, error)
}

type operationsUserDetailReader interface {
	ListOperationsUserDetails(ctx context.Context, filter OperationsUserDetailFilter) ([]OperationsUserDetail, int64, error)
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
		Users: OperationsUserSummary{
			TotalUsers:    stats.TotalUsers,
			ActiveUsers:   stats.AllActiveUsers,
			InactiveUsers: stats.AllInactiveUsers,
			ActiveRate:    percent(stats.AllActiveUsers, stats.TotalUsers),
		},
		Credits: OperationsCreditSummary{
			TotalRechargeAmount: round2(stats.TotalRechargeAmount),
			RemainingBalance:    round2(stats.RemainingBalance),
			GiftedAmount:        round2(stats.GiftedAmount),
		},
		Subscriptions: OperationsSubscriptionSummary{
			ActiveSubscriptions:     stats.ActiveSubscriptions,
			ActiveSubscriptionUsers: stats.ActiveSubscriptionUsers,
			LimitedSubscriptions:    stats.LimitedSubscriptions,
			DailyRemainingUSD:       round2(stats.SubscriptionDailyRemaining),
			WeeklyRemainingUSD:      round2(stats.SubscriptionWeeklyRemaining),
			MonthlyRemainingUSD:     round2(stats.SubscriptionMonthlyRemaining),
		},
		Breakdown: buildOperationsBreakdown(stats),
	}, nil
}

func (s *DashboardService) ListOperationsUserDetails(ctx context.Context, filter OperationsUserDetailFilter) ([]OperationsUserDetail, int64, error) {
	reader, ok := s.usageRepo.(operationsUserDetailReader)
	if !ok {
		return nil, 0, ErrOperationsFunnelUnsupported
	}
	filter.Segment = normalizeOperationsUserSegment(filter.Segment)
	if filter.Pagination.Page <= 0 {
		filter.Pagination.Page = 1
	}
	if filter.Pagination.PageSize <= 0 {
		filter.Pagination.PageSize = 20
	}
	if filter.Pagination.PageSize > 100 {
		filter.Pagination.PageSize = 100
	}
	items, total, err := reader.ListOperationsUserDetails(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].Balance = round2(items[i].Balance)
		items[i].TotalRecharged = round2(items[i].TotalRecharged)
		items[i].GiftedAmountEstimate = round2(items[i].GiftedAmountEstimate)
		items[i].PeriodUsageCost = round2(items[i].PeriodUsageCost)
		items[i].PaidOrderAmount = round2(items[i].PaidOrderAmount)
		items[i].BalanceOrderAmount = round2(items[i].BalanceOrderAmount)
		items[i].SubscriptionOrderAmount = round2(items[i].SubscriptionOrderAmount)
		items[i].SubscriptionDailyRemainingUSD = round2(items[i].SubscriptionDailyRemainingUSD)
		items[i].SubscriptionWeeklyRemainingUSD = round2(items[i].SubscriptionWeeklyRemainingUSD)
		items[i].SubscriptionMonthlyRemainingUSD = round2(items[i].SubscriptionMonthlyRemainingUSD)
	}
	return items, total, nil
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

func buildOperationsBreakdown(stats *OperationsFunnelStats) OperationsBreakdownSummary {
	consumedCredit := math.Max(stats.TotalRechargeAmount+stats.GiftedAmount-stats.RemainingBalance, 0)
	return OperationsBreakdownSummary{
		Users: []OperationsBreakdownItem{
			{Key: "active_period", Label: "周期活跃用户", Count: stats.AllActiveUsers, Percent: percent(stats.AllActiveUsers, stats.TotalUsers)},
			{Key: "inactive_period", Label: "周期非活跃用户", Count: stats.AllInactiveUsers, Percent: percent(stats.AllInactiveUsers, stats.TotalUsers)},
			{Key: "status_active", Label: "启用账号", Count: stats.ActiveStatusUsers, Percent: percent(stats.ActiveStatusUsers, stats.TotalUsers)},
			{Key: "status_disabled", Label: "禁用账号", Count: stats.DisabledStatusUsers, Percent: percent(stats.DisabledStatusUsers, stats.TotalUsers)},
			{Key: "recharged", Label: "有充值记录用户", Count: stats.RechargedUsers, Percent: percent(stats.RechargedUsers, stats.TotalUsers)},
		},
		Credits: []OperationsBreakdownItem{
			{Key: "balance_recharge", Label: "余额充值额度", Count: stats.BalanceRechargeUsers, Amount: round2(stats.TotalRechargeAmount)},
			{Key: "remaining_balance", Label: "用户剩余额度", Amount: round2(stats.RemainingBalance)},
			{Key: "gifted", Label: "赠送额度估算", Amount: round2(stats.GiftedAmount)},
			{Key: "consumed", Label: "已消耗额度估算", Amount: round2(consumedCredit)},
		},
		Revenue: []OperationsBreakdownItem{
			{Key: "balance", Label: "周期余额订单", Count: stats.BalanceOrders, Amount: round2(stats.BalanceRevenue)},
			{Key: "subscription", Label: "周期订阅订单", Count: stats.SubscriptionOrders, Amount: round2(stats.SubscriptionRevenue)},
			{Key: "subscription_purchase_users", Label: "历史订阅购买用户", Count: stats.SubscriptionPurchaseUsers},
		},
		Subscriptions: []OperationsSubscriptionQuotaBreakdown{
			buildSubscriptionQuotaBreakdown("daily", "日额度", stats.SubscriptionDailyLimit, stats.SubscriptionDailyUsed, stats.SubscriptionDailyRemaining),
			buildSubscriptionQuotaBreakdown("weekly", "周额度", stats.SubscriptionWeeklyLimit, stats.SubscriptionWeeklyUsed, stats.SubscriptionWeeklyRemaining),
			buildSubscriptionQuotaBreakdown("monthly", "月额度", stats.SubscriptionMonthlyLimit, stats.SubscriptionMonthlyUsed, stats.SubscriptionMonthlyRemaining),
		},
	}
}

func buildSubscriptionQuotaBreakdown(key, label string, limit, used, remaining float64) OperationsSubscriptionQuotaBreakdown {
	return OperationsSubscriptionQuotaBreakdown{
		Key:             key,
		Label:           label,
		LimitUSD:        round2(limit),
		UsedUSD:         round2(used),
		RemainingUSD:    round2(remaining),
		UtilizationRate: percentFloat(used, limit),
	}
}

func normalizeOperationsUserSegment(segment string) string {
	switch segment {
	case "active", "inactive", "balance", "recharge", "subscription":
		return segment
	default:
		return "all"
	}
}

func percent(value, base int64) float64 {
	if base <= 0 {
		return 0
	}
	return round2(float64(value) * 100 / float64(base))
}

func percentFloat(value, base float64) float64 {
	if base <= 0 {
		return 0
	}
	return round2(value * 100 / base)
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
