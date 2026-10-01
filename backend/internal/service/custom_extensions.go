package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/customize"
)

// CustomExtensions is the only settings adapter required by the extension host.
// Keeping it in its own file avoids growing upstream's settings DTO / parser.
func (s *SettingService) CustomExtensions() *customize.Manager {
	if s == nil {
		return customize.NewManager(nil)
	}
	return customize.NewManager(s.settingRepo)
}

func (s *PaymentService) rechargeCampaignExtensionEnabled(ctx context.Context) (bool, error) {
	if s.configService == nil || s.configService.settingRepo == nil {
		return customize.NewManager(nil).Enabled(ctx, customize.RechargeCampaigns)
	}
	return customize.NewManager(s.configService.settingRepo).Enabled(ctx, customize.RechargeCampaigns)
}
