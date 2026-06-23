package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type apiKeySelectionGroupRepo struct {
	groupRepoNoop
	groups map[int64]*Group
}

func (r *apiKeySelectionGroupRepo) GetByID(_ context.Context, id int64) (*Group, error) {
	group := r.groups[id]
	if group == nil {
		return nil, ErrGroupNotFound
	}
	cp := *group
	return &cp, nil
}

func TestSelectUsableGroupForAPIKeyBalanceFirstFallsBackToSubscription(t *testing.T) {
	now := time.Now()
	dailyLimit := 1.0
	standard := &Group{ID: 10, Name: "balance", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	subscription := &Group{ID: 20, Name: "sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		standard.ID:     standard,
		subscription.ID: subscription,
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:                 2001,
		UserID:             701,
		GroupID:            subscription.ID,
		Status:             SubscriptionStatusActive,
		StartsAt:           now.Add(-time.Hour),
		ExpiresAt:          now.Add(24 * time.Hour),
		DailyWindowStart:   &now,
		WeeklyWindowStart:  &now,
		MonthlyWindowStart: &now,
	})
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	subscriptionSvc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	apiKey := &APIKey{
		ID:              1,
		UserID:          701,
		User:            &User{ID: 701, Status: StatusActive, Balance: 5},
		GroupIDs:        []int64{standard.ID, subscription.ID},
		BillingPriority: BillingPriorityBalanceFirst,
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, standard.ID, selection.Group.ID)
	require.Nil(t, selection.Subscription)

	apiKey.User.Balance = 0
	selection, err = apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, subscription.ID, selection.Group.ID)
	require.NotNil(t, selection.Subscription)
	require.Equal(t, int64(2001), selection.Subscription.ID)
}

func TestSelectUsableGroupForAPIKeySubscriptionFirstFallsBackToBalance(t *testing.T) {
	now := time.Now()
	dailyLimit := 1.0
	standard := &Group{ID: 11, Name: "balance", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	subscription := &Group{ID: 21, Name: "sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		standard.ID:     standard,
		subscription.ID: subscription,
	}}
	subRepo := newSubscriptionUserSubRepoStub()
	subRepo.seed(&UserSubscription{
		ID:                 2101,
		UserID:             702,
		GroupID:            subscription.ID,
		Status:             SubscriptionStatusActive,
		StartsAt:           now.Add(-time.Hour),
		ExpiresAt:          now.Add(24 * time.Hour),
		DailyWindowStart:   &now,
		WeeklyWindowStart:  &now,
		MonthlyWindowStart: &now,
	})
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	subscriptionSvc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	apiKey := &APIKey{
		ID:              2,
		UserID:          702,
		User:            &User{ID: 702, Status: StatusActive, Balance: 5},
		GroupIDs:        []int64{subscription.ID, standard.ID},
		BillingPriority: BillingPrioritySubscriptionFirst,
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, subscription.ID, selection.Group.ID)
	require.NotNil(t, selection.Subscription)

	subRepo.byID[2101].DailyUsageUSD = dailyLimit
	selection, err = apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, standard.ID, selection.Group.ID)
	require.Nil(t, selection.Subscription)
}

func TestSelectUsableGroupForAPIKeySkipsExhaustedSubscriptionInstance(t *testing.T) {
	now := time.Now()
	dailyLimit := 1.0
	subscription := &Group{ID: 30, Name: "sub", Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription, DailyLimitUSD: &dailyLimit}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{subscription.ID: subscription}}
	subRepo := newSubscriptionUserSubRepoStub()
	for _, item := range []struct {
		id    int64
		usage float64
	}{
		{id: 3001, usage: dailyLimit},
		{id: 3002, usage: 0},
	} {
		subRepo.seed(&UserSubscription{
			ID:                 item.id,
			UserID:             703,
			GroupID:            subscription.ID,
			Status:             SubscriptionStatusActive,
			StartsAt:           now.Add(-time.Hour),
			ExpiresAt:          now.Add(24 * time.Hour),
			DailyWindowStart:   &now,
			WeeklyWindowStart:  &now,
			MonthlyWindowStart: &now,
			DailyUsageUSD:      item.usage,
		})
	}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	subscriptionSvc := NewSubscriptionService(nil, subRepo, nil, nil, nil)
	apiKey := &APIKey{
		ID:              3,
		UserID:          703,
		User:            &User{ID: 703, Status: StatusActive, Balance: 0},
		GroupIDs:        []int64{subscription.ID},
		BillingPriority: BillingPrioritySubscriptionFirst,
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, subscriptionSvc)
	require.NoError(t, err)
	require.Equal(t, subscription.ID, selection.Group.ID)
	require.NotNil(t, selection.Subscription)
	require.Equal(t, int64(3002), selection.Subscription.ID)
}

func TestSelectUsableGroupForAPIKeySkipsGroupsFromOtherPlatforms(t *testing.T) {
	openai := &Group{ID: 40, Name: "openai", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	gemini := &Group{ID: 41, Name: "gemini", Platform: PlatformGemini, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		openai.ID: openai,
		gemini.ID: gemini,
	}}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	apiKey := &APIKey{
		ID:       4,
		UserID:   704,
		Platform: PlatformOpenAI,
		User:     &User{ID: 704, Status: StatusActive, Balance: 5},
		GroupIDs: []int64{gemini.ID, openai.ID},
	}

	selection, err := apiKeySvc.SelectUsableGroupForAPIKey(context.Background(), apiKey, nil)
	require.NoError(t, err)
	require.Equal(t, openai.ID, selection.Group.ID)
	require.Equal(t, PlatformOpenAI, apiKey.Group.Platform)
}

func TestValidateBindableGroupIDsRejectsMixedPlatforms(t *testing.T) {
	openai := &Group{ID: 50, Name: "openai", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	gemini := &Group{ID: 51, Name: "gemini", Platform: PlatformGemini, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		openai.ID: openai,
		gemini.ID: gemini,
	}}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)

	platform, err := apiKeySvc.validateBindableGroupIDs(context.Background(), &User{ID: 705}, "", []int64{openai.ID, gemini.ID})
	require.ErrorIs(t, err, ErrAPIKeyGroupPlatformMismatch)
	require.Empty(t, platform)
}

func TestValidateBindableGroupIDsRejectsPlatformMismatch(t *testing.T) {
	openai := &Group{ID: 60, Name: "openai", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	groupRepo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{openai.ID: openai}}
	apiKeySvc := NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)

	platform, err := apiKeySvc.validateBindableGroupIDs(context.Background(), &User{ID: 706}, PlatformGemini, []int64{openai.ID})
	require.ErrorIs(t, err, ErrAPIKeyGroupPlatformMismatch)
	require.Empty(t, platform)
}
