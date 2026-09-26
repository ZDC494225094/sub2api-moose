package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Keep success filtering inside each amount aggregate: total_orders and excluded_recharge
// deliberately retain unsuccessful orders to explain the gap without inflating recharge.
const operationsFinancePaymentsQuery = `
	 SELECT to_char(created_at AT TIME ZONE $3, 'YYYY-MM-DD') AS day,
	 COALESCE(SUM(amount) FILTER (WHERE order_type = 'balance' AND status = $6),0) AS recharge,
	 COALESCE(SUM(amount) FILTER (WHERE order_type = 'subscription' AND status IN ($4,$5,$6)),0) AS subscription,
	 COUNT(*) AS total_orders, COUNT(*) FILTER (WHERE status IN ($4,$5,$6)) AS paid_orders,
	 COALESCE(SUM(amount) FILTER (WHERE order_type = 'balance' AND status NOT IN ($4,$5,$6)),0) AS excluded_recharge
	 FROM payment_orders WHERE created_at >= $1 AND created_at < $2 AND order_type IN ('balance','subscription')
	 GROUP BY 1`

// Currency normalization mirrors PaymentOrderCurrency, including legacy CNY fallback.
const operationsFinanceCashQuery = `
 SELECT to_char(created_at AT TIME ZONE $3, 'YYYY-MM-DD') AS day,
 CASE WHEN upper(btrim(provider_snapshot->>'currency')) ~ '^[A-Z]{3}$'
 THEN upper(btrim(provider_snapshot->>'currency')) ELSE 'CNY' END AS currency,
 COALESCE(SUM(pay_amount) FILTER (WHERE order_type='balance'),0) AS recharge_paid,
 COALESCE(SUM(pay_amount) FILTER (WHERE order_type='subscription'),0) AS subscription_paid,
 COALESCE(SUM(amount) FILTER (WHERE order_type='balance' AND status=$6),0) AS credited,
 COALESCE(SUM(amount) FILTER (WHERE order_type='balance' AND status IN ($4,$5)),0) AS pending_credit
 FROM payment_orders WHERE created_at >= $1 AND created_at < $2
 AND order_type IN ('balance','subscription') AND status IN ($4,$5,$6)
 GROUP BY 1,2`

