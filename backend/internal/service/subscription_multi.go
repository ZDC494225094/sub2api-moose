package service

import (
	"context"
)

func (s *SubscriptionService) ListUsableSubscriptionsForGroup(ctx context.Context, userID int64, group *Group) ([]UserSubscription, error) {
	if group == nil {
		return nil, ErrGroupNotSubscriptionType
	}
	subs, err := s.userSubRepo.ListActiveByUserIDAndGroupID(ctx, userID, group.ID)
	if err != nil {
		return nil, err
	}
	out := make([]UserSubscription, 0, len(subs))
	for i := range subs {
		sub := subs[i]
		needsMaintenance, validateErr := s.ValidateAndCheckLimits(&sub, group)
		if validateErr != nil {
			continue
		}
		if needsMaintenance {
			maintenanceCopy := sub
			s.DoWindowMaintenance(&maintenanceCopy)
		}
		out = append(out, sub)
	}
	return out, nil
}
