package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const adminMergedUsageSelectColumns = `
	id,
	request_kind,
	created_at,
	sort_model,
	user_id,
	api_key_id,
	account_id,
	request_id,
	model,
	requested_model,
	upstream_model,
	group_id,
	subscription_id,
	input_tokens,
	output_tokens,
	cache_creation_tokens,
	cache_read_tokens,
	cache_creation_5m_tokens,
	cache_creation_1h_tokens,
	image_output_tokens,
	image_output_cost,
	input_cost,
	output_cost,
	cache_creation_cost,
	cache_read_cost,
	total_cost,
	actual_cost,
	rate_multiplier,
	account_rate_multiplier,
	billing_type,
	request_type,
	stream,
	openai_ws_mode,
	duration_ms,
	first_token_ms,
	user_agent,
	ip_address,
	image_count,
	image_size,
	image_input_size,
	image_output_size,
	image_size_source,
	image_size_breakdown,
	service_tier,
	reasoning_effort,
	inbound_endpoint,
	upstream_endpoint,
	cache_ttl_overridden,
	channel_id,
	model_mapping_chain,
	billing_tier,
	billing_mode,
	account_stats_cost,
	status_code,
	error_message,
	error_phase,
	error_severity
`

func (r *usageLogRepository) GetOperationsFunnel(ctx context.Context, startTime, endTime time.Time) (*service.OperationsFunnelStats, error) {
	stats := &service.OperationsFunnelStats{}
	query := `
		WITH paid_statuses(status) AS (
			VALUES ($3), ($4), ($5)
		),
		active_users_period AS (
			SELECT DISTINCT ul.user_id
			FROM usage_logs ul
			WHERE ul.created_at >= $1
				AND ul.created_at < $2
				AND ul.actual_cost > 0
		),
		cohort AS (
			SELECT id
			FROM users
			WHERE created_at >= $1
				AND created_at < $2
				AND deleted_at IS NULL
		),
		funnel_counts AS (
			SELECT
				(SELECT COUNT(*) FROM cohort) AS registered_users,
				(
					SELECT COUNT(DISTINCT ak.user_id)
					FROM api_keys ak
					JOIN cohort c ON c.id = ak.user_id
					WHERE ak.deleted_at IS NULL
						AND ak.created_at >= $1
						AND ak.created_at < $2
				) AS created_key_users,
				(
					SELECT COUNT(DISTINCT ul.user_id)
					FROM usage_logs ul
					JOIN cohort c ON c.id = ul.user_id
					WHERE ul.created_at >= $1
						AND ul.created_at < $2
						AND ul.actual_cost > 0
				) AS active_users,
				(
					SELECT COUNT(DISTINCT po.user_id)
					FROM payment_orders po
					JOIN cohort c ON c.id = po.user_id
					WHERE po.status IN (SELECT status FROM paid_statuses)
						AND po.paid_at IS NOT NULL
						AND po.paid_at >= $1
						AND po.paid_at < $2
				) AS paying_users
		),
		all_user_stats AS (
			SELECT
				COUNT(*) AS total_users,
				COUNT(aup.user_id) AS active_users,
				COALESCE(SUM(u.balance), 0) AS remaining_balance,
				COUNT(*) FILTER (WHERE u.status = $14) AS active_status_users,
				COUNT(*) FILTER (WHERE u.status = $15) AS disabled_status_users,
				COUNT(*) FILTER (WHERE COALESCE(u.total_recharged, 0) > 0) AS recharged_users
			FROM users u
			LEFT JOIN active_users_period aup ON aup.user_id = u.id
			WHERE u.deleted_at IS NULL
		),
		period_orders AS (
			SELECT
				COUNT(*) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2) AS paid_orders,
				COUNT(DISTINCT user_id) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2) AS period_paying_users,
				COALESCE(SUM(pay_amount) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2), 0) AS total_revenue,
				COALESCE(SUM(pay_amount) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2 AND order_type = $6), 0) AS balance_revenue,
				COUNT(*) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2 AND order_type = $6) AS balance_orders,
				COALESCE(SUM(pay_amount) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2 AND order_type = $7), 0) AS subscription_revenue,
				COUNT(*) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2 AND order_type = $7) AS subscription_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $8) AS pending_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $9) AS failed_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $10) AS refund_requested_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $11) AS expired_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $12) AS cancelled_orders
			FROM payment_orders
			WHERE (created_at >= $1 AND created_at < $2)
				OR (paid_at >= $1 AND paid_at < $2)
		),
		balance_recharge_inventory AS (
			SELECT
				COALESCE(SUM(amount) FILTER (WHERE order_type = $6), 0) AS total_recharge_amount,
				COUNT(DISTINCT user_id) FILTER (WHERE order_type = $6) AS balance_recharge_users,
				COUNT(DISTINCT user_id) FILTER (WHERE order_type = $7) AS subscription_purchase_users
			FROM payment_orders
			WHERE status IN (SELECT status FROM paid_statuses)
		),
		user_credit_inventory AS (
			SELECT
				COALESCE(b.total_recharge_amount, 0) AS total_recharge_amount,
				u.remaining_balance,
				GREATEST(
					(SELECT COALESCE(SUM(total_recharged), 0) FROM users WHERE deleted_at IS NULL)
						- COALESCE(b.total_recharge_amount, 0),
					0
				) AS gifted_amount
			FROM all_user_stats u
			CROSS JOIN balance_recharge_inventory b
		),
		subscription_inventory AS (
			SELECT
				COUNT(*) AS active_subscriptions,
				COUNT(DISTINCT us.user_id) AS active_subscription_users,
				COUNT(*) FILTER (
					WHERE COALESCE(g.daily_limit_usd, 0) > 0
						OR COALESCE(g.weekly_limit_usd, 0) > 0
						OR COALESCE(g.monthly_limit_usd, 0) > 0
				) AS limited_subscriptions,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.daily_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.daily_limit_usd, 0) - COALESCE(us.daily_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_daily_remaining,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.weekly_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.weekly_limit_usd, 0) - COALESCE(us.weekly_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_weekly_remaining,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.monthly_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.monthly_limit_usd, 0) - COALESCE(us.monthly_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_monthly_remaining,
				COALESCE(SUM(COALESCE(g.daily_limit_usd, 0)), 0) AS subscription_daily_limit,
				COALESCE(SUM(COALESCE(g.weekly_limit_usd, 0)), 0) AS subscription_weekly_limit,
				COALESCE(SUM(COALESCE(g.monthly_limit_usd, 0)), 0) AS subscription_monthly_limit,
				COALESCE(SUM(CASE WHEN COALESCE(g.daily_limit_usd, 0) > 0 THEN COALESCE(us.daily_usage_usd, 0) ELSE 0 END), 0) AS subscription_daily_used,
				COALESCE(SUM(CASE WHEN COALESCE(g.weekly_limit_usd, 0) > 0 THEN COALESCE(us.weekly_usage_usd, 0) ELSE 0 END), 0) AS subscription_weekly_used,
				COALESCE(SUM(CASE WHEN COALESCE(g.monthly_limit_usd, 0) > 0 THEN COALESCE(us.monthly_usage_usd, 0) ELSE 0 END), 0) AS subscription_monthly_used
			FROM user_subscriptions us
			JOIN groups g ON g.id = us.group_id AND g.deleted_at IS NULL
			WHERE us.deleted_at IS NULL
				AND us.status = $13
				AND us.expires_at > NOW()
		)
		SELECT
			f.registered_users,
			f.created_key_users,
			f.active_users,
			f.paying_users,
			u.total_users,
			u.active_users,
			GREATEST(u.total_users - u.active_users, 0) AS inactive_users,
			p.period_paying_users,
			p.paid_orders,
			p.total_revenue,
			p.balance_revenue,
			p.balance_orders,
			p.subscription_revenue,
			p.subscription_orders,
			c.total_recharge_amount,
			c.remaining_balance,
			c.gifted_amount,
			s.active_subscriptions,
			s.active_subscription_users,
			s.limited_subscriptions,
			s.subscription_daily_remaining,
			s.subscription_weekly_remaining,
			s.subscription_monthly_remaining,
			p.pending_orders,
			p.failed_orders,
			p.refund_requested_orders,
			p.expired_orders,
			p.cancelled_orders,
			u.active_status_users,
			u.disabled_status_users,
			u.recharged_users,
			c.balance_recharge_users,
			c.subscription_purchase_users,
			s.subscription_daily_limit,
			s.subscription_weekly_limit,
			s.subscription_monthly_limit,
			s.subscription_daily_used,
			s.subscription_weekly_used,
			s.subscription_monthly_used
		FROM funnel_counts f
		CROSS JOIN all_user_stats u
		CROSS JOIN period_orders p
		CROSS JOIN user_credit_inventory c
		CROSS JOIN subscription_inventory s
	`
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{
			startTime,
			endTime,
			service.OrderStatusCompleted,
			service.OrderStatusPaid,
			service.OrderStatusRecharging,
			payment.OrderTypeBalance,
			payment.OrderTypeSubscription,
			service.OrderStatusPending,
			service.OrderStatusFailed,
			service.OrderStatusRefundRequested,
			service.OrderStatusExpired,
			service.OrderStatusCancelled,
			service.SubscriptionStatusActive,
			service.StatusActive,
			service.StatusDisabled,
		},
		&stats.RegisteredUsers,
		&stats.CreatedKeyUsers,
		&stats.ActiveUsers,
		&stats.PayingUsers,
		&stats.TotalUsers,
		&stats.AllActiveUsers,
		&stats.AllInactiveUsers,
		&stats.PeriodPayingUsers,
		&stats.PaidOrders,
		&stats.TotalRevenue,
		&stats.BalanceRevenue,
		&stats.BalanceOrders,
		&stats.SubscriptionRevenue,
		&stats.SubscriptionOrders,
		&stats.TotalRechargeAmount,
		&stats.RemainingBalance,
		&stats.GiftedAmount,
		&stats.ActiveSubscriptions,
		&stats.ActiveSubscriptionUsers,
		&stats.LimitedSubscriptions,
		&stats.SubscriptionDailyRemaining,
		&stats.SubscriptionWeeklyRemaining,
		&stats.SubscriptionMonthlyRemaining,
		&stats.PendingOrders,
		&stats.FailedOrders,
		&stats.RefundRequestedOrders,
		&stats.ExpiredOrders,
		&stats.CancelledOrders,
		&stats.ActiveStatusUsers,
		&stats.DisabledStatusUsers,
		&stats.RechargedUsers,
		&stats.BalanceRechargeUsers,
		&stats.SubscriptionPurchaseUsers,
		&stats.SubscriptionDailyLimit,
		&stats.SubscriptionWeeklyLimit,
		&stats.SubscriptionMonthlyLimit,
		&stats.SubscriptionDailyUsed,
		&stats.SubscriptionWeeklyUsed,
		&stats.SubscriptionMonthlyUsed,
	); err != nil {
		logger.LegacyPrintf("repository.usage_log", "GetOperationsFunnel deep query failed, fallback to base query: %v", err)
		return r.getOperationsFunnelBase(ctx, startTime, endTime)
	}
	return stats, nil
}

