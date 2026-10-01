package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPIKeyGroupPolicyBridgePreservesHostErrorsAndEmptyKeys(t *testing.T) {
	svc := NewAPIKeyService(nil, nil, &apiKeySelectionGroupRepo{groups: map[int64]*Group{}}, nil, nil, nil, nil)
	selected, err := svc.SelectUsableGroupForAPIKey(context.Background(), nil, nil)
	require.Nil(t, selected)
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	empty := &APIKey{Platform: "  OPENAI  "}
	selected, err = svc.SelectUsableGroupForAPIKey(context.Background(), empty, nil)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Nil(t, selected.Group)
	require.Equal(t, "  OPENAI  ", empty.Platform)
	key := &APIKey{GroupIDs: []int64{99}, Platform: PlatformOpenAI}
	selected, err = svc.SelectUsableGroupForAPIKey(context.Background(), key, nil)
	require.Nil(t, selected)
	require.ErrorIs(t, err, ErrNoUsableAPIKeyGroup)
	require.Nil(t, key.GroupID)
	require.Nil(t, key.Group)
}

func TestAPIKeyGroupPolicyBridgeReusesHostGroupAndDoesNotRewritePersistentBinding(t *testing.T) {
	cached := &Group{ID: 10, Platform: PlatformOpenAI, Status: StatusActive}
	repo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{}}
	svc := NewAPIKeyService(nil, nil, repo, nil, nil, nil, nil)
	id := int64(10)
	key := &APIKey{GroupID: &id, GroupIDs: []int64{10, 10, 0}, Group: cached, Platform: " OpenAI "}
	selected, err := svc.SelectUsableGroupForAPIKey(context.Background(), key, nil)
	require.NoError(t, err)
	require.Same(t, cached, selected.Group)
	require.Same(t, cached, key.Group)
	require.Equal(t, 0, repo.getLiteCalls)
	require.Equal(t, 0, repo.getByIDCalls)
	require.Equal(t, []int64{10, 10, 0}, key.GroupIDs)
	require.Equal(t, " OpenAI ", key.Platform, "normalization for selection must not rewrite a valid stored platform")
}

func TestAPIKeyGroupPolicyBridgeKeepsHostPreflightInChargeOfInactiveGroups(t *testing.T) {
	inactive := &Group{ID: 10, Platform: PlatformOpenAI, Status: StatusDisabled}
	repo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{10: inactive}}
	svc := NewAPIKeyService(nil, nil, repo, nil, nil, nil, nil)
	key := &APIKey{GroupIDs: []int64{10}, Platform: "unsupported-legacy-value"}
	selected, err := svc.SelectUsableGroupForAPIKey(context.Background(), key, nil)
	require.NoError(t, err)
	require.False(t, selected.Group.IsActive())
	require.Equal(t, PlatformOpenAI, key.Platform)
	require.Equal(t, int64(10), *key.GroupID)
	require.Equal(t, 1, repo.getLiteCalls)
	require.Equal(t, 0, repo.getByIDCalls)
	// Returning a fallback does not grant permission; both gateway middlewares
	// still perform the host group/status/subscription/balance checks afterwards.
}

func TestAPIKeyGroupPolicyBridgePropagatesOnlySuccessfulLiveBalance(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(map[bool]string{false: "live", true: "cached fallback"}[failed], func(t *testing.T) {
			repo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
				10: {ID: 10, Status: StatusActive},
				20: {ID: 20, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription},
			}}
			svc := NewAPIKeyService(nil, nil, repo, nil, nil, nil, nil)
			balance := &apiKeySelectionBalanceResolver{balance: 0}
			if failed {
				balance.err = errors.New("cache unavailable")
			}
			svc.SetBillingBalanceResolver(balance)
			key := &APIKey{UserID: 1, User: &User{ID: 1, Balance: 12}, GroupIDs: []int64{10, 20}, BillingPriority: BillingPrioritySubscriptionFirst}
			selected, err := svc.SelectUsableGroupForAPIKey(context.Background(), key, nil)
			require.NoError(t, err)
			require.Equal(t, 1, balance.calls)
			if failed {
				require.Equal(t, 12.0, key.User.Balance)
				require.Equal(t, int64(10), selected.Group.ID)
			} else {
				require.Equal(t, 0.0, key.User.Balance)
				require.Equal(t, int64(20), selected.Group.ID)
			}
			require.Nil(t, selected.Subscription, "no fabricated subscription when host service is absent")
		})
	}
}