func (r *usageLogRepository) GetOperationsFinance(ctx context.Context, start, end time.Time) (*service.OperationsFinanceResponse, error) {
	// One statement gives all dimensions and inventory the same database snapshot.
	// Include zero-charge requests: they may still incur upstream costs.
	const query = `WITH usage AS MATERIALIZED (
	 SELECT to_char(ul.created_at AT TIME ZONE $3, 'YYYY-MM-DD') AS day,
	 ul.account_id::text AS account, COALESCE(a.name, 'Deleted account #' || ul.account_id::text) AS account_name,
	 COALESCE(NULLIF(a.upstream_group, ''), '未分组') AS upstream,
	 ul.model, ul.actual_cost AS consumption, ul.total_cost AS list_cost,
	 COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1) AS cost
	 FROM usage_logs ul LEFT JOIN accounts a ON a.id = ul.account_id
	 WHERE ul.created_at >= $1 AND ul.created_at < $2
	), dimensions AS (
	 SELECT d.dimension, d.key, d.label, d.upstream,
	 COUNT(*) AS requests, COALESCE(SUM(u.consumption),0) AS consumption,
	 COALESCE(SUM(u.list_cost),0) AS list_cost, COALESCE(SUM(u.cost),0) AS cost
	 FROM usage u CROSS JOIN LATERAL (VALUES
	 ('day', u.day, u.day, ''),
	 ('upstream', u.upstream, u.upstream, ''),
	 ('account', u.account, u.account_name, u.upstream),
	 ('model', u.model, u.model, '')
	 ) d(dimension,key,label,upstream)
	 GROUP BY d.dimension,d.key,d.label,d.upstream
	), payments AS (` + operationsFinancePaymentsQuery + `
	), cash AS (` + operationsFinanceCashQuery + `
 ), inventory_subscriptions AS (
 SELECT jsonb_build_object(
 'StartsAt',s.starts_at,'ExpiresAt',s.expires_at,'Status',s.status,
 'DailyWindowStart',s.daily_window_start,'WeeklyWindowStart',s.weekly_window_start,'MonthlyWindowStart',s.monthly_window_start,
 'DailyUsageUSD',s.daily_usage_usd,'WeeklyUsageUSD',s.weekly_usage_usd,'MonthlyUsageUSD',s.monthly_usage_usd,
 'Group',jsonb_build_object('DailyLimitUSD',g.daily_limit_usd,'WeeklyLimitUSD',g.weekly_limit_usd,'MonthlyLimitUSD',g.monthly_limit_usd)
 ) AS data
 FROM user_subscriptions s JOIN groups g ON g.id=s.group_id JOIN users u ON u.id=s.user_id
 WHERE s.deleted_at IS NULL AND g.deleted_at IS NULL AND u.deleted_at IS NULL
 AND s.status='active' AND s.starts_at <= now() AND s.expires_at > now()
 ), days AS (
	 SELECT to_char(d, 'YYYY-MM-DD') AS day FROM generate_series(
	 ($1::timestamptz AT TIME ZONE $3)::date,
	 ($2::timestamptz AT TIME ZONE $3)::date - 1, interval '1 day') d
	), result AS (
	 SELECT 'day' AS dimension, days.day AS key, days.day AS label, '' AS upstream,
	 COALESCE(d.requests,0) AS requests, COALESCE(d.consumption,0) AS consumption,
	 COALESCE(d.list_cost,0) AS list_cost, COALESCE(d.cost,0) AS cost,
	 COALESCE(p.recharge,0) AS recharge, COALESCE(p.subscription,0) AS subscription,
	 COALESCE(p.total_orders,0) AS total_orders, COALESCE(p.paid_orders,0) AS paid_orders,
	 COALESCE(p.excluded_recharge,0) AS excluded_recharge
	 FROM days LEFT JOIN dimensions d ON d.dimension='day' AND d.key=days.day
	 LEFT JOIN payments p ON p.day=days.day
	 UNION ALL
	 SELECT dimension,key,label,upstream,requests,consumption,list_cost,cost,0,0,0,0,0
	 FROM dimensions WHERE dimension <> 'day'
	)
	SELECT result.*, (SELECT COALESCE(SUM(balance),0) FROM users WHERE deleted_at IS NULL),
 CASE WHEN result.dimension='day' AND result.key=to_char($1::timestamptz AT TIME ZONE $3,'YYYY-MM-DD')
 THEN jsonb_build_object(
 'Payments',COALESCE((SELECT jsonb_agg(cash ORDER BY day,currency) FROM cash),'[]'::jsonb),
 'Subscriptions',COALESCE((SELECT jsonb_agg(data) FROM inventory_subscriptions),'[]'::jsonb),
 'AsOf',now(),'FrozenBalance',(SELECT COALESCE(SUM(frozen_balance),0) FROM users WHERE deleted_at IS NULL))
 ELSE '{}'::jsonb END
	FROM result ORDER BY dimension,key`
	rows, err := r.sql.QueryContext(ctx, query, start, end, start.Location().String(),
		service.OrderStatusPaid, service.OrderStatusRecharging, service.OrderStatusCompleted)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &service.OperationsFinanceResponse{Rows: []service.OperationsFinanceRow{}, Payments: []service.OperationsPaymentDay{}}
	for rows.Next() {
		var row service.OperationsFinanceRow
		var metadata []byte
		if err := rows.Scan(&row.Dimension, &row.Key, &row.Label, &row.Upstream, &row.Requests,
			&row.Consumption, &row.ListCost, &row.Cost, &row.Recharge, &row.Subscription, &row.TotalOrders, &row.PaidOrders, &row.ExcludedRecharge, &result.CurrentBalance, &metadata); err != nil {
			return nil, err
		}
		var snapshot struct {
			Payments      []service.OperationsPaymentDay
			Subscriptions []service.UserSubscription
			AsOf          time.Time
			FrozenBalance float64
		}
		if err := json.Unmarshal(metadata, &snapshot); err != nil {
			return nil, err
		}
		if !snapshot.AsOf.IsZero() {
			result.Payments = snapshot.Payments
			result.Inventory.FrozenBalance = snapshot.FrozenBalance
			for _, sub := range snapshot.Subscriptions {
				if sub.Group != nil {
					result.Inventory.AddSubscription(sub, *sub.Group, snapshot.AsOf)
				}
			}
		}
		result.Rows = append(result.Rows, row)
	}
	return result, rows.Err()
}
