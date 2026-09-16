package repository

import (
	"context"
	"encoding/json"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *usageLogRepository) GetOperationsCustomers(ctx context.Context, f service.OperationsCustomerFilter) (*service.OperationsCustomersResponse, error) {
	const query = `WITH paid AS (
  SELECT user_id, count(*) AS total_orders,
   count(*) FILTER (WHERE paid_at >= $1 AND paid_at < $2) AS period_orders,
   COALESCE(sum(amount) FILTER (WHERE paid_at >= $1 AND paid_at < $2),0) AS period_amount,
   min(paid_at) AS first_paid_at, max(paid_at) AS last_paid_at
  FROM payment_orders WHERE paid_at < $3 AND status IN ($7,$8,$9)
  GROUP BY user_id
 ), usage AS (
  SELECT user_id, max(created_at) FILTER (WHERE meaningful) AS last_used_at,
   bool_or(meaningful AND created_at >= $4) AS recently_active,
   bool_or(meaningful AND created_at >= $5 AND created_at < $4) AS previously_active,
   bool_or(meaningful AND created_at >= $1 AND created_at < $2) AS period_active,
   COALESCE(sum(actual_cost) FILTER (WHERE created_at >= $1 AND created_at < $2),0) AS consumption,
   COALESCE(sum(COALESCE(account_stats_cost,total_cost)*COALESCE(account_rate_multiplier,1)) FILTER (WHERE created_at >= $1 AND created_at < $2),0) AS cost
  FROM (SELECT *, (actual_cost > 0 OR total_cost > 0 OR COALESCE(account_stats_cost,0) > 0
   OR input_tokens > 0 OR output_tokens > 0 OR cache_creation_tokens > 0 OR cache_read_tokens > 0) AS meaningful
   FROM usage_logs WHERE created_at < $3) logs
  GROUP BY user_id
 ), customers AS MATERIALIZED (
  SELECT u.id, u.email, COALESCE(u.username,'') AS username, u.balance,
   COALESCE(p.period_orders,0) AS period_orders, COALESCE(p.total_orders,0) AS total_orders,
   COALESCE(p.period_amount,0) AS period_amount, p.last_paid_at, us.last_used_at,
   COALESCE(us.consumption,0) AS consumption, COALESCE(us.cost,0) AS cost,
   COALESCE(p.period_orders > 0 AND p.total_orders >= 2,false) AS repeat,
   COALESCE(p.first_paid_at >= $1 AND p.first_paid_at < $2,false) AS new_paying,
   COALESCE(us.period_active,false) AS active,
   COALESCE(us.previously_active,false) AS previous_active,
   COALESCE(us.previously_active AND NOT us.recently_active,false) AS churned
  FROM users u LEFT JOIN paid p ON p.user_id=u.id LEFT JOIN usage us ON us.user_id=u.id
  WHERE u.deleted_at IS NULL
 ), filtered AS (
  SELECT * FROM customers WHERE
   CASE $6 WHEN 'balance' THEN balance <> 0 WHEN 'paying' THEN period_orders > 0
   WHEN 'repeat' THEN repeat WHEN 'new_paying' THEN new_paying WHEN 'active' THEN active
   WHEN 'churned' THEN churned ELSE true END
   AND ($10 = '' OR strpos(lower(email || ' ' || username), lower($10)) > 0)
 ), summary AS (
  SELECT count(*) AS total, count(*) FILTER (WHERE period_orders > 0) AS paying,
   count(*) FILTER (WHERE repeat) AS repeat, count(*) FILTER (WHERE new_paying) AS new_paying,
   count(*) FILTER (WHERE active) AS active, count(*) FILTER (WHERE churned) AS churned,
   count(*) FILTER (WHERE previous_active) AS previous_active,
   count(*) FILTER (WHERE balance <> 0) AS balance_users FROM customers
 )
 SELECT json_build_object('summary', (SELECT row_to_json(summary) FROM summary),
  'total', (SELECT count(*) FROM filtered),
  'items', COALESCE((SELECT json_agg(row_to_json(page)) FROM (
   SELECT * FROM filtered ORDER BY
    CASE WHEN $6 = 'balance' THEN balance ELSE consumption END DESC, id
   LIMIT $11 OFFSET $12
  ) page), '[]'::json))`
	var payload []byte
	err := scanSingleRow(ctx, r.sql, query, []any{f.Start, f.End, f.AsOf,
		f.AsOf.AddDate(0, 0, -f.ChurnDays), f.AsOf.AddDate(0, 0, -2*f.ChurnDays), f.Segment,
		service.OrderStatusCompleted, service.OrderStatusPaid, service.OrderStatusRecharging,
		f.Search, f.PageSize, (f.Page - 1) * f.PageSize}, &payload)
	if err != nil {
		return nil, err
	}
	var result service.OperationsCustomersResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
