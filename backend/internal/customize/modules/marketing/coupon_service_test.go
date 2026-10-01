package marketing

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

type couponTrace struct {
	t     *testing.T
	ctx   context.Context
	steps []string
	fail  string
	err   error
}

func (r *couponTrace) step(ctx context.Context, name string) error {
	r.t.Helper()
	require.Equal(r.t, r.ctx, ctx, "the caller/transaction context must reach every repository")
	r.steps = append(r.steps, name)
	if name == r.fail {
		return r.err
	}
	return nil
}

type couponTemplates struct {
	CouponTemplateRepository
	trace    *couponTrace
	template *CouponTemplate
}

func (r couponTemplates) GetByID(ctx context.Context, id int64) (*CouponTemplate, error) {
	if err := r.trace.step(ctx, "template"); err != nil {
		return nil, err
	}
	return r.template, nil
}

type couponRows struct {
	UserCouponRepository
	trace  *couponTrace
	coupon *UserCoupon
	issued *UserCoupon
	claim  bool
}

func (r *couponRows) GetByID(ctx context.Context, id int64) (*UserCoupon, error) {
	if err := r.trace.step(ctx, "coupon"); err != nil {
		return nil, err
	}
	return r.coupon, nil
}
func (r *couponRows) Create(ctx context.Context, coupon *UserCoupon) error {
	if err := r.trace.step(ctx, "issue"); err != nil {
		return err
	}
	coupon.ID = 73
	r.issued = coupon
	return nil
}
func (r *couponRows) ReserveForOrder(ctx context.Context, id, order int64, at time.Time) (bool, error) {
	return r.claim, r.trace.step(ctx, "reserve")
}
func (r *couponRows) ReleaseReservationByOrderID(ctx context.Context, id int64, at time.Time) error {
	return r.trace.step(ctx, "release-coupon")
}
func (r *couponRows) MarkUsedByOrderID(ctx context.Context, id int64, at time.Time) error {
	return r.trace.step(ctx, "consume-coupon")
}

type discountRows struct {
	PaymentOrderDiscountRepository
	trace   *couponTrace
	created *PaymentOrderDiscount
}

func (r *discountRows) Create(ctx context.Context, discount *PaymentOrderDiscount) error {
	r.created = discount
	return r.trace.step(ctx, "discount")
}
func (r *discountRows) MarkReleasedByOrderID(ctx context.Context, id int64, at time.Time) error {
	return r.trace.step(ctx, "release-discount")
}
func (r *discountRows) MarkUsedByOrderID(ctx context.Context, id int64, at time.Time) error {
	return r.trace.step(ctx, "consume-discount")
}
func validCoupon() *UserCoupon {
	return &UserCoupon{ID: 7, TemplateID: 8, UserID: 42, CouponCode: "HISTORICAL", Scope: CouponScopeUniversal, DiscountAmount: 10, ThresholdAmount: 5, Status: UserCouponStatusUnused}
}

func TestCouponIssueSnapshotsTemplateAndForwardsTransactionContext(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "host transaction")
	trace := &couponTrace{t: t, ctx: ctx}
	from := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	days, ref := 7, int64(12)
	template := &CouponTemplate{ID: 8, Name: "reward", Scope: CouponScopeBalance, DiscountAmount: 10, ThresholdAmount: 20, ValidFrom: &from, ValidDays: &days, Status: CouponTemplateStatusActive}
	rows := &couponRows{trace: trace}
	svc := newTestCouponService(couponTemplates{trace: trace, template: template}, rows, nil)
	coupon, err := svc.IssueCouponFromTemplate(ctx, 8, 42, " lottery ", &ref)
	require.NoError(t, err)
	require.Same(t, rows.issued, coupon)
	require.Equal(t, []string{"template", "issue"}, trace.steps)
	require.Equal(t, UserCouponStatusUnused, coupon.Status)
	require.Equal(t, UserCouponSourceLottery, coupon.SourceType)
	require.Equal(t, ref, *coupon.SourceRefID)
	require.Equal(t, from.AddDate(0, 0, days), *coupon.ValidUntil)
	require.Regexp(t, "^[0-9A-F]{16}$", coupon.CouponCode)
	template.DiscountAmount = 99
	template.Scope = CouponScopeSubscription
	require.Equal(t, float64(10), coupon.DiscountAmount, "issued coupon monetary snapshot must not follow template edits")
	require.Equal(t, CouponScopeBalance, coupon.Scope)
}

