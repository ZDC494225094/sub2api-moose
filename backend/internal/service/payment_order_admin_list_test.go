//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestAdminListOrdersFiltersByCreatedAtRange(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)

	user, err := client.User.Create().
		SetEmail("admin-list-orders@example.com").
		SetPasswordHash("hash").
		SetUsername("admin-list-orders").
		Save(ctx)
	require.NoError(t, err)

	createOrder := func(outTradeNo string, createdAt time.Time) {
		t.Helper()
		_, err := client.PaymentOrder.Create().
			SetUserID(user.ID).
			SetUserEmail(user.Email).
			SetUserName(user.Username).
			SetAmount(10).
			SetPayAmount(10).
			SetFeeRate(0).
			SetRechargeCode(outTradeNo).
			SetOutTradeNo(outTradeNo).
			SetPaymentType(payment.TypeAlipay).
			SetPaymentTradeNo("").
			SetOrderType(payment.OrderTypeBalance).
			SetStatus(OrderStatusPending).
			SetExpiresAt(createdAt.Add(time.Hour)).
			SetClientIP("127.0.0.1").
			SetSrcHost("api.example.com").
			SetCreatedAt(createdAt).
			Save(ctx)
		require.NoError(t, err)
	}

	createOrder("sub2_before_range", time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
	createOrder("sub2_inside_range", time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC))
	createOrder("sub2_after_range", time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC))

	start := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	svc := &PaymentService{entClient: client}

	orders, total, err := svc.AdminListOrders(ctx, 0, OrderListParams{
		Page:      1,
		PageSize:  20,
		StartTime: &start,
		EndTime:   &end,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, orders, 1)
	require.Equal(t, "sub2_inside_range", orders[0].OutTradeNo)
}

func TestAdminListOrdersFiltersByPaidAtRange(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)

	user, err := client.User.Create().
		SetEmail("admin-list-orders-paid-at@example.com").
		SetPasswordHash("hash").
		SetUsername("admin-list-orders-paid-at").
		Save(ctx)
	require.NoError(t, err)

	createOrder := func(outTradeNo string, paidAt *time.Time) {
		t.Helper()
		create := client.PaymentOrder.Create().
			SetUserID(user.ID).
			SetUserEmail(user.Email).
			SetUserName(user.Username).
			SetAmount(20).
			SetPayAmount(20).
			SetFeeRate(0).
			SetRechargeCode(outTradeNo).
			SetOutTradeNo(outTradeNo).
			SetPaymentType(payment.TypeAlipay).
			SetPaymentTradeNo("").
			SetOrderType(payment.OrderTypeBalance).
			SetStatus(OrderStatusCompleted).
			SetExpiresAt(time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)).
			SetClientIP("127.0.0.1").
			SetSrcHost("api.example.com").
			SetCreatedAt(time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))
		if paidAt != nil {
			create.SetPaidAt(*paidAt)
		}
		_, err := create.Save(ctx)
		require.NoError(t, err)
	}

	before := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	inside := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	after := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	createOrder("sub2_paid_before_range", &before)
	createOrder("sub2_paid_inside_range", &inside)
	createOrder("sub2_paid_after_range", &after)
	createOrder("sub2_paid_at_null", nil)

	start := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	svc := &PaymentService{entClient: client}

	orders, total, err := svc.AdminListOrders(ctx, 0, OrderListParams{
		Page:      1,
		PageSize:  20,
		DateField: "paid_at",
		StartTime: &start,
		EndTime:   &end,
	})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, orders, 1)
	require.Equal(t, "sub2_paid_inside_range", orders[0].OutTradeNo)
}

func TestAdminListOrdersFinanceOnly(t *testing.T) {
	ctx := context.Background()
	client := newPaymentOrderLifecycleTestClient(t)
	user, err := client.User.Create().SetEmail("finance-filter@example.com").SetPasswordHash("hash").SetUsername("finance-filter").Save(ctx)
	require.NoError(t, err)
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)
	statuses := []string{OrderStatusPending, OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted, OrderStatusExpired, OrderStatusCancelled, OrderStatusFailed, OrderStatusRefundRequested, OrderStatusRefunding, OrderStatusRefundPending, OrderStatusPartiallyRefunded, OrderStatusRefunded, OrderStatusRefundFailed}
	for _, status := range statuses {
		for _, kind := range []string{payment.OrderTypeBalance, payment.OrderTypeSubscription, "other"} {
			id := status + "_" + kind
			_, err := client.PaymentOrder.Create().SetUserID(user.ID).SetUserEmail(user.Email).SetUserName(user.Username).
				SetAmount(10).SetPayAmount(10).SetFeeRate(0).SetRechargeCode(id).SetOutTradeNo(id).SetPaymentType(payment.TypeAlipay).
				SetPaymentTradeNo("").SetOrderType(kind).SetStatus(status).SetExpiresAt(end).SetClientIP("127.0.0.1").SetSrcHost("api.example.com").SetCreatedAt(start).Save(ctx)
			require.NoError(t, err)
		}
	}
	svc := &PaymentService{entClient: client}
	rows, total, err := svc.AdminListOrders(ctx, 0, OrderListParams{Page: 1, PageSize: 20, FinanceOnly: true, StartTime: &start, EndTime: &end})
	require.NoError(t, err)
	require.Equal(t, 6, total)
	require.Len(t, rows, 6)
	amount := 0.0
	for _, row := range rows {
		require.Contains(t, []string{OrderStatusPaid, OrderStatusRecharging, OrderStatusCompleted}, row.Status)
		require.Contains(t, []string{payment.OrderTypeBalance, payment.OrderTypeSubscription}, row.OrderType)
		amount += row.Amount
	}
	require.Equal(t, 60.0, amount)
	// The operational filter must not change the ordinary order management listing.
	_, total, err = svc.AdminListOrders(ctx, 0, OrderListParams{Page: 1, PageSize: 20, StartTime: &start, EndTime: &end})
	require.NoError(t, err)
	require.Equal(t, len(statuses)*3, total)
}