func (r *usageLogRepository) getOperationsFunnelBase(ctx context.Context, startTime, endTime time.Time) (*service.OperationsFunnelStats, error) {
	stats := &service.OperationsFunnelStats{}
	query := `
		WITH paid_statuses(status) AS (
			VALUES ($3), ($4), ($5)
		),
		active_users_period AS (
			SELECT DISTINCT ul.user_id
			FROM usage_logs ul
			WHERE ul.created_at >= $1
				AND ul.created_at < $2
				AND ul.actual_cost > 0
		),
		cohort AS (
			SELECT id
			FROM users
			WHERE created_at >= $1
				AND created_at < $2
				AND deleted_at IS NULL
		),
		funnel_counts AS (
			SELECT
				(SELECT COUNT(*) FROM cohort) AS registered_users,
				(
					SELECT COUNT(DISTINCT ak.user_id)
					FROM api_keys ak
					JOIN cohort c ON c.id = ak.user_id
					WHERE ak.deleted_at IS NULL
						AND ak.created_at >= $1
						AND ak.created_at < $2
				) AS created_key_users,
				(
					SELECT COUNT(DISTINCT ul.user_id)
					FROM usage_logs ul
					JOIN cohort c ON c.id = ul.user_id
					WHERE ul.created_at >= $1
						AND ul.created_at < $2
						AND ul.actual_cost > 0
				) AS active_users,
				(
					SELECT COUNT(DISTINCT po.user_id)
					FROM payment_orders po
					JOIN cohort c ON c.id = po.user_id
					WHERE po.status IN (SELECT status FROM paid_statuses)
						AND po.paid_at IS NOT NULL
						AND po.paid_at >= $1
						AND po.paid_at < $2
				) AS paying_users
		),
		all_user_stats AS (
			SELECT
				COUNT(*) AS total_users,
				COUNT(aup.user_id) AS active_users,
				COALESCE(SUM(u.balance), 0) AS remaining_balance
			FROM users u
			LEFT JOIN active_users_period aup ON aup.user_id = u.id
			WHERE u.deleted_at IS NULL
		),
		period_orders AS (
			SELECT
				COUNT(*) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2) AS paid_orders,
				COUNT(DISTINCT user_id) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2) AS period_paying_users,
				COALESCE(SUM(pay_amount) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2), 0) AS total_revenue,
				COALESCE(SUM(pay_amount) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2 AND order_type = $6), 0) AS balance_revenue,
				COUNT(*) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2 AND order_type = $6) AS balance_orders,
				COALESCE(SUM(pay_amount) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2 AND order_type = $7), 0) AS subscription_revenue,
				COUNT(*) FILTER (WHERE status IN (SELECT status FROM paid_statuses) AND paid_at >= $1 AND paid_at < $2 AND order_type = $7) AS subscription_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $8) AS pending_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $9) AS failed_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $10) AS refund_requested_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $11) AS expired_orders,
				COUNT(*) FILTER (WHERE created_at >= $1 AND created_at < $2 AND status = $12) AS cancelled_orders
			FROM payment_orders
			WHERE (created_at >= $1 AND created_at < $2)
				OR (paid_at >= $1 AND paid_at < $2)
		),
		balance_recharge_inventory AS (
			SELECT COALESCE(SUM(amount), 0) AS total_recharge_amount
			FROM payment_orders
			WHERE status IN (SELECT status FROM paid_statuses)
				AND order_type = $6
		),
		user_credit_inventory AS (
			SELECT
				COALESCE(b.total_recharge_amount, 0) AS total_recharge_amount,
				u.remaining_balance,
				GREATEST(
					(SELECT COALESCE(SUM(total_recharged), 0) FROM users WHERE deleted_at IS NULL)
						- COALESCE(b.total_recharge_amount, 0),
					0
				) AS gifted_amount
			FROM all_user_stats u
			CROSS JOIN balance_recharge_inventory b
		),
		subscription_inventory AS (
			SELECT
				COUNT(*) AS active_subscriptions,
				COUNT(DISTINCT us.user_id) AS active_subscription_users,
				COUNT(*) FILTER (
					WHERE COALESCE(g.daily_limit_usd, 0) > 0
						OR COALESCE(g.weekly_limit_usd, 0) > 0
						OR COALESCE(g.monthly_limit_usd, 0) > 0
				) AS limited_subscriptions,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.daily_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.daily_limit_usd, 0) - COALESCE(us.daily_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_daily_remaining,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.weekly_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.weekly_limit_usd, 0) - COALESCE(us.weekly_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_weekly_remaining,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.monthly_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.monthly_limit_usd, 0) - COALESCE(us.monthly_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_monthly_remaining
			FROM user_subscriptions us
			JOIN groups g ON g.id = us.group_id AND g.deleted_at IS NULL
			WHERE us.deleted_at IS NULL
				AND us.status = $13
				AND us.expires_at > NOW()
		)
		SELECT
			f.registered_users,
			f.created_key_users,
			f.active_users,
			f.paying_users,
			u.total_users,
			u.active_users,
			GREATEST(u.total_users - u.active_users, 0) AS inactive_users,
			p.period_paying_users,
			p.paid_orders,
			p.total_revenue,
			p.balance_revenue,
			p.balance_orders,
			p.subscription_revenue,
			p.subscription_orders,
			c.total_recharge_amount,
			c.remaining_balance,
			c.gifted_amount,
			s.active_subscriptions,
			s.active_subscription_users,
			s.limited_subscriptions,
			s.subscription_daily_remaining,
			s.subscription_weekly_remaining,
			s.subscription_monthly_remaining,
			p.pending_orders,
			p.failed_orders,
			p.refund_requested_orders,
			p.expired_orders,
			p.cancelled_orders
		FROM funnel_counts f
		CROSS JOIN all_user_stats u
		CROSS JOIN period_orders p
		CROSS JOIN user_credit_inventory c
		CROSS JOIN subscription_inventory s
	`
	if err := scanSingleRow(
		ctx,
		r.sql,
		query,
		[]any{
			startTime,
			endTime,
			service.OrderStatusCompleted,
			service.OrderStatusPaid,
			service.OrderStatusRecharging,
			payment.OrderTypeBalance,
			payment.OrderTypeSubscription,
			service.OrderStatusPending,
			service.OrderStatusFailed,
			service.OrderStatusRefundRequested,
			service.OrderStatusExpired,
			service.OrderStatusCancelled,
			service.SubscriptionStatusActive,
		},
		&stats.RegisteredUsers,
		&stats.CreatedKeyUsers,
		&stats.ActiveUsers,
		&stats.PayingUsers,
		&stats.TotalUsers,
		&stats.AllActiveUsers,
		&stats.AllInactiveUsers,
		&stats.PeriodPayingUsers,
		&stats.PaidOrders,
		&stats.TotalRevenue,
		&stats.BalanceRevenue,
		&stats.BalanceOrders,
		&stats.SubscriptionRevenue,
		&stats.SubscriptionOrders,
		&stats.TotalRechargeAmount,
		&stats.RemainingBalance,
		&stats.GiftedAmount,
		&stats.ActiveSubscriptions,
		&stats.ActiveSubscriptionUsers,
		&stats.LimitedSubscriptions,
		&stats.SubscriptionDailyRemaining,
		&stats.SubscriptionWeeklyRemaining,
		&stats.SubscriptionMonthlyRemaining,
		&stats.PendingOrders,
		&stats.FailedOrders,
		&stats.RefundRequestedOrders,
		&stats.ExpiredOrders,
		&stats.CancelledOrders,
	); err != nil {
		return nil, err
	}
	return stats, nil
}