func TestCouponIssueRejectsMissingOrDisabledTemplateWithoutIssuing(t *testing.T) {
	for _, template := range []*CouponTemplate{nil, {ID: 8, Status: CouponTemplateStatusDisabled}} {
		trace := &couponTrace{t: t, ctx: context.Background()}
		rows := &couponRows{trace: trace}
		_, err := newTestCouponService(couponTemplates{trace: trace, template: template}, rows, nil).IssueCouponFromTemplate(trace.ctx, 8, 42, UserCouponSourceLottery, nil)
		require.Error(t, err)
		require.Nil(t, rows.issued)
		require.Equal(t, []string{"template"}, trace.steps)
	}
}

func TestCouponEligibilityKeepsExistingOwnershipScopeThresholdAndExpiryRules(t *testing.T) {
	tests := []struct {
		name   string
		edit   func(*UserCoupon)
		amount float64
		want   string
	}{
		{"valid", func(*UserCoupon) {}, 20, ""},
		{"foreign user", func(c *UserCoupon) { c.UserID = 43 }, 20, "does not belong"},
		{"reserved", func(c *UserCoupon) { c.Status = UserCouponStatusReserved }, 20, "unavailable"},
		{"used", func(c *UserCoupon) { c.Status = UserCouponStatusUsed }, 20, "unavailable"},
		{"expired", func(c *UserCoupon) { v := time.Now().Add(-time.Hour); c.ValidUntil = &v }, 20, "expired"},
		{"scope", func(c *UserCoupon) { c.Scope = CouponScopeSubscription }, 20, "order type"},
		{"threshold", func(*UserCoupon) {}, 4, "threshold"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trace := &couponTrace{t: t, ctx: context.Background()}
			coupon := validCoupon()
			tt.edit(coupon)
			result, err := newTestCouponService(nil, &couponRows{trace: trace, coupon: coupon}, nil).PreviewCouponForOrder(trace.ctx, ApplyPaymentCouponInput{UserID: 42, UserCouponID: 7, OrderType: "balance", OrderAmount: tt.amount})
			if tt.want != "" {
				require.ErrorContains(t, err, tt.want)
				require.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.Equal(t, float64(10), result.DiscountAmount)
			}
			require.Equal(t, []string{"coupon"}, trace.steps)
		})
	}
}

func TestCouponReservationKeepsSnapshotAndConditionalClaim(t *testing.T) {
	for _, claim := range []bool{true, false} {
		t.Run(map[bool]string{true: "claimed", false: "another order owns coupon"}[claim], func(t *testing.T) {
			trace := &couponTrace{t: t, ctx: context.Background()}
			rows := &couponRows{trace: trace, coupon: validCoupon(), claim: claim}
			discounts := &discountRows{trace: trace}
			result, err := newTestCouponService(nil, rows, discounts).ReserveCouponForOrder(trace.ctx, 91, ApplyPaymentCouponInput{UserID: 42, UserCouponID: 7, OrderType: "balance", OrderAmount: 5})
			if !claim {
				require.ErrorIs(t, err, ErrUserCouponUnavailable)
				require.Nil(t, result)
				require.Nil(t, discounts.created)
				return
			}
			require.NoError(t, err)
			require.Equal(t, []string{"coupon", "reserve", "discount"}, trace.steps)
			require.Equal(t, 4.99, result.DiscountAmount)
			require.Equal(t, float64(5), discounts.created.OriginalAmount)
			require.Equal(t, 0.01, discounts.created.DiscountedAmount)
			require.Equal(t, OrderDiscountStatusReserved, discounts.created.Status)
			require.Equal(t, int64(91), discounts.created.OrderID)
			require.Equal(t, int64(8), *discounts.created.CouponTemplateID)
			require.Equal(t, "HISTORICAL", discounts.created.CouponCode)
		})
	}
}

