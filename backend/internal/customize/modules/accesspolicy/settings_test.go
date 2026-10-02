package accesspolicy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type policySettingsStub struct {
	values map[string]string
	err    error
	calls  int
}

func (s *policySettingsStub) GetMultiple(_ context.Context, _ []string) (map[string]string, error) {
	s.calls++
	return s.values, s.err
}

type policyStateStub struct {
	enabled bool
	err     error
	calls   int
}

func (s *policyStateStub) Enabled(_ context.Context, id string) (bool, error) {
	s.calls++
	if id != ModuleID {
		panic(id)
	}
	return s.enabled, s.err
}

func TestSecuritySettingsReadFailsClosed(t *testing.T) {
	ctx := context.Background()
	_, err := ReadSettings(ctx, nil)
	require.ErrorIs(t, err, ErrSettingsUnavailable)
	_, err = ReadSettings(ctx, &policySettingsStub{err: errors.New("offline")})
	require.ErrorIs(t, err, ErrSettingsUnavailable)
	defaults, err := ReadSettings(ctx, &policySettingsStub{})
	require.NoError(t, err)
	require.Equal(t, DefaultRegistrationProofDifficulty, defaults.ProofDifficulty)
	require.False(t, defaults.ProofEnabled)
	require.Empty(t, defaults.EmailWhitelist)
	for key, values := range map[string][]string{
		DomainQuotaKey: {"", "1", "TRUE"}, ProofEnabledKey: {"", "yes"}, MainlandRestrictionKey: {"garbage"},
		ProofDifficultyKey: {"garbage", "15", "25"}, EmailWhitelistKey: {"broken-json", "{}", `["invalid@@domain"]`, `[1]`},
	} {
		for _, value := range values {
			t.Run(key+"/"+value, func(t *testing.T) {
				_, err := ParseSettings(map[string]string{key: value})
				require.ErrorIs(t, err, ErrSettingsUnavailable)
			})
		}
	}
	settings, err := ParseSettings(map[string]string{DomainQuotaKey: "true", ProofEnabledKey: "true", ProofDifficultyKey: "16", MainlandRestrictionKey: "true", EmailWhitelistKey: `["@Example.com","example.com"]`})
	require.NoError(t, err)
	require.True(t, settings.DomainQuota)
	require.True(t, settings.ProofEnabled)
	require.True(t, settings.MainlandRestriction)
	require.Equal(t, 16, settings.ProofDifficulty)
	require.Equal(t, []string{"@example.com"}, settings.EmailWhitelist)
}

func TestConfigurationAdmissionPreservesEnforcementAndNativeEdits(t *testing.T) {
	ctx := context.Background()
	current := &policySettingsStub{values: map[string]string{ProofEnabledKey: "true"}}
	states := &policyStateStub{}
	require.NoError(t, CheckConfigurationChange(ctx, nil, nil, map[string]string{EmailWhitelistKey: `["@example.com"]`, "site_name": "native"}))
	require.NoError(t, CheckConfigurationChange(ctx, current, states, map[string]string{ProofEnabledKey: "true", DomainQuotaKey: "false", ProofDifficultyKey: "18"}))
	require.Zero(t, states.calls)
	for _, key := range ConfigurationKeys() {
		next := "true"
		if key == ProofEnabledKey {
			next = "false"
		}
		if key == ProofDifficultyKey {
			next = "20"
		}
		require.ErrorIs(t, CheckConfigurationChange(ctx, current, states, map[string]string{key: next}), ErrExtensionDisabled)
	}
	require.Equal(t, "true", current.values[ProofEnabledKey])
	states.enabled = true
	require.NoError(t, CheckConfigurationChange(ctx, current, states, map[string]string{ProofEnabledKey: "false"}))
	states.err = errors.New("state offline")
	require.ErrorIs(t, CheckConfigurationChange(ctx, current, states, map[string]string{ProofEnabledKey: "false"}), states.err)
	require.ErrorIs(t, CheckConfigurationChange(ctx, current, nil, map[string]string{ProofEnabledKey: "false"}), ErrExtensionDisabled)
	require.ErrorIs(t, CheckConfigurationChange(ctx, nil, states, map[string]string{ProofEnabledKey: "false"}), ErrSettingsUnavailable)
	current.err = errors.New("settings offline")
	require.ErrorIs(t, CheckConfigurationChange(ctx, current, states, map[string]string{ProofEnabledKey: "true"}), ErrSettingsUnavailable)
}
