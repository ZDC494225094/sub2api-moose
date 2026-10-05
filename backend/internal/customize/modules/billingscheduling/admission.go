// Package billingscheduling owns fork-only probe options and group rate sync.
// Native account rate sync, composite routing and request pricing are not gated.
package billingscheduling

import "context"

const ModuleID = "billing-scheduling"

type StateReader interface {
	Enabled(context.Context, string) (bool, error)
}

// Enabled fails closed for optional background enhancements, not native probes.
// A caller must preserve upstream observation/account-sync work on failure.
func Enabled(ctx context.Context, states StateReader) bool {
	if states == nil {
		return false
	}
	enabled, err := states.Enabled(ctx, ModuleID)
	return err == nil && enabled
}

func ProbeOptions(enabled bool, globalInterval, accountInterval int, autoSync bool) (int, bool) {
	if !enabled {
		if globalInterval < 5 {
			globalInterval = 5
		}
		return globalInterval, false
	}
	return accountInterval, autoSync
}