func TestCouponHistoricalConsumeAndReleaseNeedNoCatalogOrSwitch(t *testing.T) {
	for _, op := range []string{"consume", "release"} {
		for _, fail := range []string{"", op + "-discount", op + "-coupon"} {
			t.Run(op+"/"+fail, func(t *testing.T) {
				type testContextKey struct{}
				ctx := context.WithValue(context.Background(), testContextKey{}, "existing order")
				trace := &couponTrace{t: t, ctx: ctx, fail: fail, err: errors.New("repository failed")}
				svc := newTestCouponService(nil, &couponRows{trace: trace}, &discountRows{trace: trace})
				var err error
				if op == "consume" {
					err = svc.ConsumeReservedCouponByOrderID(ctx, 91)
				} else {
					err = svc.ReleaseCouponReservationByOrderID(ctx, 91)
				}
				if fail == "" {
					require.NoError(t, err)
				} else {
					require.ErrorIs(t, err, trace.err)
				}
				expected := []string{op + "-discount"}
				if !strings.HasSuffix(fail, "-discount") {
					expected = append(expected, op+"-coupon")
				}
				require.Equal(t, expected, trace.steps)
			})
		}
	}
}

type recoveryDiscountRows struct {
	PaymentOrderDiscountRepository
	discount  *PaymentOrderDiscount
	restored  bool
	restoreOK bool
}

func (r *recoveryDiscountRows) GetByOrderID(context.Context, int64) (*PaymentOrderDiscount, error) {
	return r.discount, nil
}
func (r *recoveryDiscountRows) RestoreReservationByOrderID(context.Context, int64, time.Time) (bool, error) {
	r.restored = true
	return r.restoreOK, nil
}
func TestPaidCouponRecoveryPreservesHistoricalOwnership(t *testing.T) {
	for _, mode := range []string{"native", "reserved", "used", "half_used", "released", "old_failed", "other_reserved", "other_used", "wrong_user", "claim_lost", "restore_lost", "missing_coupon", "bad_snapshot"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			trace := &couponTrace{t: t, ctx: ctx}
			id, order, other := int64(7), int64(91), int64(92)
			coupon := validCoupon()
			expired := time.Now().Add(-24 * time.Hour)
			coupon.ValidUntil = &expired // paid price must not be re-quoted at current expiry
			discount := &PaymentOrderDiscount{OrderID: order, UserCouponID: &id, Status: OrderDiscountStatusReleased}
			rows := &couponRows{trace: trace, coupon: coupon, claim: true}
			snapshots := &recoveryDiscountRows{discount: discount, restoreOK: true}
			switch mode {
			case "native":
				snapshots.discount = nil
			case "reserved":
				discount.Status = OrderDiscountStatusReserved
				coupon.Status = UserCouponStatusReserved
				coupon.ReservedOrderID = &order
			case "used":
				discount.Status = OrderDiscountStatusUsed
				coupon.Status = UserCouponStatusUsed
				coupon.UsedOrderID = &order
			case "half_used":
				discount.Status = OrderDiscountStatusUsed
				coupon.Status = UserCouponStatusReserved
				coupon.ReservedOrderID = &order
			case "old_failed":
				discount.Status = OrderDiscountStatusReserved
			case "other_reserved":
				coupon.Status = UserCouponStatusReserved
				coupon.ReservedOrderID = &other
			case "other_used":
				coupon.Status = UserCouponStatusUsed
				coupon.UsedOrderID = &other
			case "wrong_user":
				coupon.UserID = 99
			case "claim_lost":
				rows.claim = false
			case "restore_lost":
				snapshots.restoreOK = false
			case "missing_coupon":
				rows.coupon = nil
			case "bad_snapshot":
				discount.UserCouponID = nil
			}
			svc := newTestCouponService(nil, rows, snapshots)
			err := svc.ReconcilePaidOrderCoupon(ctx, order, 42)
			success := mode == "native" || mode == "reserved" || mode == "used" || mode == "half_used" || mode == "released" || mode == "old_failed"
			if success {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "paid order coupon is unavailable")
			}
			require.Equal(t, mode == "released" || mode == "restore_lost", snapshots.restored)
			if mode == "other_reserved" || mode == "other_used" || mode == "wrong_user" {
				require.NotContains(t, trace.steps, "reserve")
			}
		})
	}
}
