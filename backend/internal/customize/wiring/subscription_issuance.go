package wiring

import (
	"context"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/subscriptionextensions"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type subscriptionIssuance struct {
	states subscriptionextensions.StateReader
}

func (a *subscriptionIssuance) NewSubscriptionPolicy(ctx context.Context) (string, error) {
	manifest, ok := customize.Lookup(subscriptionextensions.ModuleID)
	if a == nil || a.states == nil || !ok || !manifest.Managed {
		return "", fmt.Errorf("subscription issuance admission unavailable")
	}
	return subscriptionextensions.NewPolicy(ctx, a.states)
}
func ProvideSubscriptionIssuance(settings service.SettingRepository) service.SubscriptionIssuanceAdmission {
	return &subscriptionIssuance{states: customize.NewManager(marketingSettingReader{settings})}
}
