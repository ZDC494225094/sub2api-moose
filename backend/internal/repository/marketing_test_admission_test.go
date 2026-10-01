package repository

import (
	"context"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type marketingTestAdmission struct{}

func (marketingTestAdmission) RequireNewBusiness(context.Context) error { return nil }
func newRepositoryMarketingTestCouponService(t service.CouponTemplateRepository, c service.UserCouponRepository, d service.PaymentOrderDiscountRepository) *service.CouponService {
	return marketing.NewCouponServiceWithAdmission(t, c, d, marketingTestAdmission{})
}
func newRepositoryMarketingTestLotteryService(client *dbent.Client, activities service.LotteryActivityRepository, prizes service.LotteryPrizeRepository, states service.LotteryUserStateRepository, chances service.LotteryChanceLogRepository, records service.LotteryDrawRecordRepository, progress service.LotteryConsumeProgressRepository, users service.UserRepository, redeem *service.RedeemService, coupons *service.CouponService) *service.LotteryService {
	return service.NewLotteryServiceWithAdmission(client, activities, prizes, states, chances, records, progress, users, redeem, coupons, marketingTestAdmission{})
}
