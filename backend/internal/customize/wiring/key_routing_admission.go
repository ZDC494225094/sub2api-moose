package wiring

import (
	"context"
	"errors"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var errKeyRoutingAdmissionUnavailable = errors.New("multi-group key admission is not configured")

type keyRoutingAdmission struct {
	states marketingStateReader
	lookup func(string) (customize.Manifest, bool)
}

// MultiGroupEnabled reads the shared switch on every decision; nothing is
// cached. A missing registration or unreadable state is an error, never an
// implicit "enabled": key ownership chosen from it is permanent.
func (a *keyRoutingAdmission) MultiGroupEnabled(ctx context.Context) (bool, error) {
	if a == nil || a.lookup == nil || a.states == nil {
		return false, errKeyRoutingAdmissionUnavailable
	}
	manifest, exists := a.lookup(multigroupbilling.ModuleID)
	if !exists || manifest.ID != multigroupbilling.ModuleID || !manifest.Managed {
		return false, errKeyRoutingAdmissionUnavailable
	}
	return a.states.Enabled(ctx, multigroupbilling.ModuleID)
}

func ProvideKeyRoutingAdmission(settings service.SettingRepository) service.APIKeyRoutingAdmission {
	return &keyRoutingAdmission{states: customize.NewManager(marketingSettingReader{settings}), lookup: customize.Lookup}
}
