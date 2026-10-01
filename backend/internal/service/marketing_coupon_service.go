// CouponService is a compatibility bridge. Rules live in the marketing extension;
// historical reservation/consumption/release deliberately do not consult switches.
package service

import "github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"

var (
	ErrCouponTemplateNotFound = marketing.ErrCouponTemplateNotFound
	ErrUserCouponNotFound     = marketing.ErrUserCouponNotFound
	ErrUserCouponInvalid      = marketing.ErrUserCouponInvalid
	ErrUserCouponUnavailable  = marketing.ErrUserCouponUnavailable
)

type CouponService = marketing.CouponService

func NewCouponService(templates CouponTemplateRepository, coupons UserCouponRepository, discounts PaymentOrderDiscountRepository) *CouponService {
	return marketing.NewCouponService(templates, coupons, discounts)
}
