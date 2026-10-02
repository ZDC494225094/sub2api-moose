package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
	"github.com/stretchr/testify/require"
)

type accessPolicyRepo struct {
	registrationProofSettingRepo
	readErr error
	writes  int
}

func (r *accessPolicyRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}
	return r.registrationProofSettingRepo.GetMultiple(ctx, keys)
}
func (r *accessPolicyRepo) SetMultiple(ctx context.Context, updates map[string]string) error {
	r.writes++
	return r.registrationProofSettingRepo.SetMultiple(ctx, updates)
}

func TestAccessPolicyConfigurationAllSettingsSavePaths(t *testing.T) {
	for _, withDefaults := range []bool{false, true} {
		for _, state := range []string{"true", "false", "missing", "malformed"} {
			t.Run(state+map[bool]string{false: "/plain", true: "/auth-defaults"}[withDefaults], func(t *testing.T) {
				repo := &accessPolicyRepo{registrationProofSettingRepo: registrationProofSettingRepo{values: map[string]string{SettingKeyRegistrationProofEnabled: "true"}}}
				if state != "missing" {
					repo.values[customize.Key(accesspolicy.ModuleID)] = state
				}
				svc := NewSettingService(repo, nil)
				save := func(settings *SystemSettings, omitted OmittedSettingKeys) error {
					if withDefaults {
						return svc.UpdateSettingsWithAuthSourceDefaultsOmitting(context.Background(), settings, nil, omitted)
					}
					return svc.UpdateSettingsOmitting(context.Background(), settings, omitted)
				}
				err := save(&SystemSettings{RegistrationProofEnabled: false}, nil)
				if state == "true" {
					require.NoError(t, err)
					require.Equal(t, "false", repo.values[SettingKeyRegistrationProofEnabled])
				} else {
					require.Error(t, err)
					require.Zero(t, repo.writes)
					require.Equal(t, "true", repo.values[SettingKeyRegistrationProofEnabled])
				}
				// Omitted security fields cannot be zeroed by an unrelated settings save.
				repo.values[SettingKeyRegistrationProofEnabled] = "true"
				omitted := OmittedSettingKeys{}
				for _, key := range accesspolicy.ConfigurationKeys() {
					omitted[key] = struct{}{}
				}
				require.NoError(t, save(&SystemSettings{SiteName: "native edit"}, omitted))
				require.Equal(t, "true", repo.values[SettingKeyRegistrationProofEnabled])
				require.Equal(t, "native edit", repo.values[SettingKeySiteName])
			})
		}
	}
}

func TestAccessPolicyEnforcementFailsClosedOnSettingsFailure(t *testing.T) {
	for _, broken := range []string{"offline", "malformed-whitelist", "malformed-proof", "missing-service"} {
		t.Run(broken, func(t *testing.T) {
			svc := newRegistrationProofTestService(map[string]string{})
			repo := &accessPolicyRepo{registrationProofSettingRepo: registrationProofSettingRepo{values: map[string]string{}}}
			svc.settingService = NewSettingService(repo, nil)
			switch broken {
			case "offline":
				repo.readErr = errors.New("offline")
			case "malformed-whitelist":
				repo.values[SettingKeyRegistrationEmailSuffixWhitelist] = "broken"
			case "malformed-proof":
				repo.values[SettingKeyRegistrationProofEnabled] = "invalid"
			case "missing-service":
				svc.settingService = nil
			}
			ctx := context.Background()
			_, err := svc.CreateRegistrationProofChallenge(ctx, "user@example.com", "127.0.0.1")
			require.ErrorIs(t, err, accesspolicy.ErrSettingsUnavailable)
			require.ErrorIs(t, svc.VerifyRegistrationProof(ctx, "user@example.com", "127.0.0.1", "", ""), accesspolicy.ErrSettingsUnavailable)
			require.ErrorIs(t, svc.validateRegistrationEmailPolicy(ctx, "user@example.com"), accesspolicy.ErrSettingsUnavailable)
			require.ErrorIs(t, svc.validateRegistrationEmailQuota(ctx, "user@example.com"), accesspolicy.ErrSettingsUnavailable)
		})
	}
}

func TestAccessPolicySettingsKeysMatchHost(t *testing.T) {
	require.Equal(t, []string{SettingKeyRegistrationEmailSuffixWhitelist, SettingKeyRegistrationEmailDomainQuotaEnabled, SettingKeyRegistrationProofEnabled, SettingKeyRegistrationProofDifficulty, SettingKeyMainlandChinaAccessRestrictionEnabled}, accesspolicy.SettingsKeys())
}

// An embedded interface intentionally panics on unexpected repository calls:
// policy loading and revalidation must happen before any user write.
type accessPolicyUserRepo struct {
	UserRepository
	writes int
}

func (r *accessPolicyUserRepo) CreateWithEmailAliasGuard(context.Context, *User) error {
	r.writes++
	return nil
}

func TestAccessPolicyUserCreationRechecksRulesBeforeWrite(t *testing.T) {
	for _, state := range []string{"true", "false", "malformed"} {
		t.Run(state, func(t *testing.T) {
			repo := &accessPolicyRepo{registrationProofSettingRepo: registrationProofSettingRepo{values: map[string]string{
				customize.Key(accesspolicy.ModuleID):       state,
				SettingKeyRegistrationEmailSuffixWhitelist: `["@allowed.example"]`,
			}}}
			users := &accessPolicyUserRepo{}
			svc := &AuthService{settingService: NewSettingService(repo, nil), userRepo: users}
			ctx := context.Background()
			require.Error(t, svc.createUserWithRegistrationEmailGuard(ctx, &User{Email: "user@other.example"}))
			require.Zero(t, users.writes)
			require.NoError(t, svc.createUserWithRegistrationEmailGuard(ctx, &User{Email: "user@allowed.example"}))
			require.Equal(t, 1, users.writes)
			repo.readErr = errors.New("settings offline")
			require.ErrorIs(t, svc.createUserWithRegistrationEmailGuard(ctx, &User{Email: "user@allowed.example"}), accesspolicy.ErrSettingsUnavailable)
			require.Equal(t, 1, users.writes)
			repo.readErr = nil
			repo.values[SettingKeyRegistrationEmailSuffixWhitelist] = "corrupt"
			require.ErrorIs(t, svc.createUserWithRegistrationEmailGuard(ctx, &User{Email: "user@allowed.example"}), accesspolicy.ErrSettingsUnavailable)
			require.Equal(t, 1, users.writes)
		})
	}
}

func TestAccessPolicyConfigurationReadFailureDoesNotWrite(t *testing.T) {
	repo := &accessPolicyRepo{readErr: errors.New("offline")}
	svc := NewSettingService(repo, nil)
	require.ErrorIs(t, svc.UpdateSettings(context.Background(), &SystemSettings{RegistrationProofEnabled: true}), accesspolicy.ErrSettingsUnavailable)
	require.Zero(t, repo.writes)
}
