// Compatibility aliases keep the existing host DTOs and repository adapters source-compatible.
// Marketing rules and data contracts are owned by the independent extension module.
package service

import "github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"

const (
	CouponScopeBalance           = marketing.CouponScopeBalance
	CouponScopeSubscription      = marketing.CouponScopeSubscription
	CouponScopeUniversal         = marketing.CouponScopeUniversal
	CouponTemplateStatusActive   = marketing.CouponTemplateStatusActive
	CouponTemplateStatusDisabled = marketing.CouponTemplateStatusDisabled
	UserCouponStatusUnused       = marketing.UserCouponStatusUnused
	UserCouponStatusReserved     = marketing.UserCouponStatusReserved
	UserCouponStatusUsed         = marketing.UserCouponStatusUsed
	UserCouponStatusExpired      = marketing.UserCouponStatusExpired
	UserCouponStatusDisabled     = marketing.UserCouponStatusDisabled
	UserCouponSourceLottery      = marketing.UserCouponSourceLottery
	OrderDiscountStatusReserved  = marketing.OrderDiscountStatusReserved
	OrderDiscountStatusReleased  = marketing.OrderDiscountStatusReleased
	OrderDiscountStatusUsed      = marketing.OrderDiscountStatusUsed
)

type CouponTemplate = marketing.CouponTemplate
type UserCoupon = marketing.UserCoupon
type PaymentOrderDiscount = marketing.PaymentOrderDiscount
type CreateCouponTemplateInput = marketing.CreateCouponTemplateInput
type UpdateCouponTemplateInput = marketing.UpdateCouponTemplateInput
type CouponTemplateListFilter = marketing.CouponTemplateListFilter
type UserCouponListFilter = marketing.UserCouponListFilter
type ApplyPaymentCouponInput = marketing.ApplyPaymentCouponInput
type ApplyPaymentCouponResult = marketing.ApplyPaymentCouponResult
type UserCouponOrderView = marketing.UserCouponOrderView
