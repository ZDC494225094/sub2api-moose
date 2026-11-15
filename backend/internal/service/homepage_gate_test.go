package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
	"github.com/stretchr/testify/require"
)

type homepageSettingsRepo struct {
	registrationProofSettingRepo
	reads int
	err   error
}

func (r *homepageSettingsRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	r.reads++
	if r.err != nil {
		return nil, r.err
	}
	return r.registrationProofSettingRepo.GetMultiple(ctx, keys)
}

func TestHomepageSettingCachedAndFailuresDoNotDisableGate(t *testing.T) {
	repo := &homepageSettingsRepo{registrationProofSettingRepo: registrationProofSettingRepo{values: map[string]string{SettingKeyHomepageCaptchaEnabled: "true"}}}
	s := NewSettingService(repo, &config.Config{})
	for range 10 {
		enabled, err := s.HomepageCaptchaEnabled(context.Background())
		require.NoError(t, err)
		require.True(t, enabled)
	}
	require.Equal(t, 1, repo.reads)
	s.homepageCaptchaCache.expires = time.Time{}
	repo.err = errors.New("db offline")
	_, err := s.HomepageCaptchaEnabled(context.Background())
	require.ErrorIs(t, err, accesspolicy.ErrSettingsUnavailable)
	repo.err = nil
	repo.values[SettingKeyHomepageCaptchaEnabled] = "corrupt"
	_, err = s.HomepageCaptchaEnabled(context.Background())
	require.ErrorIs(t, err, accesspolicy.ErrSettingsUnavailable)
	repo.values[SettingKeyHomepageCaptchaEnabled] = ""
	_, err = s.HomepageCaptchaEnabled(context.Background())
	require.ErrorIs(t, err, accesspolicy.ErrSettingsUnavailable)
	delete(repo.values, SettingKeyHomepageCaptchaEnabled)
	enabled, err := s.HomepageCaptchaEnabled(context.Background())
	require.NoError(t, err)
	require.False(t, enabled)
}

func TestHomepageSettingSaveAndOmissionPreserveSwitch(t *testing.T) {
	repo := &homepageSettingsRepo{registrationProofSettingRepo: registrationProofSettingRepo{values: map[string]string{customize.Key("access-policy"): "true"}}}
	s := NewSettingService(repo, &config.Config{})
	ctx := context.Background()
	settings, err := s.GetAllSettings(ctx)
	require.NoError(t, err)
	settings.HomepageCaptchaEnabled = true
	require.NoError(t, s.UpdateSettings(ctx, settings))
	require.Equal(t, "true", repo.values[SettingKeyHomepageCaptchaEnabled])
	enabled, err := s.HomepageCaptchaEnabled(ctx)
	require.NoError(t, err)
	require.True(t, enabled)
	settings.HomepageCaptchaEnabled = false
	require.NoError(t, s.UpdateSettingsOmitting(ctx, settings, OmittedSettingKeys{SettingKeyHomepageCaptchaEnabled: {}}))
	enabled, err = s.HomepageCaptchaEnabled(ctx)
	require.NoError(t, err)
	require.True(t, enabled)
	require.Equal(t, "true", repo.values[SettingKeyHomepageCaptchaEnabled])
	require.NoError(t, s.UpdateSettings(ctx, settings))
	enabled, err = s.HomepageCaptchaEnabled(ctx)
	require.NoError(t, err)
	require.False(t, enabled)
}

func TestHomepageGateFlagIsIncludedInPublicAndInjectedSettings(t *testing.T) {
	repo := &registrationProofSettingRepo{values: map[string]string{SettingKeyHomepageCaptchaEnabled: "true"}}
	s := NewSettingService(repo, &config.Config{})
	settings, err := s.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.HomepageCaptchaEnabled)
	injected, err := s.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	data, err := json.Marshal(injected)
	require.NoError(t, err)
	require.Contains(t, string(data), `"homepage_captcha_enabled":true`)
}
