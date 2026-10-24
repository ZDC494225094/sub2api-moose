package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

// These helpers inherit their caller's authentication, rate-limit and audit
// middleware. Runtime switches are enforced by customize.Manager before entry.
func registerCustomUserRoutes(authenticated *gin.RouterGroup, h *handler.Handlers, extensions *customize.Manager) {
	if h != nil && h.Extensions != nil && h.Extensions.Wechat != nil {
		h.Extensions.Wechat.RegisterUserRoutes(authenticated)
	}
	// Core-only hosts and upstream route tests do not have to construct plugins.
	if h == nil || h.Extensions == nil || h.Extensions.Playground == nil {
		return
	}
	playground := authenticated.Group("/playground")
	playground.POST("/runs", extensions.Require("playground"), h.Extensions.Playground.StartRun)
	playground.GET("/runs/:id", h.Extensions.Playground.GetRun)
	playground.GET("/runs/:id/images/:index", h.Extensions.Playground.GetRunImage)
	playground.GET("/runs/:id/videos/:index", h.Extensions.Playground.GetRunVideo)
	playground.GET("/runs/:id/audio/:index", h.Extensions.Playground.GetRunAudio)
	playground.DELETE("/runs/:id", h.Extensions.Playground.CancelRun)
	canvas := authenticated.Group("/canvas")
	canvas.GET("/config", extensions.Require("infinite-canvas"), h.Extensions.Playground.GetCanvasConfig)
	canvas.POST("/runs", extensions.Require("infinite-canvas"), h.Extensions.Playground.StartCanvasRun)
	canvas.GET("/runs/:id", h.Extensions.Playground.GetRun)
	canvas.GET("/runs/:id/images/:index", h.Extensions.Playground.GetRunImage)
	canvas.GET("/runs/:id/videos/:index", h.Extensions.Playground.GetRunVideo)
	canvas.GET("/runs/:id/audio/:index", h.Extensions.Playground.GetRunAudio)
	canvas.DELETE("/runs/:id", h.Extensions.Playground.CancelRun)
}

func registerCustomOperationsRoutes(admin *gin.RouterGroup, h *handler.Handlers, extensions *customize.Manager) {
	if h != nil && h.Extensions != nil && h.Extensions.BotNotify != nil {
		h.Extensions.BotNotify.Register(admin, extensions)
	}
	if h != nil && h.Extensions != nil && h.Extensions.UserGuide != nil {
		h.Extensions.UserGuide.RegisterAdmin(admin, extensions)
	}
	if h == nil || h.Extensions == nil || h.Extensions.Operations == nil {
		return
	}
	operations := admin.Group("/operations", extensions.Require("operations-analytics"))
	operations.GET("/balance/history", h.Extensions.Operations.GetOperationsBalanceHistory)
}

func registerCustomDashboardRoutes(dashboard *gin.RouterGroup, h *handler.Handlers, extensions *customize.Manager) {
	if h == nil || h.Extensions == nil || h.Extensions.Operations == nil {
		return
	}
	// A child group keeps the gate off upstream dashboard endpoints.
	dashboard = dashboard.Group("", extensions.Require("operations-analytics"))
	dashboard.GET("/operations-funnel", h.Extensions.Operations.GetOperationsFunnel)
	dashboard.GET("/operations-finance", h.Extensions.Operations.GetOperationsFinance)
	dashboard.GET("/operations-customers", h.Extensions.Operations.GetOperationsCustomers)
	dashboard.GET("/operations-users", h.Extensions.Operations.GetOperationsUserDetails)
	dashboard.GET("/operations-marketing-recipients", h.Extensions.Operations.ListOperationsMarketingRecipients)
	dashboard.GET("/operations-marketing-email-records", h.Extensions.Operations.ListOperationsMarketingEmailRecords)
	dashboard.POST("/operations-marketing-email", h.Extensions.Operations.SendOperationsMarketingEmail)
}

func registerCustomPaymentUserRoutes(group *gin.RouterGroup, h *handler.ExtensionHandlers, extensions *customize.Manager) {
	if h != nil && h.Marketing != nil {
		h.Marketing.RegisterUserRoutes(group)
	}
	if h == nil || h.RechargeCampaigns == nil {
		return
	}
	group.GET("/campaigns", extensions.Require(customize.RechargeCampaigns), h.RechargeCampaigns.ListPublic)
}
func registerCustomPaymentPublicRoutes(group *gin.RouterGroup, h *handler.ExtensionHandlers, extensions *customize.Manager, rateLimit gin.HandlerFunc) {
	if h != nil && h.Wechat != nil {
		h.Wechat.RegisterPublicRoutes(group)
	}
	if h == nil || h.RechargeCampaigns == nil {
		return
	}
	group.GET("/campaigns", rateLimit, extensions.Require(customize.RechargeCampaigns), h.RechargeCampaigns.ListPublic)
}
func registerCustomPaymentAdminRoutes(group *gin.RouterGroup, h *handler.ExtensionHandlers, extensions *customize.Manager) {
	if h != nil && h.Marketing != nil {
		h.Marketing.RegisterAdminRoutes(group)
	}
	if h == nil || h.RechargeCampaigns == nil {
		return
	}
	group = group.Group("", extensions.Require(customize.RechargeCampaigns))
	group.GET("/campaigns", h.RechargeCampaigns.ListAdmin)
	group.POST("/campaigns", h.RechargeCampaigns.Save)
	group.PUT("/campaigns/:id", h.RechargeCampaigns.Save)
}

// Public announcements are a premium-home addition; authenticated upstream
// announcement routes remain independent of this switch.
func registerCustomPublicAnnouncementRoute(v1 *gin.RouterGroup, extensions *customize.Manager, list gin.HandlerFunc) {
	v1.GET("/announcements/public", extensions.Require("premium-home"), list)
}

// The public user guide is a user-guide extension addition, mounted next to
// the public announcements route and gated by its own switch.
func registerCustomPublicGuideRoute(v1 *gin.RouterGroup, h *handler.Handlers, extensions *customize.Manager) {
	if h == nil || h.Extensions == nil || h.Extensions.UserGuide == nil {
		return
	}
	v1.GET("/guide", extensions.Require("user-guide"), h.Extensions.UserGuide.Public)
}