func (r *usageLogRepository) ListOperationsUserDetails(ctx context.Context, filter service.OperationsUserDetailFilter) (items []service.OperationsUserDetail, total int64, err error) {
	pageSize := filter.Pagination.Limit()
	if pageSize > 100 {
		pageSize = 100
	}
	offset := filter.Pagination.Offset()
	baseQuery := `
		WITH paid_statuses(status) AS (
			VALUES ($3), ($4), ($5)
		),
		active_users_period AS (
			SELECT DISTINCT ul.user_id
			FROM usage_logs ul
			WHERE ul.created_at >= $1
				AND ul.created_at < $2
				AND ul.actual_cost > 0
		),
		period_usage AS (
			SELECT
				ul.user_id,
				COUNT(*) AS period_requests,
				COALESCE(SUM(ul.actual_cost), 0) AS period_usage_cost
			FROM usage_logs ul
			WHERE ul.created_at >= $1
				AND ul.created_at < $2
				AND ul.actual_cost > 0
			GROUP BY ul.user_id
		),
		period_orders AS (
			SELECT
				po.user_id,
				COUNT(*) AS paid_order_count,
				COALESCE(SUM(po.pay_amount), 0) AS paid_order_amount,
				COALESCE(SUM(po.pay_amount) FILTER (WHERE po.order_type = $6), 0) AS balance_order_amount,
				COALESCE(SUM(po.pay_amount) FILTER (WHERE po.order_type = $7), 0) AS subscription_order_amount
			FROM payment_orders po
			WHERE po.status IN (SELECT status FROM paid_statuses)
				AND po.paid_at IS NOT NULL
				AND po.paid_at >= $1
				AND po.paid_at < $2
			GROUP BY po.user_id
		),
		all_balance_orders AS (
			SELECT
				po.user_id,
				COALESCE(SUM(po.amount), 0) AS balance_recharged
			FROM payment_orders po
			WHERE po.status IN (SELECT status FROM paid_statuses)
				AND po.order_type = $6
			GROUP BY po.user_id
		),
		active_subscriptions AS (
			SELECT
				us.user_id,
				COUNT(*) AS active_subscription_count,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.daily_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.daily_limit_usd, 0) - COALESCE(us.daily_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_daily_remaining_usd,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.weekly_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.weekly_limit_usd, 0) - COALESCE(us.weekly_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_weekly_remaining_usd,
				COALESCE(SUM(
					CASE WHEN COALESCE(g.monthly_limit_usd, 0) > 0
						THEN GREATEST(COALESCE(g.monthly_limit_usd, 0) - COALESCE(us.monthly_usage_usd, 0), 0)
						ELSE 0
					END
				), 0) AS subscription_monthly_remaining_usd
			FROM user_subscriptions us
			JOIN groups g ON g.id = us.group_id AND g.deleted_at IS NULL
			WHERE us.deleted_at IS NULL
				AND us.status = $8
				AND us.expires_at > NOW()
			GROUP BY us.user_id
		),
		base AS (
			SELECT
				u.id AS user_id,
				u.email,
				u.username AS user_name,
				u.status,
				COALESCE(u.balance, 0) AS balance,
				COALESCE(u.total_recharged, 0) AS total_recharged,
				GREATEST(COALESCE(u.total_recharged, 0) - COALESCE(abo.balance_recharged, 0), 0) AS gifted_amount_estimate,
				u.last_active_at,
				u.created_at,
				COALESCE(pu.period_requests, 0) AS period_requests,
				COALESCE(pu.period_usage_cost, 0) AS period_usage_cost,
				COALESCE(po.paid_order_count, 0) AS paid_order_count,
				COALESCE(po.paid_order_amount, 0) AS paid_order_amount,
				COALESCE(po.balance_order_amount, 0) AS balance_order_amount,
				COALESCE(po.subscription_order_amount, 0) AS subscription_order_amount,
				COALESCE(sub.active_subscription_count, 0) AS active_subscription_count,
				COALESCE(sub.subscription_daily_remaining_usd, 0) AS subscription_daily_remaining_usd,
				COALESCE(sub.subscription_weekly_remaining_usd, 0) AS subscription_weekly_remaining_usd,
				COALESCE(sub.subscription_monthly_remaining_usd, 0) AS subscription_monthly_remaining_usd,
				aup.user_id IS NOT NULL AS is_period_active
			FROM users u
			LEFT JOIN active_users_period aup ON aup.user_id = u.id
			LEFT JOIN period_usage pu ON pu.user_id = u.id
			LEFT JOIN period_orders po ON po.user_id = u.id
			LEFT JOIN all_balance_orders abo ON abo.user_id = u.id
			LEFT JOIN active_subscriptions sub ON sub.user_id = u.id
			WHERE u.deleted_at IS NULL
		)
	`
	args := []any{
		filter.StartTime,
		filter.EndTime,
		service.OrderStatusCompleted,
		service.OrderStatusPaid,
		service.OrderStatusRecharging,
		payment.OrderTypeBalance,
		payment.OrderTypeSubscription,
		service.SubscriptionStatusActive,
	}
	segmentWhere, orderBy := operationsUserDetailSQLClause(filter.Segment)
	countQuery := baseQuery + " SELECT COUNT(*) FROM base WHERE " + segmentWhere
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []service.OperationsUserDetail{}, 0, nil
	}

	dataQuery := baseQuery + `
		SELECT
			user_id,
			email,
			user_name,
			status,
			balance,
			total_recharged,
			gifted_amount_estimate,
			last_active_at,
			created_at,
			period_requests,
			period_usage_cost,
			paid_order_count,
			paid_order_amount,
			balance_order_amount,
			subscription_order_amount,
			active_subscription_count,
			subscription_daily_remaining_usd,
			subscription_weekly_remaining_usd,
			subscription_monthly_remaining_usd
		FROM base
		WHERE ` + segmentWhere + `
		ORDER BY ` + orderBy + `
		LIMIT $9 OFFSET $10
	`
	rows, err := r.sql.QueryContext(ctx, dataQuery, append(args, pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			items = nil
		}
	}()
	items = make([]service.OperationsUserDetail, 0, pageSize)
	for rows.Next() {
		var item service.OperationsUserDetail
		var lastActiveAt sql.NullTime
		if err = rows.Scan(
			&item.UserID,
			&item.Email,
			&item.UserName,
			&item.Status,
			&item.Balance,
			&item.TotalRecharged,
			&item.GiftedAmountEstimate,
			&lastActiveAt,
			&item.CreatedAt,
			&item.PeriodRequests,
			&item.PeriodUsageCost,
			&item.PaidOrderCount,
			&item.PaidOrderAmount,
			&item.BalanceOrderAmount,
			&item.SubscriptionOrderAmount,
			&item.ActiveSubscriptionCount,
			&item.SubscriptionDailyRemainingUSD,
			&item.SubscriptionWeeklyRemainingUSD,
			&item.SubscriptionMonthlyRemainingUSD,
		); err != nil {
			return nil, 0, err
		}
		if lastActiveAt.Valid {
			item.LastActiveAt = &lastActiveAt.Time
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func operationsUserDetailSQLClause(segment string) (where string, orderBy string) {
	switch segment {
	case "active":
		return "is_period_active", "last_active_at DESC NULLS LAST, period_usage_cost DESC, user_id DESC"
	case "inactive":
		return "NOT is_period_active", "last_active_at ASC NULLS FIRST, user_id DESC"
	case "balance":
		return "balance > 0", "balance DESC, user_id DESC"
	case "recharge":
		return "total_recharged > 0", "total_recharged DESC, user_id DESC"
	case "subscription":
		return "active_subscription_count > 0", "active_subscription_count DESC, subscription_monthly_remaining_usd DESC, user_id DESC"
	default:
		return "TRUE", "user_id DESC"
	}
}

func (r *usageLogRepository) ListAdminWithFilters(ctx context.Context, params pagination.PaginationParams, filters UsageLogFilters) ([]service.UsageLog, *pagination.PaginationResult, error) {
	conditions := make([]string, 0, 10)
	args := make([]any, 0, 10)

	if filters.UserID > 0 {
		conditions = append(conditions, fmt.Sprintf("user_id = $%d", len(args)+1))
		args = append(args, filters.UserID)
	}
	if filters.APIKeyID > 0 {
		conditions = append(conditions, fmt.Sprintf("api_key_id = $%d", len(args)+1))
		args = append(args, filters.APIKeyID)
	}
	if filters.AccountID > 0 {
		conditions = append(conditions, fmt.Sprintf("account_id = $%d", len(args)+1))
		args = append(args, filters.AccountID)
	}
	if filters.GroupID > 0 {
		conditions = append(conditions, fmt.Sprintf("group_id = $%d", len(args)+1))
		args = append(args, filters.GroupID)
	}
	if strings.TrimSpace(filters.Model) != "" {
		conditions = append(conditions, fmt.Sprintf("(model = $%d OR requested_model = $%d OR upstream_model = $%d)", len(args)+1, len(args)+1, len(args)+1))
		args = append(args, filters.Model)
	}
	conditions, args = appendRequestTypeOrStreamWhereCondition(conditions, args, filters.RequestType, filters.Stream)
	if filters.BillingType != nil {
		conditions = append(conditions, fmt.Sprintf("billing_type = $%d", len(args)+1))
		args = append(args, int16(*filters.BillingType))
	}
	conditions, args = appendUsageLogBillingModeWhereCondition(conditions, args, filters.BillingMode)
	if filters.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", len(args)+1))
		args = append(args, *filters.StartTime)
	}
	if filters.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at < $%d", len(args)+1))
		args = append(args, *filters.EndTime)
	}

	whereClause := buildWhere(conditions)
	countQuery := "WITH merged AS (" + r.adminUsageMergedCTE() + ") SELECT COUNT(*) FROM merged " + whereClause
	var total int64
	if err := scanSingleRow(ctx, r.sql, countQuery, args, &total); err != nil {
		return nil, nil, err
	}

	limitPos := len(args) + 1
	offsetPos := len(args) + 2
	listArgs := append(append([]any{}, args...), params.Limit(), params.Offset())
	query := fmt.Sprintf(
		"WITH merged AS (%s) SELECT %s FROM merged %s ORDER BY %s LIMIT $%d OFFSET $%d",
		r.adminUsageMergedCTE(),
		adminMergedUsageSelectColumns,
		whereClause,
		adminUsageOrderBy(params),
		limitPos,
		offsetPos,
	)
	logs, err := r.queryAdminMergedUsageLogs(ctx, query, listArgs...)
	if err != nil {
		return nil, nil, err
	}
	if err := r.hydrateUsageLogAssociations(ctx, logs); err != nil {
		return nil, nil, err
	}
	return logs, paginationResultFromTotal(total, params), nil
}

