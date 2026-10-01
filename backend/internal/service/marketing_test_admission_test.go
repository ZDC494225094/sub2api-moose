package service

import (
	"context"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
)

type marketingTestAdmission struct{}

func (marketingTestAdmission) RequireNewBusiness(context.Context) error { return nil }
func newMarketingTestCouponService(t CouponTemplateRepository, c UserCouponRepository, d PaymentOrderDiscountRepository) *CouponService {
	return marketing.NewCouponServiceWithAdmission(t, c, d, marketingTestAdmission{})
}
func newMarketingTestLotteryService(client *dbent.Client, activities LotteryActivityRepository, prizes LotteryPrizeRepository, states LotteryUserStateRepository, chances LotteryChanceLogRepository, records LotteryDrawRecordRepository, progress LotteryConsumeProgressRepository, users UserRepository, redeem *RedeemService, coupons *CouponService) *LotteryService {
	return NewLotteryServiceWithAdmission(client, activities, prizes, states, chances, records, progress, users, redeem, coupons, marketingTestAdmission{})
}
