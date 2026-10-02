package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
)

func (s *SettingService) registrationAccessSettings(ctx context.Context) (accesspolicy.Settings, error) {
	if s == nil || s.settingRepo == nil {
		return accesspolicy.Settings{}, accesspolicy.ErrSettingsUnavailable
	}
	return accesspolicy.ReadSettings(ctx, s.settingRepo)
}

func (s *SettingService) checkAccessPolicyConfiguration(ctx context.Context, updates map[string]string) error {
	return accesspolicy.CheckConfigurationChange(ctx, s.settingRepo, s.CustomExtensions(), updates)
}
