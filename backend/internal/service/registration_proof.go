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

func (s *AuthService) CreateRegistrationProofChallenge(ctx context.Context, email, remoteIP string) (*RegistrationProofChallenge, error) {
	if s == nil {
		return nil, accesspolicy.ErrSettingsUnavailable
	}
	policy, err := s.settingService.registrationAccessSettings(ctx)
	if err != nil {
		return nil, err
	}
	if !policy.ProofEnabled {
		return &RegistrationProofChallenge{Enabled: false}, nil
	}
	return accesspolicy.CreateRegistrationProofChallenge(s.registrationProofSecret(), policy.ProofDifficulty, email, remoteIP)
}
func (s *AuthService) VerifyRegistrationProof(ctx context.Context, email, remoteIP, challenge, solution string) error {
	if s == nil {
		return accesspolicy.ErrSettingsUnavailable
	}
	policy, err := s.settingService.registrationAccessSettings(ctx)
	if err != nil {
		return err
	}
	if !policy.ProofEnabled {
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
