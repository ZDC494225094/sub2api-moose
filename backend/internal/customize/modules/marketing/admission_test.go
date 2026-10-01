package marketing

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type admissionFunc func(context.Context) error

func (f admissionFunc) RequireNewBusiness(ctx context.Context) error { return f(ctx) }

type admissionTransaction struct {
	calls int
	ctx   context.Context
}

func (tx *admissionTransaction) WithinLotteryTransaction(_ context.Context, fn func(context.Context) error) error {
	tx.calls++
	return fn(tx.ctx)
}

func TestMarketingAdmissionRejectsBeforeAnyNewBusinessSideEffect(t *testing.T) {
	denied := errors.New("disabled or setting read failed")
	for _, test := range []struct {
		name      string
		admission NewBusinessAdmission
		want      error
	}{
		{"disabled", admissionFunc(func(context.Context) error { return denied }), denied},
		{"unconfigured", nil, ErrMarketingAdmissionUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			// Nil repositories deliberately panic if a guard is placed after a read/write.
			coupons := NewCouponServiceWithAdmission(nil, nil, nil, test.admission)
			_, err := coupons.IssueCouponFromTemplate(ctx, 1, 2, "lottery", nil)
			require.ErrorIs(t, err, test.want)
			_, err = coupons.PreviewCouponForOrder(ctx, ApplyPaymentCouponInput{})
			require.ErrorIs(t, err, test.want)
			_, err = coupons.ReserveCouponForOrder(ctx, 3, ApplyPaymentCouponInput{})
			require.ErrorIs(t, err, test.want)
			tx := &admissionTransaction{ctx: ctx}
			lottery := NewLotteryServiceWithAdmission[struct{}](tx, nil, nil, nil, nil, nil, nil, nil, nil, coupons, test.admission)
			_, err = lottery.Draw(ctx, LotteryDrawInput{})
			require.ErrorIs(t, err, test.want)
			require.Zero(t, tx.calls)
			state := &LotteryUserState{AvailableDrawTimes: 5, TotalGrantedTimes: 5}
			before := *state
			granted, err := lottery.tryGrantDefaultChances(ctx, &LotteryActivity{DefaultDrawTimes: 10}, state, 2, time.Now())
			require.ErrorIs(t, err, test.want)
			require.False(t, granted)
			require.Equal(t, before, *state, "disabling must not erase historical chances")
		})
	}
}

func TestLotteryAdmissionRechecksInsideHostTransaction(t *testing.T) {
	type key struct{}
	ctx := context.Background()
	txCtx := context.WithValue(ctx, key{}, "transaction")
	denied := errors.New("disabled while entering transaction")
	calls := 0
	gate := admissionFunc(func(c context.Context) error {
		calls++
		if calls == 1 {
			require.Equal(t, ctx, c)
			return nil
		}
		require.Equal(t, txCtx, c)
		return denied
	})
	tx := &admissionTransaction{ctx: txCtx}
	svc := NewLotteryServiceWithAdmission[struct{}](tx, nil, nil, nil, nil, nil, nil, nil, nil, nil, gate)
	_, err := svc.Draw(ctx, LotteryDrawInput{})
	require.ErrorIs(t, err, denied)
	require.Equal(t, 1, tx.calls)
	require.Equal(t, 2, calls)
}

func TestCouponAdmissionDoesNotInterceptHistoricalSettlement(t *testing.T) {
	ctx := context.Background()
	trace := &couponTrace{t: t, ctx: ctx}
	gate := admissionFunc(func(context.Context) error { t.Fatal("historical settlement must not consult admission"); return nil })
	svc := NewCouponServiceWithAdmission(nil, &couponRows{trace: trace}, &discountRows{trace: trace}, gate)
	require.NoError(t, svc.ConsumeReservedCouponByOrderID(ctx, 31))
	require.NoError(t, svc.ReleaseCouponReservationByOrderID(ctx, 32))
	require.Equal(t, []string{"consume-discount", "consume-coupon", "release-discount", "release-coupon"}, trace.steps)
}

func TestCouponAdmissionIsReadOnEveryNewRequest(t *testing.T) {
	ctx := context.Background()
	trace := &couponTrace{t: t, ctx: ctx}
	denied := errors.New("disabled")
	enabled := true
	gate := admissionFunc(func(c context.Context) error {
		require.Equal(t, ctx, c)
		if !enabled {
			return denied
		}
		return nil
	})
	svc := NewCouponServiceWithAdmission(nil, &couponRows{trace: trace, coupon: validCoupon()}, nil, gate)
	input := ApplyPaymentCouponInput{UserCouponID: 7, UserID: 42, OrderType: "balance", OrderAmount: 100}
	_, err := svc.PreviewCouponForOrder(ctx, input)
	require.NoError(t, err)
	enabled = false
	_, err = svc.PreviewCouponForOrder(ctx, input)
	require.ErrorIs(t, err, denied)
	require.Equal(t, []string{"coupon"}, trace.steps)
	enabled = true
	_, err = svc.PreviewCouponForOrder(ctx, input)
	require.NoError(t, err)
	require.Equal(t, []string{"coupon", "coupon"}, trace.steps)
}

func TestMarketingAdmissionRejectsAllManagementWritesBeforeRepositoryAccess(t *testing.T) {
	denied := errors.New("disabled or setting unavailable")
	for _, gate := range []NewBusinessAdmission{nil, admissionFunc(func(context.Context) error { return denied })} {
		want := denied
		if gate == nil {
			want = ErrMarketingAdmissionUnavailable
		}
		coupons := NewCouponServiceWithAdmission(nil, nil, nil, gate)
		lottery := NewLotteryServiceWithAdmission[struct{}](nil, nil, nil, nil, nil, nil, nil, nil, nil, coupons, gate)
		ctx := context.Background()
		writes := map[string]func() error{
			"create template": func() error { _, err := coupons.CreateTemplate(ctx, nil); return err },
			"update template": func() error { _, err := coupons.UpdateTemplate(ctx, 1, nil); return err },
			"create activity": func() error { _, err := lottery.CreateActivity(ctx, nil); return err },
			"update activity": func() error { _, err := lottery.UpdateActivity(ctx, 1, nil); return err },
			"delete activity": func() error { return lottery.DeleteActivity(ctx, 1) },
			"create prize":    func() error { _, err := lottery.CreatePrize(ctx, nil); return err },
			"update prize":    func() error { _, err := lottery.UpdatePrize(ctx, 1, nil); return err },
		}
		for name, write := range writes {
			t.Run(name, func(t *testing.T) { require.ErrorIs(t, write(), want) })
		}
	}
}

func TestMarketingLegacyConstructorsNeverBypassAdmission(t *testing.T) {
	ctx := context.Background()
	coupons := NewCouponService(nil, nil, nil)
	_, err := coupons.IssueCouponFromTemplate(ctx, 1, 2, "lottery", nil)
	require.ErrorIs(t, err, ErrMarketingAdmissionUnavailable)
	_, err = coupons.PreviewCouponForOrder(ctx, ApplyPaymentCouponInput{})
	require.ErrorIs(t, err, ErrMarketingAdmissionUnavailable)
	lottery := NewLotteryService[struct{}](nil, nil, nil, nil, nil, nil, nil, nil, nil, coupons)
	_, err = lottery.Draw(ctx, LotteryDrawInput{})
	require.ErrorIs(t, err, ErrMarketingAdmissionUnavailable)
}