func (r *usageLogRepository) adminUsageMergedCTE() string {
	return `
	SELECT
		ul.id,
		'success'::text AS request_kind,
		ul.created_at,
		COALESCE(NULLIF(TRIM(ul.requested_model), ''), ul.model) AS sort_model,
		ul.user_id,
		ul.api_key_id,
		ul.account_id,
		ul.request_id,
		ul.model,
		ul.requested_model,
		ul.upstream_model,
		ul.group_id,
		ul.subscription_id,
		ul.input_tokens,
		ul.output_tokens,
		ul.cache_creation_tokens,
		ul.cache_read_tokens,
		ul.cache_creation_5m_tokens,
		ul.cache_creation_1h_tokens,
		ul.image_output_tokens,
		ul.image_output_cost,
		ul.input_cost,
		ul.output_cost,
		ul.cache_creation_cost,
		ul.cache_read_cost,
		ul.total_cost,
		ul.actual_cost,
		ul.rate_multiplier,
		ul.account_rate_multiplier,
		ul.billing_type,
		ul.request_type,
		ul.stream,
		ul.openai_ws_mode,
		ul.duration_ms,
		ul.first_token_ms,
		ul.user_agent,
		CASE
			WHEN ul.ip_address IS NULL THEN NULL::text
			ELSE ul.ip_address::text
		END AS ip_address,
		ul.image_count,
		ul.image_size,
		ul.image_input_size,
		ul.image_output_size,
		ul.image_size_source,
		ul.image_size_breakdown,
		ul.service_tier,
		ul.reasoning_effort,
		ul.inbound_endpoint,
		ul.upstream_endpoint,
		ul.cache_ttl_overridden,
		ul.channel_id,
		ul.model_mapping_chain,
		ul.billing_tier,
		ul.billing_mode,
		ul.account_stats_cost,
		NULL::integer AS status_code,
		NULL::text AS error_message,
		NULL::text AS error_phase,
		NULL::text AS error_severity
	FROM usage_logs ul

	UNION ALL

	SELECT
		-o.id AS id,
		'error'::text AS request_kind,
		o.created_at,
		COALESCE(NULLIF(TRIM(o.requested_model), ''), NULLIF(TRIM(o.model), ''), NULLIF(TRIM(o.upstream_model), '')) AS sort_model,
		COALESCE(o.user_id, 0) AS user_id,
		COALESCE(o.api_key_id, 0) AS api_key_id,
		COALESCE(o.account_id, 0) AS account_id,
		COALESCE(NULLIF(o.request_id, ''), NULLIF(o.client_request_id, ''), '') AS request_id,
		COALESCE(NULLIF(o.requested_model, ''), NULLIF(o.model, ''), NULLIF(o.upstream_model, ''), '') AS model,
		NULLIF(o.requested_model, '') AS requested_model,
		NULLIF(o.upstream_model, '') AS upstream_model,
		o.group_id,
		NULL::bigint AS subscription_id,
		0 AS input_tokens,
		0 AS output_tokens,
		0 AS cache_creation_tokens,
		0 AS cache_read_tokens,
		0 AS cache_creation_5m_tokens,
		0 AS cache_creation_1h_tokens,
		0 AS image_output_tokens,
		0::numeric AS image_output_cost,
		0::numeric AS input_cost,
		0::numeric AS output_cost,
		0::numeric AS cache_creation_cost,
		0::numeric AS cache_read_cost,
		0::numeric AS total_cost,
		0::numeric AS actual_cost,
		1::numeric AS rate_multiplier,
		NULL::numeric AS account_rate_multiplier,
		0::smallint AS billing_type,
		COALESCE(o.request_type, 0)::smallint AS request_type,
		COALESCE(o.stream, false) AS stream,
		false AS openai_ws_mode,
		o.duration_ms,
		COALESCE(o.time_to_first_token_ms::integer, NULL) AS first_token_ms,
		NULLIF(o.user_agent, '') AS user_agent,
		CASE
			WHEN o.client_ip IS NULL OR o.client_ip::text = '' THEN NULL::text
			ELSE o.client_ip::text
		END AS ip_address,
		0 AS image_count,
		NULL::text AS image_size,
		NULL::text AS image_input_size,
		NULL::text AS image_output_size,
		NULL::text AS image_size_source,
		NULL::jsonb AS image_size_breakdown,
		NULL::text AS service_tier,
		NULL::text AS reasoning_effort,
		NULLIF(o.inbound_endpoint, '') AS inbound_endpoint,
		NULLIF(o.upstream_endpoint, '') AS upstream_endpoint,
		false AS cache_ttl_overridden,
		NULL::bigint AS channel_id,
		NULL::text AS model_mapping_chain,
		NULL::text AS billing_tier,
		NULL::text AS billing_mode,
		NULL::numeric AS account_stats_cost,
		o.status_code,
		NULLIF(o.error_message, '') AS error_message,
		NULLIF(o.error_phase, '') AS error_phase,
		NULLIF(o.severity, '') AS error_severity
	FROM ops_error_logs o
	WHERE COALESCE(o.status_code, 0) >= 400
	`
}

