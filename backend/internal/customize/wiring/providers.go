// Package wiring is the business-extension composition root. It is deliberately
// outside core service/handler provider sets and unrelated to OAuth provider plugins.
package wiring

import (
	"context"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/rechargecampaigns"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(
	repository.NewOperationsRepository,
	service.NewOperationsService,
	service.NewOperationsMarketingEmailService,
	service.ProvidePlaygroundRunService,
	admin.NewOperationsHandler,
	handler.NewPlaygroundHandler,
	handler.NewExtensionHandlers,
	ProvideMarketingAdmission,
	ProvideKeyRoutingAdmission,
	ProvideSubscriptionIssuance,
	ProvideSiteCustomizationAdmission,
	ProvideCouponService,
	ProvideLotteryService,
	repository.NewCouponTemplateRepository,
	repository.NewUserCouponRepository,
	repository.NewPaymentOrderDiscountRepository,
	repository.NewLotteryActivityRepository,
	repository.NewLotteryPrizeRepository,
	repository.NewLotteryUserStateRepository,
	repository.NewLotteryChanceLogRepository,
	repository.NewLotteryDrawRecordRepository,
	repository.NewLotteryConsumeProgressRepository,
	ProvideRechargeCampaigns,
	ProvideMarketingHandler,
)

func ProvideRechargeCampaigns(client *ent.Client, affiliate *service.AffiliateService) *rechargecampaigns.Handler {
	catalog := rechargecampaigns.NewService(rechargecampaigns.NewSQLRepository(client), func(ctx context.Context) bool {
		return affiliate != nil && affiliate.IsEnabled(ctx)
	})
	return rechargecampaigns.NewHandler(catalog)
}

func ProvideMarketingHandler(coupons *service.CouponService, lottery *service.LotteryService) *marketing.Handler[service.RedeemCode] {
	var couponPort marketing.CouponHTTPService
	if coupons != nil {
		couponPort = coupons
	}
	var lotteryPort marketing.LotteryHTTPService[service.RedeemCode]
	if lottery != nil {
		lotteryPort = lottery
	}
	return marketing.NewHandler[service.RedeemCode](couponPort, lotteryPort, func(c *gin.Context) (int64, bool) {
		subject, ok := middleware.GetAuthSubjectFromContext(c)
		return subject.UserID, ok
	})
}
