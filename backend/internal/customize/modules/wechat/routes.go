package wechat

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes mounts the OAuth endpoints that WeChat redirects to.
// Mount on a public (rate-limited) group, never behind login.
func (h *Handler) RegisterPublicRoutes(group *gin.RouterGroup) {
	group.GET("/wechat/oauth/url", h.GetOAuthURL)
	group.GET("/wechat/oauth/callback", h.OAuthCallback)
}

// RegisterUserRoutes mounts the authenticated endpoints.
func (h *Handler) RegisterUserRoutes(group *gin.RouterGroup) {
	group.GET("/wechat/binding", h.GetBinding)
	group.POST("/wechat/bind", h.BindOpenID)
	group.POST("/wechat/lottery-notify", h.NotifyDrawResult)
}
