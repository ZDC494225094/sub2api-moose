package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/subscriptionextensions"
)

// SubscriptionIssuanceAdmission is supplied at the composition root.
// The compatibility constructor has native behavior, never implicitly enables customization.
type SubscriptionIssuanceAdmission interface {
	NewSubscriptionPolicy(context.Context) (string, error)
}
type subscriptionOwnerRepository interface {
	LockSubscriptionOwner(context.Context, int64) error
	FindNativeForAssignment(context.Context, int64, int64) (*UserSubscription, error)
}

func (s *SubscriptionService) NewSubscriptionPolicy(ctx context.Context) (string, error) {
	if s == nil || s.issuanceAdmission == nil {
		return subscriptionextensions.Native, nil
	}
	policy, err := s.issuanceAdmission.NewSubscriptionPolicy(ctx)
	if err != nil {
		return "", err
	}
	if policy == "" {
		return "", fmt.Errorf("empty subscription admission policy")
	}
	return subscriptionextensions.Resolve(policy)
}
func (s *SettingService) NewSubscriptionPolicy(ctx context.Context) (string, error) {
	if s == nil {
		return subscriptionextensions.Native, nil
	}
	return subscriptionextensions.NewPolicy(ctx, s.CustomExtensions())
}

// A policy passed by an internal fulfillment path is already frozen. Public DTOs
// never bind this field; an empty live input chooses the current creation policy.
func (s *SubscriptionService) assignWithPolicy(ctx context.Context, input *AssignSubscriptionInput, assignment, deferCache bool) (*UserSubscription, bool, error) {
	if input == nil {
		return nil, false, ErrSubscriptionNilInput
	}
	policy, err := subscriptionextensions.Resolve(input.issuancePolicy)
	if input.issuancePolicy == "" {
		policy, err = s.NewSubscriptionPolicy(ctx)
	}
	if err != nil {
		return nil, false, err
	}
	group, err := s.groupRepo.GetByID(ctx, input.GroupID)
	if err != nil {
		return nil, false, fmt.Errorf("group not found: %w", err)
	}
	if !group.IsSubscriptionType() {
		return nil, false, ErrGroupNotSubscriptionType
	}
	grant := *input
	grant.issuancePolicy = policy
	var sub *UserSubscription
	reused := false
	changed := false
	err = s.withSubscriptionUpdateTx(ctx, func(txCtx context.Context) error {
		if policy == subscriptionextensions.Native {
			repo, ok := s.userSubRepo.(subscriptionOwnerRepository)
			if !ok {
				return fmt.Errorf("native subscription ownership repository unavailable")
			}
			// Lock an always-existing owner row BEFORE lookup, including first issuance.
			// Row locks live in the same transaction as renewal/create and payment/redeem.
			if err := repo.LockSubscriptionOwner(txCtx, input.UserID); err != nil {
				return err
			}
			existing, err := repo.FindNativeForAssignment(txCtx, input.UserID, input.GroupID)
			if err != nil && !errors.Is(err, ErrSubscriptionNotFound) {
				return err
			}
			if err == nil && existing == nil {
				return fmt.Errorf("native lookup returned nil subscription")
			}
			if existing != nil {
				if existing.CustomSubscriptionPolicy != subscriptionextensions.Native || existing.UserID != input.UserID || existing.GroupID != input.GroupID {
					return fmt.Errorf("native subscription ownership mismatch")
				}
				reused = true
				now := time.Now()
				if s.now != nil {
					now = s.now()
				}
				expired := existing.Status == SubscriptionStatusExpired || (existing.Status != SubscriptionStatusSuspended && !existing.ExpiresAt.After(now))
				if assignment && !expired {
					if reason, conflict := detectAssignSemanticConflict(existing, input); conflict {
						return ErrSubscriptionAssignConflict.WithMetadata(map[string]string{"conflict_reason": reason})
					}
					sub = existing
					return nil
				}
				if err := s.updateExistingSubscriptionTerm(txCtx, existing.ID, normalizeAssignValidityDays(input.ValidityDays), input.Notes, assignment); err != nil {
					return err
				}
				sub, err = s.userSubRepo.GetByID(txCtx, existing.ID)
				changed = err == nil
				return err
			}
		}
		var err error
		sub, err = s.createSubscription(txCtx, &grant)
		changed = err == nil
		return err
	})
	if err != nil {
		return nil, false, err
	}
	if changed && !deferCache {
		s.maybeInvalidateAssignmentCaches(input.UserID, input.GroupID, false)
		if err := s.invalidateSubscriptionByID(sub.ID); err != nil {
			log.Printf("Warning: subscription %d committed but cache invalidation failed: %v", sub.ID, err)
		}
	}
	return sub, reused, nil
}
