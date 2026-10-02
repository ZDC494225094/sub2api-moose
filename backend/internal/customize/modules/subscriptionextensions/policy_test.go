package subscriptionextensions

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
)

type state struct {
	enabled bool
	err     error
}

func (s state) Enabled(_ context.Context, id string) (bool, error) {
	if id != ModuleID {
		panic(id)
	}
	return s.enabled, s.err
}
func TestNewPolicyAndHistoricalOwnership(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		s    StateReader
		want string
	}{{nil, Native}, {state{}, Native}, {state{enabled: true}, Independent}} {
		got, err := NewPolicy(ctx, tc.s)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
	_, err := NewPolicy(ctx, state{err: errors.New("offline")})
	require.Error(t, err)
	for _, p := range []string{"", Independent, Native} {
		got, err := Resolve(p)
		require.NoError(t, err)
		if p == "" {
			p = Independent
		}
		require.Equal(t, p, got)
	}
	_, err = Resolve("future-unknown")
	require.Error(t, err)
}
func TestSnapshotsFreezeAndPreserveProviderData(t *testing.T) {
	src := map[string]any{"provider_key": "alipay", "campaign": "old"}
	for _, policy := range []string{Native, Independent} {
		snapshot, err := WithSnapshot(src, policy)
		require.NoError(t, err)
		require.NotContains(t, src, SnapshotKey)
		require.Equal(t, "alipay", snapshot["provider_key"])
		require.Equal(t, "old", snapshot["campaign"])
		got, err := FromSnapshot(snapshot)
		require.NoError(t, err)
		require.Equal(t, policy, got)
	}
	for _, snapshot := range []map[string]any{nil, src} {
		got, err := FromSnapshot(snapshot)
		require.NoError(t, err)
		require.Equal(t, Independent, got)
	}
	for _, value := range []any{"", false, 42, nil, "unknown"} {
		_, err := FromSnapshot(map[string]any{SnapshotKey: value})
		require.Error(t, err)
	}
	_, err := WithSnapshot(src, "unknown")
	require.Error(t, err)
}
