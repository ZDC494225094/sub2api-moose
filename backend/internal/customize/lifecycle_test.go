package customize

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLifecycleSeesCommittedSwitchAndUnregisters(t *testing.T) {
	manager := NewManager(newStore())
	calls := 0
	unregister := RegisterLifecycle(func(id string) {
		require.Equal(t, "bot-notify", id)
		enabled, err := manager.Enabled(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, calls == 0, enabled)
		calls++
	})
	require.NoError(t, manager.SetEnabled(context.Background(), "bot-notify", true))
	require.NoError(t, manager.SetEnabled(context.Background(), "bot-notify", false))
	unregister()
	require.NoError(t, manager.SetEnabled(context.Background(), "bot-notify", true))
	require.Equal(t, 2, calls)
}
