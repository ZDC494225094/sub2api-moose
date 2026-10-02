package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
)

// Compatibility seam: settings and authentication stay in the host; the v1
// challenge protocol belongs to accesspolicy. Existing tokens remain valid.
const (
	RegistrationProofMinDifficulty     = accesspolicy.RegistrationProofMinDifficulty
	RegistrationProofMaxDifficulty     = accesspolicy.RegistrationProofMaxDifficulty
	DefaultRegistrationProofDifficulty = accesspolicy.DefaultRegistrationProofDifficulty
)

var (
	ErrRegistrationProofFailed        = accesspolicy.ErrRegistrationProofFailed
	ErrRegistrationProofNotConfigured = accesspolicy.ErrRegistrationProofNotConfigured
)

type RegistrationProofChallenge = accesspolicy.RegistrationProofChallenge

func normalizeRegistrationProofDifficulty(value string) int {
	return accesspolicy.NormalizeRegistrationProofDifficulty(value)
}
func clampRegistrationProofDifficulty(value int) int {
	return accesspolicy.ClampRegistrationProofDifficulty(value)
}

func (s *SettingService) IsRegistrationProofEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationProofEnabled)
	return err == nil && value == "true"
}

func (s *SettingService) GetRegistrationProofDifficulty(ctx context.Context) int {
	if s == nil || s.settingRepo == nil {
		return DefaultRegistrationProofDifficulty
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationProofDifficulty)
	if err != nil {
		return DefaultRegistrationProofDifficulty
	}
	return normalizeRegistrationProofDifficulty(value)
}

func (s *AuthService) CreateRegistrationProofChallenge(ctx context.Context, email, remoteIP string) (*RegistrationProofChallenge, error) {
	if s == nil || s.settingService == nil || !s.settingService.IsRegistrationProofEnabled(ctx) {
		return &RegistrationProofChallenge{Enabled: false}, nil
	}
	return accesspolicy.CreateRegistrationProofChallenge(s.registrationProofSecret(), s.settingService.GetRegistrationProofDifficulty(ctx), email, remoteIP)
}
func (s *AuthService) VerifyRegistrationProof(ctx context.Context, email, remoteIP, challenge, solution string) error {
	if s == nil || s.settingService == nil || !s.settingService.IsRegistrationProofEnabled(ctx) {
		return nil
	}
	return accesspolicy.VerifyRegistrationProof(s.registrationProofSecret(), email, remoteIP, challenge, solution)
}
func (s *AuthService) registrationProofSecret() []byte {
	if s == nil || s.cfg == nil {
		return nil
	}
	return accesspolicy.RegistrationProofSecret(s.cfg.JWT.Secret)
}
