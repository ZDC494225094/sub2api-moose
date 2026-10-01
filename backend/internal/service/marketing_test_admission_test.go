package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
)

type marketingTestAdmission struct{}

func (marketingTestAdmission) RequireNewBusiness(context.Context) error { return nil }
func newMarketingTestCouponService(t CouponTemplateRepository, c UserCouponRepository, d PaymentOrderDiscountRepository) *CouponService {
	return marketing.NewCouponServiceWithAdmission(t, c, d, marketingTestAdmission{})
}
