package repository

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

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
	), payments AS (
	 SELECT to_char(created_at AT TIME ZONE $3, 'YYYY-MM-DD') AS day,
	 COALESCE(SUM(amount) FILTER (WHERE order_type IN ('balance','subscription')),0) AS recharge,
	 COALESCE(SUM(amount) FILTER (WHERE order_type = 'subscription'),0) AS subscription
	 FROM payment_orders WHERE created_at >= $1 AND created_at < $2
	 GROUP BY 1
	), days AS (
	 SELECT to_char(d, 'YYYY-MM-DD') AS day FROM generate_series(
	 ($1::timestamptz AT TIME ZONE $3)::date,
	 ($2::timestamptz AT TIME ZONE $3)::date - 1, interval '1 day') d
	), result AS (
	 SELECT 'day' AS dimension, days.day AS key, days.day AS label, '' AS upstream,
	 COALESCE(d.requests,0) AS requests, COALESCE(d.consumption,0) AS consumption,
	 COALESCE(d.list_cost,0) AS list_cost, COALESCE(d.cost,0) AS cost,
	 COALESCE(p.recharge,0) AS recharge, COALESCE(p.subscription,0) AS subscription
	 FROM days LEFT JOIN dimensions d ON d.dimension='day' AND d.key=days.day
	 LEFT JOIN payments p ON p.day=days.day
	 UNION ALL
	 SELECT dimension,key,label,upstream,requests,consumption,list_cost,cost,0,0
	 FROM dimensions WHERE dimension <> 'day'
	)
	SELECT result.*, (SELECT COALESCE(SUM(balance),0) FROM users WHERE deleted_at IS NULL)
	FROM result ORDER BY dimension,key`
	rows, err := r.sql.QueryContext(ctx, query, start, end, start.Location().String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := &service.OperationsFinanceResponse{Rows: []service.OperationsFinanceRow{}}
	for rows.Next() {
		var row service.OperationsFinanceRow
		if err := rows.Scan(&row.Dimension, &row.Key, &row.Label, &row.Upstream, &row.Requests,
			&row.Consumption, &row.ListCost, &row.Cost, &row.Recharge, &row.Subscription, &result.CurrentBalance); err != nil {
			return nil, err
		}
		result.Rows = append(result.Rows, row)
	}
	return result, rows.Err()
}