func adminUsageOrderBy(params pagination.PaginationParams) string {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := strings.ToUpper(params.NormalizedSortOrder(pagination.SortOrderDesc))

	var column string
	switch sortBy {
	case "model":
		column = "sort_model"
	case "created_at":
		column = "created_at"
	default:
		column = "created_at"
	}
	return fmt.Sprintf("%s %s, request_kind %s, request_id %s", column, sortOrder, sortOrder, sortOrder)
}

func (r *usageLogRepository) queryAdminMergedUsageLogs(ctx context.Context, query string, args ...any) (logs []service.UsageLog, err error) {
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
			logs = nil
		}
	}()

	logs = make([]service.UsageLog, 0)
	for rows.Next() {
		log, scanErr := scanAdminMergedUsageLog(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		logs = append(logs, *log)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

func scanAdminMergedUsageLog(scanner interface{ Scan(...any) error }) (*service.UsageLog, error) {
	var (
		id                    int64
		requestKind           string
		createdAt             time.Time
		sortModel             sql.NullString
		userID                int64
		apiKeyID              int64
		accountID             int64
		requestID             sql.NullString
		model                 string
		requestedModel        sql.NullString
		upstreamModel         sql.NullString
		groupID               sql.NullInt64
		subscriptionID        sql.NullInt64
		inputTokens           int
		outputTokens          int
		cacheCreationTokens   int
		cacheReadTokens       int
		cacheCreation5m       int
		cacheCreation1h       int
		imageOutputTokens     int
		imageOutputCost       float64
		inputCost             float64
		outputCost            float64
		cacheCreationCost     float64
		cacheReadCost         float64
		totalCost             float64
		actualCost            float64
		rateMultiplier        float64
		accountRateMultiplier sql.NullFloat64
		billingType           int16
		requestTypeRaw        int16
		stream                bool
		openaiWSMode          bool
		durationMs            sql.NullInt64
		firstTokenMs          sql.NullInt64
		userAgent             sql.NullString
		ipAddress             sql.NullString
		imageCount            int
		imageSize             sql.NullString
		imageInputSize        sql.NullString
		imageOutputSize       sql.NullString
		imageSizeSource       sql.NullString
		imageSizeBreakdown    sql.NullString
		serviceTier           sql.NullString
		reasoningEffort       sql.NullString
		inboundEndpoint       sql.NullString
		upstreamEndpoint      sql.NullString
		cacheTTLOverridden    bool
		channelID             sql.NullInt64
		modelMappingChain     sql.NullString
		billingTier           sql.NullString
		billingMode           sql.NullString
		accountStatsCost      sql.NullFloat64
		statusCode            sql.NullInt64
		errorMessage          sql.NullString
		errorPhase            sql.NullString
		errorSeverity         sql.NullString
	)

	if err := scanner.Scan(
		&id,
		&requestKind,
		&createdAt,
		&sortModel,
		&userID,
		&apiKeyID,
		&accountID,
		&requestID,
		&model,
		&requestedModel,
		&upstreamModel,
		&groupID,
		&subscriptionID,
		&inputTokens,
		&outputTokens,
		&cacheCreationTokens,
		&cacheReadTokens,
		&cacheCreation5m,
		&cacheCreation1h,
		&imageOutputTokens,
		&imageOutputCost,
		&inputCost,
		&outputCost,
		&cacheCreationCost,
		&cacheReadCost,
		&totalCost,
		&actualCost,
		&rateMultiplier,
		&accountRateMultiplier,
		&billingType,
		&requestTypeRaw,
		&stream,
		&openaiWSMode,
		&durationMs,
		&firstTokenMs,
		&userAgent,
		&ipAddress,
		&imageCount,
		&imageSize,
		&imageInputSize,
		&imageOutputSize,
		&imageSizeSource,
		&imageSizeBreakdown,
		&serviceTier,
		&reasoningEffort,
		&inboundEndpoint,
		&upstreamEndpoint,
		&cacheTTLOverridden,
		&channelID,
		&modelMappingChain,
		&billingTier,
		&billingMode,
		&accountStatsCost,
		&statusCode,
		&errorMessage,
		&errorPhase,
		&errorSeverity,
	); err != nil {
		return nil, err
	}

	log := &service.UsageLog{
		ID:                    id,
		RequestKind:           requestKind,
		UserID:                userID,
		APIKeyID:              apiKeyID,
		AccountID:             accountID,
		Model:                 model,
		RequestedModel:        coalesceTrimmedString(requestedModel, coalesceTrimmedString(sortModel, model)),
		InputTokens:           inputTokens,
		OutputTokens:          outputTokens,
		CacheCreationTokens:   cacheCreationTokens,
		CacheReadTokens:       cacheReadTokens,
		CacheCreation5mTokens: cacheCreation5m,
		CacheCreation1hTokens: cacheCreation1h,
		ImageOutputTokens:     imageOutputTokens,
		ImageOutputCost:       imageOutputCost,
		InputCost:             inputCost,
		OutputCost:            outputCost,
		CacheCreationCost:     cacheCreationCost,
		CacheReadCost:         cacheReadCost,
		TotalCost:             totalCost,
		ActualCost:            actualCost,
		RateMultiplier:        rateMultiplier,
		AccountRateMultiplier: nullFloat64Ptr(accountRateMultiplier),
		BillingType:           int8(billingType),
		RequestType:           service.RequestTypeFromInt16(requestTypeRaw),
		ImageCount:            imageCount,
		CacheTTLOverridden:    cacheTTLOverridden,
		CreatedAt:             createdAt,
		StatusCode:            nullIntPtr(statusCode),
		ErrorMessage:          nullStringPtr(errorMessage),
		ErrorPhase:            nullStringPtr(errorPhase),
		ErrorSeverity:         nullStringPtr(errorSeverity),
	}
	log.Stream = stream
	log.OpenAIWSMode = openaiWSMode
	log.RequestType = log.EffectiveRequestType()
	log.Stream, log.OpenAIWSMode = service.ApplyLegacyRequestFields(log.RequestType, stream, openaiWSMode)

	if requestID.Valid {
		log.RequestID = requestID.String
	}
	if upstreamModel.Valid {
		log.UpstreamModel = strPtr(strings.TrimSpace(upstreamModel.String))
	}
	if groupID.Valid {
		value := groupID.Int64
		log.GroupID = &value
	}
	if subscriptionID.Valid {
		value := subscriptionID.Int64
		log.SubscriptionID = &value
	}
	if durationMs.Valid {
		value := int(durationMs.Int64)
		log.DurationMs = &value
	}
	if firstTokenMs.Valid {
		value := int(firstTokenMs.Int64)
		log.FirstTokenMs = &value
	}
	if userAgent.Valid {
		log.UserAgent = strPtr(userAgent.String)
	}
	if ipAddress.Valid {
		log.IPAddress = strPtr(ipAddress.String)
	}
	if imageSize.Valid {
		log.ImageSize = strPtr(imageSize.String)
	}
	if imageInputSize.Valid {
		log.ImageInputSize = strPtr(imageInputSize.String)
	}
	if imageOutputSize.Valid {
		log.ImageOutputSize = strPtr(imageOutputSize.String)
	}
	if imageSizeSource.Valid {
		log.ImageSizeSource = strPtr(imageSizeSource.String)
	}
	if imageSizeBreakdown.Valid && imageSizeBreakdown.String != "" {
		var breakdown map[string]int
		if err := json.Unmarshal([]byte(imageSizeBreakdown.String), &breakdown); err == nil {
			log.ImageSizeBreakdown = breakdown
		}
	}
	if serviceTier.Valid {
		log.ServiceTier = strPtr(serviceTier.String)
	}
	if reasoningEffort.Valid {
		log.ReasoningEffort = strPtr(reasoningEffort.String)
	}
	if inboundEndpoint.Valid {
		log.InboundEndpoint = strPtr(inboundEndpoint.String)
	}
	if upstreamEndpoint.Valid {
		log.UpstreamEndpoint = strPtr(upstreamEndpoint.String)
	}
	if channelID.Valid {
		value := channelID.Int64
		log.ChannelID = &value
	}
	if modelMappingChain.Valid {
		log.ModelMappingChain = strPtr(modelMappingChain.String)
	}
	if billingTier.Valid {
		log.BillingTier = strPtr(billingTier.String)
	}
	if billingMode.Valid {
		log.BillingMode = strPtr(billingMode.String)
	}
	if accountStatsCost.Valid {
		value := accountStatsCost.Float64
		log.AccountStatsCost = &value
	}

	return log, nil
}

func nullIntPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	i := int(v.Int64)
	return &i
}

func nullStringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return strPtr(v.String)
}

func strPtr(v string) *string {
	value := strings.TrimSpace(v)
	return &value
}
