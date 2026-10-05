package billingscheduling

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

func (s state) Enabled(context.Context, string) (bool, error) { return s.enabled, s.err }
func TestProbeAdmissionFailsClosed(t *testing.T) {
	for _, reader := range []StateReader{nil, state{}, state{true, errors.New("offline")}} {
		require.False(t, Enabled(context.Background(), reader))
	}
	require.True(t, Enabled(context.Background(), state{enabled: true}))
}
func TestDisabledOptionsRestoreNativeProbePolicy(t *testing.T) {
	interval, sync := ProbeOptions(false, 30, 1, true)
	require.Equal(t, 30, interval)
	require.False(t, sync)
	interval, sync = ProbeOptions(false, 1, 1, true)
	require.Equal(t, 5, interval)
	require.False(t, sync)
	interval, sync = ProbeOptions(true, 30, 1, true)
	require.Equal(t, 1, interval)
	require.True(t, sync)
}
