package service

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
)

type APIKeyGroupSelection struct {
	Group        *Group
	Subscription *UserSubscription
}

// SelectUsableGroupForAPIKey adapts host entities to the independent policy.
// It preserves historical keys while new-key admission is still being isolated;
// no runtime toggle is advertised for this pending module yet.
func (s *APIKeyService) SelectUsableGroupForAPIKey(ctx context.Context, apiKey *APIKey, subscriptionSvc *SubscriptionService) (*APIKeyGroupSelection, error) {
	if apiKey == nil {
		return nil, ErrAPIKeyNotFound
	}
	policy, err := multigroupbilling.RoutingPolicy(apiKey.RoutingPolicy)
	if err != nil {
		return nil, err
	}
	if policy == multigroupbilling.NativeRouting {
		// A nil selection means leave the host-loaded single Group/GroupID intact.
		// Both middleware paths continue native group/auth/subscription preflight.
		return nil, nil
	}
	key := multigroupbilling.Key{
		UserID: apiKey.UserID, Primary: apiKey.GroupID, GroupIDs: apiKey.GroupIDs,
		Platform: NormalizeAPIKeyPlatform(apiKey.Platform), Priority: apiKey.BillingPriority,
	}
	if apiKey.User != nil {
		balance := apiKey.User.Balance
		key.CachedBalance = &balance
	}
	host := multigroupbilling.Host[*Group, UserSubscription]{
		ResolveGroup: func(ctx context.Context, id int64) (*multigroupbilling.Group[*Group], error) {
			group, err := s.resolveAPIKeyCandidateGroup(ctx, apiKey, id)
			if err != nil || group == nil {
				return nil, err
			}
			return &multigroupbilling.Group[*Group]{Value: group, Platform: DefaultAPIKeyPlatform(group.Platform), Active: group.IsActive(), Subscription: group.IsSubscriptionType()}, nil
		},
	}
	if subscriptionSvc != nil {
		host.ListSubscriptions = subscriptionSvc.ListUsableSubscriptionsForGroup
	}
	if s.billingBalanceResolver != nil {
		host.Balance = s.billingBalanceResolver.GetUserBalance
	}
	selected, err := multigroupbilling.Select(ctx, &key, host)
	if key.InferredPlatform != "" {
		apiKey.Platform = key.InferredPlatform
	}
	if key.ResolvedBalance != nil && apiKey.User != nil {
		apiKey.User.Balance = *key.ResolvedBalance
	}
	if errors.Is(err, multigroupbilling.ErrNoUsableGroup) {
		return nil, ErrNoUsableAPIKeyGroup
	}
	if err != nil {
		return nil, err
	}
	if selected.Group == nil {
		return &APIKeyGroupSelection{}, nil
	}
	group := selected.Group.Value
	apiKey.GroupID = &group.ID
	apiKey.Group = group
	return &APIKeyGroupSelection{Group: group, Subscription: selected.Subscription}, nil
}

func (s *APIKeyService) resolveAPIKeyCandidateGroup(ctx context.Context, apiKey *APIKey, groupID int64) (*Group, error) {
	if apiKey != nil && apiKey.Group != nil && apiKey.Group.ID == groupID {
		return apiKey.Group, nil
	}
	return s.groupRepo.GetByIDLite(ctx, groupID)
}
