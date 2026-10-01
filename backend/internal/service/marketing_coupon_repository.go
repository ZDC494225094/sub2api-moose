// Compatibility aliases keep the existing host DTOs and repository adapters source-compatible.
// Marketing rules and data contracts are owned by the independent extension module.
package service

import "github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"

type CouponTemplateRepository = marketing.CouponTemplateRepository
type UserCouponRepository = marketing.UserCouponRepository
type PaymentOrderDiscountRepository = marketing.PaymentOrderDiscountRepository
