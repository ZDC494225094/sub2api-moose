//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type couponTemplateRepoStub struct{}

func (s *couponTemplateRepoStub) Create(context.Context, *CouponTemplate) error {
	panic("unexpected Create call")
}

func (s *couponTemplateRepoStub) Update(context.Context, *CouponTemplate) error {
	panic("unexpected Update call")
}

func (s *couponTemplateRepoStub) GetByID(context.Context, int64) (*CouponTemplate, error) {
	panic("unexpected GetByID call")
}

func (s *couponTemplateRepoStub) List(context.Context, pagination.PaginationParams, CouponTemplateListFilter) ([]CouponTemplate, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

type userCouponRepoStubForMarketing struct {
	coupon           *UserCoupon
	getErr           error
	reserveOK        bool
	reserveErr       error
	releaseErr       error
	reserveCalls     int
	releaseCalls     int
	reservedCouponID int64
	reservedOrderID  int64
	releasedOrderID  int64
	lastReservedAt   time.Time
	lastReleasedAt   time.Time
}

func (s *userCouponRepoStubForMarketing) Create(context.Context, *UserCoupon) error {
	panic("unexpected Create call")
}

func (s *userCouponRepoStubForMarketing) GetByID(context.Context, int64) (*UserCoupon, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.coupon == nil {
		return nil, ErrUserCouponNotFound
	}
	cloned := *s.coupon
	return &cloned, nil
}

func (s *userCouponRepoStubForMarketing) GetByCode(context.Context, string) (*UserCoupon, error) {
	panic("unexpected GetByCode call")
}

func (s *userCouponRepoStubForMarketing) ListByUser(context.Context, int64, pagination.PaginationParams, UserCouponListFilter) ([]UserCoupon, *pagination.PaginationResult, error) {
	panic("unexpected ListByUser call")
}

func (s *userCouponRepoStubForMarketing) ReserveForOrder(_ context.Context, couponID int64, orderID int64, reservedAt time.Time) (bool, error) {
	s.reserveCalls++
	s.reservedCouponID = couponID
	s.reservedOrderID = orderID
	s.lastReservedAt = reservedAt
	if s.reserveErr != nil {
		return false, s.reserveErr
	}
	return s.reserveOK, nil
}

func (s *userCouponRepoStubForMarketing) ReleaseReservationByOrderID(_ context.Context, orderID int64, releasedAt time.Time) error {
	s.releaseCalls++
	s.releasedOrderID = orderID
	s.lastReleasedAt = releasedAt
	return s.releaseErr
}

func (s *userCouponRepoStubForMarketing) MarkUsedByOrderID(context.Context, int64, time.Time) error {
	panic("unexpected MarkUsedByOrderID call")
}

type paymentOrderDiscountRepoStub struct {
	createErr       error
	created         *PaymentOrderDiscount
	createCallCount int
}

func (s *paymentOrderDiscountRepoStub) Create(_ context.Context, item *PaymentOrderDiscount) error {
	s.createCallCount++
	if s.createErr != nil {
		return s.createErr
	}
	cloned := *item
	s.created = &cloned
	return nil
}

func (s *paymentOrderDiscountRepoStub) GetByOrderID(context.Context, int64) (*PaymentOrderDiscount, error) {
	panic("unexpected GetByOrderID call")
}

func (s *paymentOrderDiscountRepoStub) MarkReleasedByOrderID(context.Context, int64, time.Time) error {
	panic("unexpected MarkReleasedByOrderID call")
}

func (s *paymentOrderDiscountRepoStub) MarkUsedByOrderID(context.Context, int64, time.Time) error {
	panic("unexpected MarkUsedByOrderID call")
}

func TestCouponServiceEvaluateCouponForOrder_UsesActualDiscountWhenClampedToMinimum(t *testing.T) {
	svc := &CouponService{}
	coupon := &UserCoupon{
		ID:              10,
		UserID:          99,
		Scope:           CouponScopeBalance,
		DiscountAmount:  10,
		ThresholdAmount: 0,
		Status:          UserCouponStatusUnused,
	}

	result, err := svc.evaluateCouponForOrder(coupon, ApplyPaymentCouponInput{
		UserID:      99,
		OrderType:   "balance",
		OrderAmount: 5,
	})
	require.NoError(t, err)
	require.Equal(t, 0.01, result.DiscountedAmount)
	require.Equal(t, 4.99, result.DiscountAmount)
}

func TestCouponServiceReserveCouponForOrder_ReleasesCouponWhenDiscountRecordCreateFails(t *testing.T) {
	userCouponRepo := &userCouponRepoStubForMarketing{
		coupon: &UserCoupon{
			ID:              11,
			TemplateID:      7,
			UserID:          5,
			CouponCode:      "ABC123",
			Scope:           CouponScopeBalance,
			DiscountAmount:  10,
			ThresholdAmount: 0,
			Status:          UserCouponStatusUnused,
		},
		reserveOK: true,
	}
	discountRepo := &paymentOrderDiscountRepoStub{
		createErr: errors.New("insert discount failed"),
	}
	svc := NewCouponService(&couponTemplateRepoStub{}, userCouponRepo, discountRepo)

	result, err := svc.ReserveCouponForOrder(context.Background(), 123, ApplyPaymentCouponInput{
		UserID:       5,
		OrderType:    "balance",
		OrderAmount:  5,
		UserCouponID: 11,
	})
	require.Nil(t, result)
	require.Error(t, err)
	require.ErrorContains(t, err, "insert discount failed")
	require.Equal(t, 1, userCouponRepo.reserveCalls)
	require.Equal(t, 1, userCouponRepo.releaseCalls)
	require.Equal(t, int64(123), userCouponRepo.releasedOrderID)
}

func TestCouponServiceReserveCouponForOrder_UsesActualDiscountAmountInReservation(t *testing.T) {
	userCouponRepo := &userCouponRepoStubForMarketing{
		coupon: &UserCoupon{
			ID:              22,
			TemplateID:      8,
			UserID:          6,
			CouponCode:      "XYZ789",
			Scope:           CouponScopeSubscription,
			DiscountAmount:  10,
			ThresholdAmount: 0,
			Status:          UserCouponStatusUnused,
		},
		reserveOK: true,
	}
	discountRepo := &paymentOrderDiscountRepoStub{}
	svc := NewCouponService(&couponTemplateRepoStub{}, userCouponRepo, discountRepo)

	result, err := svc.ReserveCouponForOrder(context.Background(), 456, ApplyPaymentCouponInput{
		UserID:       6,
		OrderType:    "subscription",
		OrderAmount:  5,
		UserCouponID: 22,
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 4.99, result.DiscountAmount)
	require.NotNil(t, discountRepo.created)
	require.Equal(t, 4.99, discountRepo.created.DiscountAmount)
	require.Equal(t, 0.01, discountRepo.created.DiscountedAmount)
	require.Equal(t, 0, userCouponRepo.releaseCalls)
}
