//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestPaymentCreationFailurePreservesLifecycleAndError(t *testing.T) {
	for _, status := range []string{OrderStatusPending, OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted, OrderStatusCancelled, OrderStatusExpired, OrderStatusFailed, OrderStatusRefunding, OrderStatusRefunded} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, status, time.Now())
			if status == OrderStatusPending {
				var err error
				order, err = client.PaymentOrder.UpdateOneID(order.ID).ClearPaidAt().Save(ctx)
				require.NoError(t, err)
			}
			// Compare persisted values, not the application clock monotonic component.
			order, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			svc := &PaymentService{entClient: client, couponService: newMarketingTestCouponService(nil, nil, nil)}
			cause := infraerrors.ServiceUnavailable("PROVIDER_TIMEOUT", "secret-provider-url").WithMetadata(map[string]string{"provider": "test"})
			require.Same(t, cause, svc.recordPaymentCreationFailure(ctx, order.ID, cause))
			current, err := client.PaymentOrder.Get(ctx, order.ID)
			require.NoError(t, err)
			require.Equal(t, order.Status, current.Status)
			require.Equal(t, order.PaidAt, current.PaidAt)
			require.Equal(t, order.PaymentTradeNo, current.PaymentTradeNo)
			require.Equal(t, order.UpdatedAt, current.UpdatedAt)
			require.Equal(t, order.FailedReason, current.FailedReason)
			logs, err := svc.GetOrderAuditLogs(ctx, order.ID)
			require.NoError(t, err)
			require.Len(t, logs, 1)
			require.Equal(t, "ORDER_CREATE_RESPONSE_FAILED", logs[0].Action)
			require.Contains(t, logs[0].Detail, "unconfirmed")
			require.NotContains(t, logs[0].Detail, "secret-provider-url")
		})
	}
}

func TestPaymentCreationFailureAuditFailurePreservesOriginalError(t *testing.T) {
	client := newPaymentConfigServiceTestClient(t)
	order := createPaymentFulfillmentSubscriptionOrder(t, context.Background(), client, OrderStatusPending, time.Now())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cause := errors.New("provider timeout")
	svc := &PaymentService{entClient: client}
	require.Same(t, cause, svc.recordPaymentCreationFailure(ctx, order.ID, cause))
	current, err := client.PaymentOrder.Get(context.Background(), order.ID)
	require.NoError(t, err)
	require.Equal(t, OrderStatusPending, current.Status)
}
