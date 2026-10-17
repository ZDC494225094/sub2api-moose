package botnotify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNextPushDelayAlignsToWallClock(t *testing.T) {
	zone := time.FixedZone("CST", 8*3600)
	at := func(hour, min, sec int) time.Time {
		return time.Date(2026, 10, 17, hour, min, sec, 0, zone)
	}
	// Hourly pushes land exactly on the hour.
	require.Equal(t, 39*time.Minute+30*time.Second, nextPushDelay(at(12, 20, 30), time.Hour))
	// Exactly on a boundary waits for the following one, never zero.
	require.Equal(t, time.Hour, nextPushDelay(at(12, 0, 0), time.Hour))
	// A 30-minute interval aligns to :00 and :30.
	require.Equal(t, 10*time.Minute, nextPushDelay(at(12, 20, 0), 30*time.Minute))
	// The last run of the day rolls to midnight.
	require.Equal(t, time.Minute, nextPushDelay(at(23, 59, 0), time.Hour))
	// Sub-minute/invalid intervals fall back to hourly alignment.
	require.Equal(t, 39*time.Minute+30*time.Second, nextPushDelay(at(12, 20, 30), 0))
}
