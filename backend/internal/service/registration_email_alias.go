package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
)

type EmailAliasProbe = accesspolicy.EmailAliasProbe

func NormalizeEmailForAliasDedup(email string) string {
	return accesspolicy.NormalizeEmailForAliasDedup(email)
}
func EmailAliasDedupProbes(email string) []EmailAliasProbe {
	return accesspolicy.EmailAliasDedupProbes(email)
}

// existsByEmailOrAlias reports whether an email — or any alias variant that
// resolves to the same inbox — is already registered.
//
// It first performs the exact ExistsByEmail check, then, only on a miss, probes
// for an alias collision. Consistent with ExistsByEmail, lookup errors are
// surfaced (fail-closed) so the registration path returns a service error instead
// of letting an attacker bypass the check by inducing errors.
func (s *AuthService) existsByEmailOrAlias(ctx context.Context, email string) (bool, error) {
	exists, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil || exists {
		return exists, err
	}
	return s.userRepo.ExistsByEmailAlias(ctx, email)
}
