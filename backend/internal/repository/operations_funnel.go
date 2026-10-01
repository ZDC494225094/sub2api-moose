package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *operationsRepository) GetOperationsFunnel(ctx context.Context, startTime, endTime time.Time) (*service.OperationsFunnelStats, error) {
	stats := &service.OperationsFunnelStats{}
	query := `
		WITH paid_statuses(status) AS (
			VALUES ($3), ($4), ($5)
		),
		balance_recharge_by_user AS (
			SELECT user_id, COALESCE(SUM(amount), 0) AS balance_recharged
			FROM payment_orders
			WHERE status IN (SELECT status FROM paid_statuses)
				AND order_type = $6
			GROUP BY user_id
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
				COALESCE(SUM(LEAST(GREATEST(u.balance, 0), COALESCE(br.balance_recharged, 0))), 0) AS balance_recharge_remaining,
				COALESCE(SUM(GREATEST(GREATEST(u.balance, 0) - COALESCE(br.balance_recharged, 0), 0)), 0) AS gifted_remaining,
				COALESCE(SUM(GREATEST(COALESCE(u.total_recharged, 0) - COALESCE(br.balance_recharged, 0), 0)), 0) AS gifted_amount,
				COUNT(*) FILTER (WHERE u.status = $14) AS active_status_users,
				COUNT(*) FILTER (WHERE u.status = $15) AS disabled_status_users,
				COUNT(*) FILTER (WHERE COALESCE(u.total_recharged, 0) > 0) AS recharged_users
			FROM users u
			LEFT JOIN active_users_period aup ON aup.user_id = u.id
			LEFT JOIN balance_recharge_by_user br ON br.user_id = u.id
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
				COALESCE(SUM(amount) FILTER (WHERE order_type = $6), 0) AS balance_recharge_amount,
				COALESCE(SUM(amount) FILTER (WHERE order_type = $7), 0) AS subscription_recharge_amount,
				COUNT(DISTINCT user_id) FILTER (WHERE order_type = $6) AS balance_recharge_users,
				COUNT(DISTINCT user_id) FILTER (WHERE order_type = $7) AS subscription_purchase_users
			FROM payment_orders
			WHERE status IN (SELECT status FROM paid_statuses)
		),
		user_credit_inventory AS (
			SELECT
				COALESCE(b.balance_recharge_amount, 0) + COALESCE(b.subscription_recharge_amount, 0) AS total_recharge_amount,
				COALESCE(b.balance_recharge_amount, 0) AS balance_recharge_amount,
				COALESCE(b.subscription_recharge_amount, 0) AS subscription_recharge_amount,
				b.balance_recharge_users,
				b.subscription_purchase_users,
				u.remaining_balance,
				u.balance_recharge_remaining,
				u.gifted_amount,
				u.gifted_remaining
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
				COALESCE(SUM(COALESCE((
					SELECT MIN(quota.remaining)
					FROM (VALUES
						(CASE WHEN COALESCE(g.daily_limit_usd, 0) > 0 THEN GREATEST(g.daily_limit_usd - COALESCE(us.daily_usage_usd, 0), 0) END),
						(CASE WHEN COALESCE(g.weekly_limit_usd, 0) > 0 THEN GREATEST(g.weekly_limit_usd - COALESCE(us.weekly_usage_usd, 0), 0) END),
						(CASE WHEN COALESCE(g.monthly_limit_usd, 0) > 0 THEN GREATEST(g.monthly_limit_usd - COALESCE(us.monthly_usage_usd, 0), 0) END)
					) AS quota(remaining)
					WHERE quota.remaining IS NOT NULL
				), 0)), 0) AS subscription_remaining,
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
			c.balance_recharge_amount,
			c.subscription_recharge_amount,
			c.remaining_balance,
			c.balance_recharge_remaining,
			c.gifted_amount,
			c.gifted_remaining,
			s.active_subscriptions,
			s.active_subscription_users,
			s.limited_subscriptions,
			s.subscription_daily_remaining,
			s.subscription_weekly_remaining,
			s.subscription_monthly_remaining,
			s.subscription_remaining,
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
		&stats.BalanceRechargeAmount,
		&stats.SubscriptionRechargeAmount,
		&stats.RemainingBalance,
		&stats.BalanceRechargeRemaining,
		&stats.GiftedAmount,
		&stats.GiftedRemaining,
		&stats.ActiveSubscriptions,
		&stats.ActiveSubscriptionUsers,
		&stats.LimitedSubscriptions,
		&stats.SubscriptionDailyRemaining,
		&stats.SubscriptionWeeklyRemaining,
		&stats.SubscriptionMonthlyRemaining,
		&stats.SubscriptionRemaining,
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

func (r *operationsRepository) getOperationsFunnelBase(ctx context.Context, startTime, endTime time.Time) (*service.OperationsFunnelStats, error) {
	stats := &service.OperationsFunnelStats{}
	query := `
		WITH paid_statuses(status) AS (
			VALUES ($3), ($4), ($5)
		),
		balance_recharge_by_user AS (
			SELECT user_id, COALESCE(SUM(amount), 0) AS balance_recharged
			FROM payment_orders
			WHERE status IN (SELECT status FROM paid_statuses)
				AND order_type = $6
			GROUP BY user_id
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
				COALESCE(SUM(LEAST(GREATEST(u.balance, 0), COALESCE(br.balance_recharged, 0))), 0) AS balance_recharge_remaining,
				COALESCE(SUM(GREATEST(GREATEST(u.balance, 0) - COALESCE(br.balance_recharged, 0), 0)), 0) AS gifted_remaining,
				COALESCE(SUM(GREATEST(COALESCE(u.total_recharged, 0) - COALESCE(br.balance_recharged, 0), 0)), 0) AS gifted_amount
			FROM users u
			LEFT JOIN active_users_period aup ON aup.user_id = u.id
			LEFT JOIN balance_recharge_by_user br ON br.user_id = u.id
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
				COALESCE(SUM(amount) FILTER (WHERE order_type = $6), 0) AS balance_recharge_amount,
				COALESCE(SUM(amount) FILTER (WHERE order_type = $7), 0) AS subscription_recharge_amount
			FROM payment_orders
			WHERE status IN (SELECT status FROM paid_statuses)
		),
		user_credit_inventory AS (
			SELECT
				COALESCE(b.balance_recharge_amount, 0) + COALESCE(b.subscription_recharge_amount, 0) AS total_recharge_amount,
				COALESCE(b.balance_recharge_amount, 0) AS balance_recharge_amount,
				COALESCE(b.subscription_recharge_amount, 0) AS subscription_recharge_amount,
				u.remaining_balance,
				u.balance_recharge_remaining,
				u.gifted_amount,
				u.gifted_remaining
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
				COALESCE(SUM(COALESCE((
					SELECT MIN(quota.remaining)
					FROM (VALUES
						(CASE WHEN COALESCE(g.daily_limit_usd, 0) > 0 THEN GREATEST(g.daily_limit_usd - COALESCE(us.daily_usage_usd, 0), 0) END),
						(CASE WHEN COALESCE(g.weekly_limit_usd, 0) > 0 THEN GREATEST(g.weekly_limit_usd - COALESCE(us.weekly_usage_usd, 0), 0) END),
						(CASE WHEN COALESCE(g.monthly_limit_usd, 0) > 0 THEN GREATEST(g.monthly_limit_usd - COALESCE(us.monthly_usage_usd, 0), 0) END)
					) AS quota(remaining)
					WHERE quota.remaining IS NOT NULL
				), 0)), 0) AS subscription_remaining
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
			c.balance_recharge_amount,
			c.subscription_recharge_amount,
			c.remaining_balance,
			c.balance_recharge_remaining,
			c.gifted_amount,
			c.gifted_remaining,
			s.active_subscriptions,
			s.active_subscription_users,
			s.limited_subscriptions,
			s.subscription_daily_remaining,
			s.subscription_weekly_remaining,
			s.subscription_monthly_remaining,
			s.subscription_remaining,
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
		&stats.BalanceRechargeAmount,
		&stats.SubscriptionRechargeAmount,
		&stats.RemainingBalance,
		&stats.BalanceRechargeRemaining,
		&stats.GiftedAmount,
		&stats.GiftedRemaining,
		&stats.ActiveSubscriptions,
		&stats.ActiveSubscriptionUsers,
		&stats.LimitedSubscriptions,
		&stats.SubscriptionDailyRemaining,
		&stats.SubscriptionWeeklyRemaining,
		&stats.SubscriptionMonthlyRemaining,
		&stats.SubscriptionRemaining,
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

func (r *operationsRepository) ListOperationsUserDetails(ctx context.Context, filter service.OperationsUserDetailFilter) (items []service.OperationsUserDetail, total int64, err error) {
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
