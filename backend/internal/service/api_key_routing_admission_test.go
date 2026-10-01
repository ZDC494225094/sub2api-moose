package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type routingAdmissionProbe struct {
	on    bool
	err   error
	calls int
}

func (p *routingAdmissionProbe) MultiGroupEnabled(context.Context) (bool, error) {
	p.calls++
	return p.on, p.err
}

// Only the methods Create/Update use are implemented; anything else panics.
type routingKeyRepo struct {
	APIKeyRepository
	stored  *APIKey
	created []*APIKey
	updates []APIKeyUpdateFields
}

func (r *routingKeyRepo) ExistsByKey(context.Context, string) (bool, error) { return false, nil }
func (r *routingKeyRepo) Create(_ context.Context, key *APIKey) error {
	key.ID = int64(len(r.created) + 1)
	clone := *key
	r.created = append(r.created, &clone)
	return nil
}
func (r *routingKeyRepo) GetByID(context.Context, int64) (*APIKey, error) {
	clone := *r.stored
	clone.GroupIDs = append([]int64(nil), r.stored.GroupIDs...)
	return &clone, nil
}
func (r *routingKeyRepo) Update(_ context.Context, key *APIKey, fields APIKeyUpdateFields) error {
	r.updates = append(r.updates, fields)
	clone := *key
	r.stored = &clone
	return nil
}

