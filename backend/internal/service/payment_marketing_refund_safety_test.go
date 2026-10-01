//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// Characterize the host's current safety boundary before adding exceptional
// refunds. PaidAt is payment evidence, not proof of balance/subscription delivery.
func TestMarketingFailedRefundCannotBypassHostSafety(t *testing.T) {
	for _, kind := range []string{payment.OrderTypeBalance, payment.OrderTypeSubscription} {
		for _, force := range []bool{false, true} {
			t.Run(kind+map[bool]string{false: "/normal", true: "/force"}[force], func(t *testing.T) {
				ctx := context.Background()
				client := newPaymentConfigServiceTestClient(t)
				order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusFailed, time.Now())
				order, err := client.PaymentOrder.UpdateOneID(order.ID).SetOrderType(kind).SetPayAmount(60).Save(ctx)
				require.NoError(t, err)
				require.NotNil(t, order.PaidAt)
				seedRefundCouponHistory(t, ctx, client, order)
				// Failed fulfillment retains reservations, rather than returning a paid coupon.
				_, err = client.ExecContext(ctx, `UPDATE user_coupons SET status='reserved',used_order_id=NULL,used_at=NULL,reserved_order_id=$1 WHERE id=9`, order.ID)
				require.NoError(t, err)
				_, err = client.ExecContext(ctx, `UPDATE payment_order_discounts SET status='reserved',used_at=NULL WHERE id=11`)
				require.NoError(t, err)
				before := refundCouponHistorySnapshot(t, ctx, client)
				svc := &PaymentService{entClient: client, couponService: marketing.NewCouponServiceWithAdmission(nil, nil, nil, refundAdmissionMustNotRun{t})}
				plan, result, err := svc.PrepareRefund(ctx, order.ID, 80, "failed fulfillment", force, true)
				require.Equal(t, "INVALID_STATUS", infraerrors.Reason(err))
				require.Nil(t, plan)
				require.Nil(t, result)
				// A stale or manually supplied plan cannot bypass the execution-time claim.
				result, err = svc.ExecuteRefund(ctx, &RefundPlan{OrderID: order.ID, Order: order, RefundAmount: 80, GatewayAmount: 60, Force: force, DeductBalance: true, DeductionType: map[string]string{payment.OrderTypeBalance: payment.DeductionTypeBalance, payment.OrderTypeSubscription: payment.DeductionTypeSubscription}[kind], BalanceToDeduct: 80, SubDaysToDeduct: 30, SubscriptionID: 1})
				require.Equal(t, "CONFLICT", infraerrors.Reason(err))
				require.Nil(t, result)
				current, err := client.PaymentOrder.Get(ctx, order.ID)
				require.NoError(t, err)
				require.Equal(t, OrderStatusFailed, current.Status)
				require.Equal(t, order.PaidAt, current.PaidAt)
				require.Equal(t, before, refundCouponHistorySnapshot(t, ctx, client))
				count, err := client.PaymentAuditLog.Query().Count(ctx)
				require.NoError(t, err)
				require.Zero(t, count)
			})
		}
	}
}

func TestMarketingHistoricalRefundAndFulfillmentClaimsRemainExclusive(t *testing.T) {
	for _, status := range []string{OrderStatusRefundRequested, OrderStatusRefundPending, OrderStatusRefunding, OrderStatusRefundFailed, OrderStatusRefunded, OrderStatusPartiallyRefunded} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			stale := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusFailed, time.Now())
			_, err := client.PaymentOrder.UpdateOneID(stale.ID).SetStatus(status).Save(ctx)
			require.NoError(t, err)
			seedRefundCouponHistory(t, ctx, client, stale)
			before := refundCouponHistorySnapshot(t, ctx, client)
			svc := &PaymentService{entClient: client, couponService: marketing.NewCouponServiceWithAdmission(nil, nil, nil, refundAdmissionMustNotRun{t})}
			require.Equal(t, "INVALID_STATUS", infraerrors.Reason(svc.RetryFulfillment(ctx, stale.ID)))
			// Simulate an interleaving after the retry read, without claiming PG concurrency.
			lease, err := svc.acquirePaymentFulfillmentLease(ctx, stale)
			require.Equal(t, "CONFLICT", infraerrors.Reason(err))
			require.Nil(t, lease)
			current, err := client.PaymentOrder.Get(ctx, stale.ID)
			require.NoError(t, err)
			require.Equal(t, status, current.Status)
			require.Equal(t, before, refundCouponHistorySnapshot(t, ctx, client))
		})
	}
}
