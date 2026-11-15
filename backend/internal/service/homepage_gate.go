package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
)

const SettingKeyHomepageCaptchaEnabled = accesspolicy.HomepageCaptchaEnabledKey

type homepageCaptchaCache struct {
	enabled         bool
	expires         time.Time
	turnstile       accesspolicy.HomepageTurnstileConfig
	providerExpires time.Time
}

func (s *SettingService) HomepageTurnstileConfig(ctx context.Context) (accesspolicy.HomepageTurnstileConfig, error) {
	if s == nil || s.settingRepo == nil {
		return accesspolicy.HomepageTurnstileConfig{}, accesspolicy.ErrSettingsUnavailable
	}
	s.homepageCaptchaMu.Lock()
	defer s.homepageCaptchaMu.Unlock()
	if time.Now().Before(s.homepageCaptchaCache.providerExpires) {
		return s.homepageCaptchaCache.turnstile, nil
	}
	config, err := s.GetCaptchaProviderConfig(ctx)
	if err != nil {
		return accesspolicy.HomepageTurnstileConfig{}, err
	}
	if !config.TurnstileEnabled {
		s.homepageCaptchaCache.turnstile = accesspolicy.HomepageTurnstileConfig{}
		s.homepageCaptchaCache.providerExpires = time.Now().Add(30 * time.Second)
		return accesspolicy.HomepageTurnstileConfig{}, nil
	}
	if captchaProvidersConflict(config.TurnstileEnabled, config.Tencent.Enabled, config.Aliyun.Enabled) {
		return accesspolicy.HomepageTurnstileConfig{}, ErrCaptchaProviderConflict
	}
	if strings.TrimSpace(config.TurnstileSiteKey) == "" || strings.TrimSpace(config.TurnstileSecretKey) == "" {
		return accesspolicy.HomepageTurnstileConfig{}, ErrTurnstileNotConfigured
	}
	s.homepageCaptchaCache.turnstile = accesspolicy.HomepageTurnstileConfig{SiteKey: config.TurnstileSiteKey, SecretKey: config.TurnstileSecretKey}
	s.homepageCaptchaCache.providerExpires = time.Now().Add(30 * time.Second)
	return s.homepageCaptchaCache.turnstile, nil
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

// Homepage tokens must come from this browser's challenge widget, not an
// unrelated login widget using the same site key.
func (s *TurnstileService) VerifyHomepageToken(ctx context.Context, config accesspolicy.HomepageTurnstileConfig, token, challengeID, remoteIP string) error {
	if s == nil || s.verifier == nil || config.SecretKey == "" {
		return ErrTurnstileNotConfigured
	}
	if token == "" || len(token) > 2048 || challengeID == "" {
		return ErrTurnstileVerificationFailed
	}
	result, err := s.verifier.VerifyToken(ctx, config.SecretKey, token, remoteIP)
	if err != nil {
		return err
	}
	if result == nil || !result.Success || result.Action != "homepage" || result.CData != challengeID {
		return ErrTurnstileVerificationFailed
	}
	return nil
}
