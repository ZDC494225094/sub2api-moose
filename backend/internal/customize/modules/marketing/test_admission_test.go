package marketing

import "context"

func newTestCouponService(t CouponTemplateRepository, c UserCouponRepository, d PaymentOrderDiscountRepository) *CouponService {
	return NewCouponServiceWithAdmission(t, c, d, admissionFunc(func(context.Context) error { return nil }))
}
