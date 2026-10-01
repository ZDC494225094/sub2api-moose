package marketing

import "github.com/gin-gonic/gin"

// RegisterUserRoutes and RegisterAdminRoutes must be mounted inside the host's
// auth/rate-limit/audit groups, never on a public payment/webhook group.
// The module remains unmanaged until admission and historical drains are ready.
func (h *Handler[T]) RegisterUserRoutes(group *gin.RouterGroup) {
	group.GET("/coupons", h.GetMyCoupons)
	group.GET("/lottery/active", h.GetActiveLottery)
	group.POST("/lottery/draw", h.DrawLottery)
	group.GET("/lottery/my-records", h.ListMyDrawRecords)
}
func (h *Handler[T]) RegisterAdminRoutes(group *gin.RouterGroup) {
	couponTemplates := group.Group("/coupon-templates")
	{
		couponTemplates.GET("", h.ListCouponTemplates)
		couponTemplates.POST("", h.CreateCouponTemplate)
		couponTemplates.PUT("/:id", h.UpdateCouponTemplate)
	}

	lottery := group.Group("/lottery")
	{
		lottery.GET("/activities", h.ListLotteryActivities)
		lottery.POST("/activities", h.CreateLotteryActivity)
		lottery.PUT("/activities/:id", h.UpdateLotteryActivity)
		lottery.DELETE("/activities/:id", h.DeleteLotteryActivity)
		lottery.GET("/activities/:id/draw-records", h.ListActivityDrawRecords)
		lottery.POST("/prizes", h.CreateLotteryPrize)
		lottery.PUT("/prizes/:id", h.UpdateLotteryPrize)
	}

}