func newRoutingAdmissionService(repo *routingKeyRepo, admission APIKeyRoutingAdmission) *APIKeyService {
	groups := &apiKeySelectionGroupRepo{groups: map[int64]*Group{
		1: {ID: 1, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
		2: {ID: 2, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
		3: {ID: 3, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
	}}
	svc := NewAPIKeyService(repo, &apiKeySelectionUserRepo{user: &User{ID: 7, Status: StatusActive}}, groups, nil, nil, nil, &config.Config{})
	if admission != nil {
		svc.SetRoutingAdmission(admission)
	}
	return svc
}

func requireRoutingReason(t *testing.T, err error, reason string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, reason, infraerrors.Reason(err), err.Error())
}

func TestAPIKeyCreateRecordsDurableRoutingOwnership(t *testing.T) {
	ctx := context.Background()
	one := int64(1)
	multi := CreateAPIKeyRequest{Name: "multi", GroupIDs: []int64{1, 2}, BillingPriority: BillingPrioritySubscriptionFirst}
	single := CreateAPIKeyRequest{Name: "single", GroupID: &one, GroupIDs: []int64{1}, Platform: PlatformOpenAI, BillingPriority: BillingPriorityBalanceFirst}

	t.Run("enabled keeps multi-group behavior", func(t *testing.T) {
		repo, probe := &routingKeyRepo{}, &routingAdmissionProbe{on: true}
		key, err := newRoutingAdmissionService(repo, probe).Create(ctx, 7, multi)
		require.NoError(t, err)
		require.Equal(t, multigroupbilling.LegacyRouting, key.RoutingPolicy)
		require.Equal(t, []int64{1, 2}, key.GroupIDs)
		require.Equal(t, BillingPrioritySubscriptionFirst, key.BillingPriority)
		require.Equal(t, multigroupbilling.LegacyRouting, repo.created[0].RoutingPolicy)
		require.Equal(t, 1, probe.calls)
	})
	t.Run("enabled single-group key is still extension-owned", func(t *testing.T) {
		repo := &routingKeyRepo{}
		key, err := newRoutingAdmissionService(repo, &routingAdmissionProbe{on: true}).Create(ctx, 7, single)
		require.NoError(t, err)
		require.Equal(t, multigroupbilling.LegacyRouting, key.RoutingPolicy)
	})
	t.Run("disabled creates upstream-routed key from the existing UI payload", func(t *testing.T) {
		repo := &routingKeyRepo{}
		key, err := newRoutingAdmissionService(repo, &routingAdmissionProbe{}).Create(ctx, 7, single)
		require.NoError(t, err)
		require.Equal(t, multigroupbilling.NativeRouting, key.RoutingPolicy)
		require.Equal(t, int64(1), *key.GroupID)
		require.Equal(t, BillingPriorityBalanceFirst, key.BillingPriority)
		require.Equal(t, PlatformOpenAI, key.Platform)
	})
	t.Run("disabled rejects extension configuration before any write", func(t *testing.T) {
		for _, req := range []CreateAPIKeyRequest{multi, {Name: "sub", GroupID: &one, BillingPriority: BillingPrioritySubscriptionFirst}} {
			repo := &routingKeyRepo{}
			_, err := newRoutingAdmissionService(repo, &routingAdmissionProbe{}).Create(ctx, 7, req)
			requireRoutingReason(t, err, "CUSTOM_EXTENSION_DISABLED")
			require.Empty(t, repo.created)
		}
	})
	t.Run("unreadable switch never guesses permanent ownership", func(t *testing.T) {
		repo := &routingKeyRepo{}
		failure := errors.New("settings unavailable")
		_, err := newRoutingAdmissionService(repo, &routingAdmissionProbe{on: true, err: failure}).Create(ctx, 7, single)
		require.ErrorIs(t, err, failure)
		require.Empty(t, repo.created)
	})
	t.Run("no extension host behaves like upstream", func(t *testing.T) {
		repo := &routingKeyRepo{}
		svc := newRoutingAdmissionService(repo, nil)
		key, err := svc.Create(ctx, 7, single)
		require.NoError(t, err)
		require.Equal(t, multigroupbilling.NativeRouting, key.RoutingPolicy)
		_, err = svc.Create(ctx, 7, multi)
		requireRoutingReason(t, err, "CUSTOM_EXTENSION_DISABLED")
		require.Len(t, repo.created, 1)
	})
}

func TestAPIKeyUpdateRespectsRoutingOwnership(t *testing.T) {
	ctx := context.Background()
	one := int64(1)
	name := "renamed"
	sub, bal := BillingPrioritySubscriptionFirst, BillingPriorityBalanceFirst
	platform := PlatformOpenAI
	legacyMulti := func() *APIKey {
		return &APIKey{ID: 9, UserID: 7, Key: "k", Status: StatusActive, GroupID: &one, GroupIDs: []int64{1, 2}, Platform: PlatformOpenAI,
			BillingPriority: sub, RoutingPolicy: multigroupbilling.LegacyRouting}
	}
	native := func() *APIKey {
		return &APIKey{ID: 9, UserID: 7, Key: "k", Status: StatusActive, GroupID: &one, GroupIDs: []int64{1}, Platform: PlatformOpenAI,
			BillingPriority: bal, RoutingPolicy: multigroupbilling.NativeRouting}
	}
	// The Keys page resends platform, group_ids and billing_priority on every save.
	echo := func(key *APIKey, groups []int64, priority string) UpdateAPIKeyRequest {
		return UpdateAPIKeyRequest{Name: &name, Platform: &platform, GroupIDs: groups, GroupIDsSet: true, BillingPriority: &priority}
	}
	cases := []struct {
		name      string
		stored    *APIKey
		req       func(*APIKey) UpdateAPIKeyRequest
		on        bool
		reason    string
		wantReads int
		wantIDs   []int64
	}{
		{name: "legacy edit with unchanged multi config while disabled", stored: legacyMulti(), req: func(k *APIKey) UpdateAPIKeyRequest { return echo(k, []int64{1, 2}, sub) }, wantIDs: []int64{1, 2}},
		{name: "legacy name-only edit while disabled", stored: legacyMulti(), req: func(*APIKey) UpdateAPIKeyRequest { return UpdateAPIKeyRequest{Name: &name} }, wantIDs: []int64{1, 2}},
		{name: "legacy reduce to single group while disabled", stored: legacyMulti(), req: func(k *APIKey) UpdateAPIKeyRequest { return echo(k, []int64{2}, bal) }, wantIDs: []int64{2}},
		{name: "legacy add group while disabled", stored: legacyMulti(), req: func(k *APIKey) UpdateAPIKeyRequest { return echo(k, []int64{1, 2, 3}, sub) }, reason: "CUSTOM_EXTENSION_DISABLED", wantReads: 1},
		{name: "legacy add group while enabled", stored: legacyMulti(), on: true, req: func(k *APIKey) UpdateAPIKeyRequest { return echo(k, []int64{1, 2, 3}, sub) }, wantReads: 1, wantIDs: []int64{1, 2, 3}},
		{name: "native single group edit", stored: native(), req: func(k *APIKey) UpdateAPIKeyRequest { return echo(k, []int64{3}, bal) }, wantIDs: []int64{3}},
		{name: "native multi rejected even when enabled", stored: native(), on: true, req: func(k *APIKey) UpdateAPIKeyRequest { return echo(k, []int64{1, 2}, bal) }, reason: "API_KEY_NATIVE_ROUTING"},
		{name: "native subscription-first rejected", stored: native(), on: true, req: func(*APIKey) UpdateAPIKeyRequest { return UpdateAPIKeyRequest{BillingPriority: &sub} }, reason: "API_KEY_NATIVE_ROUTING"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, probe := &routingKeyRepo{stored: tc.stored}, &routingAdmissionProbe{on: tc.on}
			before := *tc.stored
			got, err := newRoutingAdmissionService(repo, probe).Update(ctx, 9, 7, tc.req(tc.stored))
			require.Equal(t, tc.wantReads, probe.calls)
			if tc.reason != "" {
				requireRoutingReason(t, err, tc.reason)
				require.Empty(t, repo.updates, "rejected edits must not write")
				require.Equal(t, before.GroupIDs, repo.stored.GroupIDs)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantIDs, got.GroupIDs)
			require.Equal(t, tc.stored.RoutingPolicy, got.RoutingPolicy, "ownership never changes on update")
		})
	}
}
