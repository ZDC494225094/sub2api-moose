package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// APIKeyRoutingAdmission reports whether new multi-group key configuration is
// admitted. It is supplied by the extension composition root; without it the
// service behaves like upstream and never creates multi-group keys.
type APIKeyRoutingAdmission interface {
	MultiGroupEnabled(context.Context) (bool, error)
}

var ErrAPIKeyNativeRouting = infraerrors.BadRequest("API_KEY_NATIVE_ROUTING", "this API key uses upstream single-group routing; create a new key for multi-group or subscription-first billing")

func (s *APIKeyService) SetRoutingAdmission(admission APIKeyRoutingAdmission) {
	s.routingAdmission = admission
}

func (s *APIKeyService) multiGroupEnabled() multigroupbilling.EnabledFunc {
	if s == nil || s.routingAdmission == nil {
		return nil
	}
	return s.routingAdmission.MultiGroupEnabled
}

// APIKeyRoutingPolicyForDisplay resolves legacy empty values; unknown values are
// shown as stored rather than silently relabelled.
func APIKeyRoutingPolicyForDisplay(policy string) string {
	if resolved, err := multigroupbilling.RoutingPolicy(policy); err == nil {
		return resolved
	}
	return policy
}

func mapRoutingAdmissionError(err error) error {
	switch {
	case errors.Is(err, multigroupbilling.ErrExtensionDisabled):
		return customize.Disabled(multigroupbilling.ModuleID)
	case errors.Is(err, multigroupbilling.ErrNativeKeyExtension):
		return ErrAPIKeyNativeRouting
	}
	return err
}

// prepareAPIKeyBinding is the host boundary for creation. Policy never owns
// user authorization, protocol definitions or repository access.
// The returned routing policy is durable ownership, decided once at creation.
func (s *APIKeyService) prepareAPIKeyBinding(ctx context.Context, user *User, req CreateAPIKeyRequest) (multigroupbilling.Binding, string, error) {
	binding := multigroupbilling.PlanCreate(req.GroupID, req.GroupIDs, req.Platform)
	policy, err := multigroupbilling.DecideCreate(ctx, s.multiGroupEnabled(), binding.GroupIDs, req.BillingPriority)
	if err != nil {
		return multigroupbilling.Binding{}, "", mapRoutingAdmissionError(err)
	}
	platform, err := s.validateBindableGroupIDs(ctx, user, binding.Platform, binding.GroupIDs)
	if err != nil {
		return multigroupbilling.Binding{}, "", err
	}
	binding.Platform = platform
	return binding, policy, nil
}

func (s *APIKeyService) applyAPIKeyBindingUpdate(ctx context.Context, userID int64, key *APIKey, req UpdateAPIKeyRequest, fields *APIKeyUpdateFields) error {
	plan := multigroupbilling.PlanUpdate(
		multigroupbilling.Binding{Primary: key.GroupID, GroupIDs: key.GroupIDs, Platform: key.Platform},
		multigroupbilling.BindingPatch{Primary: req.GroupID, GroupIDs: req.GroupIDs, GroupIDsSet: req.GroupIDsSet, Platform: req.Platform},
	)
	if plan.Changed || req.BillingPriority != nil {
		if err := multigroupbilling.CheckUpdate(ctx, s.multiGroupEnabled(), multigroupbilling.UpdateRequest{
			Policy:          key.RoutingPolicy,
			Current:         multigroupbilling.Binding{Primary: key.GroupID, GroupIDs: key.GroupIDs, Platform: key.Platform},
			CurrentPriority: key.BillingPriority,
			Plan:            plan,
			NextPriority:    req.BillingPriority,
		}); err != nil {
			return mapRoutingAdmissionError(err)
		}
	}
	if !plan.Changed {
		return nil
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	platform, err := s.validateBindableGroupIDs(ctx, user, plan.Platform, plan.GroupIDs)
	if err != nil {
		return err
	}
	key.Platform, key.GroupIDs = platform, plan.GroupIDs
	fields.Platform = true
	if plan.WriteGroups {
		key.GroupID = plan.Primary
		fields.GroupID, fields.GroupIDs = true, true
	}
	return nil
}

func (s *APIKeyService) validateBindableGroupIDs(ctx context.Context, user *User, requestedPlatform string, groupIDs []int64) (string, error) {
	platform := NormalizeAPIKeyPlatform(requestedPlatform)
	if strings.TrimSpace(requestedPlatform) != "" && platform == "" {
		return "", ErrInvalidAPIKeyPlatform
	}
	for _, groupID := range NormalizeAPIKeyGroupIDs(nil, groupIDs) {
		group, err := s.groupRepo.GetByID(ctx, groupID)
		if err != nil {
			return "", fmt.Errorf("get group: %w", err)
		}
		if !s.canUserBindGroup(ctx, user, group) {
			return "", ErrGroupNotAllowed
		}
		groupPlatform := DefaultAPIKeyPlatform(group.Platform)
		if platform == "" {
			platform = groupPlatform
		}
		if groupPlatform != platform {
			return "", ErrAPIKeyGroupPlatformMismatch
		}
	}
	return DefaultAPIKeyPlatform(platform), nil
}
