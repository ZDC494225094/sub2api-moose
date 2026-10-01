package service

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeyRoutingOwnershipCache(t *testing.T) {
	svc := &APIKeyService{}
	for _, policy := range []string{"", multigroupbilling.LegacyRouting, multigroupbilling.NativeRouting, "future-v2"} {
		t.Run(policy, func(t *testing.T) {
			key := &APIKey{ID: 1, UserID: 2, RoutingPolicy: policy, User: &User{ID: 2}}
			snapshot := svc.snapshotFromAPIKey(context.Background(), key)
			raw, err := json.Marshal(snapshot)
			require.NoError(t, err)
			var decoded APIKeyAuthSnapshot
			require.NoError(t, json.Unmarshal(raw, &decoded))
			restored, ok, err := svc.applyAuthCacheEntry("key", &APIKeyAuthCacheEntry{Snapshot: &decoded})
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, policy, restored.RoutingPolicy)
		})
	}
	_, ok, err := svc.applyAuthCacheEntry("old", &APIKeyAuthCacheEntry{Snapshot: &APIKeyAuthSnapshot{Version: 24}})
	require.NoError(t, err)
	require.False(t, ok, "reload old cache entries missing durable ownership")
}
func TestAPIKeyRoutingOwnershipNativeBypassesCustomSelection(t *testing.T) {
	primary := int64(1)
	group := &Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive}
	key := &APIKey{RoutingPolicy: multigroupbilling.NativeRouting, GroupID: &primary, Group: group, GroupIDs: []int64{99}, BillingPriority: BillingPrioritySubscriptionFirst}
	// Nil dependencies prove this path does not resolve candidates or query balance.
	result, err := (&APIKeyService{}).SelectUsableGroupForAPIKey(context.Background(), key, nil)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Same(t, group, key.Group)
	require.Same(t, &primary, key.GroupID)
	require.Equal(t, []int64{99}, key.GroupIDs)
	key.RoutingPolicy = "future-v2"
	_, err = (&APIKeyService{}).SelectUsableGroupForAPIKey(context.Background(), key, nil)
	require.ErrorIs(t, err, multigroupbilling.ErrUnknownRoutingPolicy)
	require.Same(t, group, key.Group)
}
