package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
)

const SettingKeyHomepageCaptchaEnabled = accesspolicy.HomepageCaptchaEnabledKey

type homepageCaptchaCache struct {
	enabled bool
	expires time.Time
}

// Cache a single switch; the lightweight page must not fetch public settings or
// query the database on every visit. Saving this setting replaces the cache.
func (s *SettingService) HomepageCaptchaEnabled(ctx context.Context) (bool, error) {
	if s == nil || s.settingRepo == nil {
		return false, accesspolicy.ErrSettingsUnavailable
	}
	s.homepageCaptchaMu.Lock()
	defer s.homepageCaptchaMu.Unlock()
	if time.Now().Before(s.homepageCaptchaCache.expires) {
		return s.homepageCaptchaCache.enabled, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyHomepageCaptchaEnabled})
	if err != nil {
		return false, accesspolicy.ErrSettingsUnavailable
	}
	raw, exists := values[SettingKeyHomepageCaptchaEnabled]
	if exists && raw != "true" && raw != "false" {
		return false, accesspolicy.ErrSettingsUnavailable
	}
	s.homepageCaptchaCache = homepageCaptchaCache{enabled: raw == "true", expires: time.Now().Add(30 * time.Second)}
	return s.homepageCaptchaCache.enabled, nil
}
