package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type canvasManagedKeyRepoStub struct {
	APIKeyRepository
	stored  *APIKey
	user    *User
	group   *Group
	loadErr error
	creates int
}

func (r *canvasManagedKeyRepoStub) FindCanvasManagedKey(context.Context, int64, int64) (*APIKey, error) {
	if r.stored == nil {
		return nil, ErrAPIKeyNotFound
	}
	key := *r.stored
	key.User, key.Group = nil, nil
	return &key, nil
}
func (r *canvasManagedKeyRepoStub) Create(_ context.Context, key *APIKey) error {
	r.creates++
	key.ID = 9
	stored := *key
	r.stored = &stored
	return nil
}
func (r *canvasManagedKeyRepoStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	if r.loadErr != nil {
		return nil, r.loadErr
	}
	if r.stored == nil || r.stored.ID != id {
		return nil, ErrAPIKeyNotFound
	}
	key := *r.stored
	key.User, key.Group = r.user, r.group
	return &key, nil
}
func (*canvasManagedKeyRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) {
	return nil, nil
}

func TestCanvasManagedKeyLoadsRelationsBeforePrecharge(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "new_key", true: "existing_key"}[existing], func(t *testing.T) {
			charges := &canvasRepoStub{}
			billing, original := canvasTestBilling(charges)
			repo := &canvasManagedKeyRepoStub{user: original.User, group: original.Group}
			if existing {
				repo.stored = original
			}
			keys := billing.keys
			keys.apiKeyRepo = repo
			keys.cfg = billing.cfg
			keys.userRepo = &apiKeySelectionUserRepo{user: original.User}
			keys.groupRepo = &apiKeySelectionGroupRepo{groups: map[int64]*Group{original.Group.ID: original.Group}}
			keys.userSubRepo = newSubscriptionUserSubRepoStub()
			ctx := context.Background()
			key, err := keys.GetOrCreateCanvasManagedKey(ctx, original.UserID, original.Group.ID)
			require.NoError(t, err)
			require.NotNil(t, key.User)
			require.NotNil(t, key.Group)
			charge, _, created, err := billing.reserve(ctx, key, PlaygroundRunRequest{ID: "managed-key-run", Mode: "image", Model: "gpt-image-2", N: 1})
			require.NoError(t, err)
			require.True(t, created)
			require.Positive(t, charge.ReservedCost)
			require.Equal(t, 1, charges.reserves)
			if existing {
				require.Zero(t, repo.creates)
			} else {
				require.Equal(t, 1, repo.creates)
			}
			// Reload failures must stop the request instead of returning an incomplete key.
			repo.loadErr = errors.New("key reload failed")
			key, err = keys.GetOrCreateCanvasManagedKey(ctx, original.UserID, original.Group.ID)
			require.ErrorIs(t, err, repo.loadErr)
			require.Nil(t, key)
		})
	}
}
