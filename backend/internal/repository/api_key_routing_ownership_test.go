package repository

import (
	"context"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyRoutingOwnershipPersistence(t *testing.T) {
	for _, policy := range []string{"", multigroupbilling.LegacyRouting, multigroupbilling.NativeRouting} {
		t.Run(policy, func(t *testing.T) {
			repo, client := newAPIKeyRepoSQLite(t)
			ctx := context.Background()
			user := mustCreateAPIKeyRepoUser(t, ctx, client, "ownership@test.com")
			key := &service.APIKey{UserID: user.ID, Key: "ownership-key", Name: "original", Status: service.StatusActive, RoutingPolicy: policy}
			expected, err := multigroupbilling.RoutingPolicy(policy)
			require.NoError(t, err)
			require.NoError(t, repo.Create(ctx, key))
			require.Equal(t, expected, key.RoutingPolicy)
			byID, err := repo.GetByID(ctx, key.ID)
			require.NoError(t, err)
			require.Equal(t, expected, byID.RoutingPolicy)
			byAuth, err := repo.GetByKeyForAuth(ctx, key.Key)
			require.NoError(t, err)
			require.Equal(t, expected, byAuth.RoutingPolicy)
			// Even a modified service object must not transfer ownership on ordinary updates.
			key.RoutingPolicy = "future-v2"
			key.Name = "renamed"
			require.NoError(t, repo.Update(ctx, key, service.APIKeyUpdateFields{Name: true}))
			persisted, err := client.APIKey.Get(ctx, key.ID)
			require.NoError(t, err)
			require.Equal(t, expected, persisted.CustomRoutingPolicy)
			require.Equal(t, "renamed", persisted.Name)
			require.NoError(t, repo.Delete(ctx, key.ID))
			persisted, err = client.APIKey.Get(mixins.SkipSoftDelete(ctx), key.ID)
			require.NoError(t, err)
			require.NotNil(t, persisted.DeletedAt)
			require.Equal(t, expected, persisted.CustomRoutingPolicy)
		})
	}
}

func TestAPIKeyRoutingOwnershipRejectsUnknownBeforeWrite(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	key := &service.APIKey{Key: "unknown", RoutingPolicy: "future-v2"}
	require.ErrorIs(t, repo.Create(ctx, key), multigroupbilling.ErrUnknownRoutingPolicy)
	count, err := client.APIKey.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
	require.Zero(t, key.ID)
}
