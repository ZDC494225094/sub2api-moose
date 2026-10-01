package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/rechargecampaigns"
	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// ExtensionHandlers is the host's single extension container. Core handler
// structs do not grow one field for every custom module.
type ExtensionHandlers struct {
	Marketing         *marketing.Handler[service.RedeemCode]
	Operations        *admin.OperationsHandler
	RechargeCampaigns *rechargecampaigns.Handler
	Playground        *PlaygroundHandler
}

func NewExtensionHandlers(operations *admin.OperationsHandler, playground *PlaygroundHandler, campaigns *rechargecampaigns.Handler, marketingHandler *marketing.Handler[service.RedeemCode]) *ExtensionHandlers {
	return &ExtensionHandlers{Operations: operations, Playground: playground, RechargeCampaigns: campaigns, Marketing: marketingHandler}
}
