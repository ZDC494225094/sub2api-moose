package service

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeyBindingBridge(t *testing.T) {
	ctx := context.Background()
	primary := int64(1)
	platform := PlatformOpenAI
	group := &Group{ID: 1, Platform: platform, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}
	user := &User{ID: 7}
	repo := &apiKeySelectionGroupRepo{groups: map[int64]*Group{1: group}}
	svc := &APIKeyService{userRepo: &apiKeySelectionUserRepo{user: user}, groupRepo: repo}
	t.Run("absent patch needs no host services", func(t *testing.T) {
		key := &APIKey{GroupID: &primary, GroupIDs: []int64{1}, Platform: platform}
		fields := APIKeyUpdateFields{Name: true}
		require.NoError(t, (&APIKeyService{}).applyAPIKeyBindingUpdate(ctx, 7, key, UpdateAPIKeyRequest{}, &fields))
		require.Equal(t, APIKeyUpdateFields{Name: true}, fields)
		require.Same(t, &primary, key.GroupID)
	})
	t.Run("platform only does not persist groups", func(t *testing.T) {
		key := &APIKey{GroupID: &primary, GroupIDs: []int64{1}, Platform: platform}
		fields := APIKeyUpdateFields{Name: true}
		require.NoError(t, svc.applyAPIKeyBindingUpdate(ctx, 7, key, UpdateAPIKeyRequest{Platform: &platform}, &fields))
		require.Equal(t, APIKeyUpdateFields{Name: true, Platform: true}, fields)
		require.Same(t, &primary, key.GroupID)
	})
	t.Run("clear persists both group fields", func(t *testing.T) {
		key := &APIKey{GroupID: &primary, GroupIDs: []int64{1}, Platform: platform}
		fields := APIKeyUpdateFields{}
		require.NoError(t, svc.applyAPIKeyBindingUpdate(ctx, 7, key, UpdateAPIKeyRequest{GroupIDsSet: true}, &fields))
		require.Nil(t, key.GroupID)
		require.Empty(t, key.GroupIDs)
		require.Equal(t, DefaultAPIKeyPlatform(""), key.Platform)
		require.Equal(t, APIKeyUpdateFields{Platform: true, GroupID: true, GroupIDs: true}, fields)
	})
	for _, tc := range []struct {
		name string
		id   int64
		want error
	}{
		{"missing", 99, ErrGroupNotFound}, {"forbidden", 2, ErrGroupNotAllowed}, {"mismatch", 3, ErrAPIKeyGroupPlatformMismatch},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo.groups[2] = &Group{ID: 2, Platform: platform, IsExclusive: true, SubscriptionType: SubscriptionTypeStandard}
			repo.groups[3] = &Group{ID: 3, Platform: PlatformGemini, SubscriptionType: SubscriptionTypeStandard}
			key := &APIKey{GroupID: &primary, GroupIDs: []int64{1}, Platform: platform}
			fields := APIKeyUpdateFields{Name: true}
			err := svc.applyAPIKeyBindingUpdate(ctx, 7, key, UpdateAPIKeyRequest{GroupIDsSet: true, GroupIDs: []int64{tc.id}, Platform: &platform}, &fields)
			require.ErrorIs(t, err, tc.want)
			require.Equal(t, []int64{1}, key.GroupIDs)
			require.Equal(t, platform, key.Platform)
			require.Same(t, &primary, key.GroupID)
			require.Equal(t, APIKeyUpdateFields{Name: true}, fields)
		})
	}
	t.Run("create validates and infers platform", func(t *testing.T) {
		got, policy, err := svc.prepareAPIKeyBinding(ctx, user, CreateAPIKeyRequest{GroupIDs: []int64{1, 1}})
		require.NoError(t, err)
		require.Equal(t, multigroupbilling.NativeRouting, policy, "without an extension host new keys are upstream-routed")
		require.Equal(t, primary, *got.Primary)
		require.Equal(t, platform, got.Platform)
		require.Equal(t, []int64{1}, got.GroupIDs)
	})
	t.Run("missing user leaves fields intact", func(t *testing.T) {
		key := &APIKey{GroupID: &primary, Platform: platform}
		fields := APIKeyUpdateFields{}
		err := svc.applyAPIKeyBindingUpdate(ctx, 99, key, UpdateAPIKeyRequest{GroupIDsSet: true}, &fields)
		require.ErrorIs(t, err, ErrUserNotFound)
		require.Equal(t, APIKeyUpdateFields{}, fields)
		require.Same(t, &primary, key.GroupID)
	})
}
