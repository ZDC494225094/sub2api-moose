package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/rechargecampaigns"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/userguide"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/wechat"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ExtensionHandlers is the host's single extension container. Core handler
// structs do not grow one field for every custom module.
type ExtensionHandlers struct {
	BotNotify         *botnotify.Handler
	UserGuide         *userguide.Handler
	Marketing         *marketing.Handler[service.RedeemCode]
	Wechat            *wechat.Handler
	Operations        *admin.OperationsHandler
	RechargeCampaigns *rechargecampaigns.Handler
	Playground        *PlaygroundHandler
}

func NewExtensionHandlers(operations *admin.OperationsHandler, playground *PlaygroundHandler, campaigns *rechargecampaigns.Handler, marketingHandler *marketing.Handler[service.RedeemCode], wechatHandler *wechat.Handler, bot *botnotify.Handler, guide *userguide.Handler) *ExtensionHandlers {
	return &ExtensionHandlers{Operations: operations, Playground: playground, RechargeCampaigns: campaigns, Marketing: marketingHandler, Wechat: wechatHandler, BotNotify: bot, UserGuide: guide}
}
