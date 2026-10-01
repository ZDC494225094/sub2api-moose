package wiring

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type billingSchedulingSettingsProbe struct {
	service.SettingRepository
	values map[string]string
	err    error
	calls  int
	ctx    context.Context
	keys   []string
}

func (p *billingSchedulingSettingsProbe) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	p.calls++
	p.ctx = ctx
	p.keys = append([]string(nil), keys...)
	return p.values, p.err
}

// Exercise the real catalog, settings reader and production provider, including
// the legacy entry points used by token billing and profit-control scheduling.
func TestBillingSchedulingProductionAdmissionControlsLegacyPricing(t *testing.T) {
	t.Cleanup(func() {
		service.SetRateMultiplierAdmission(nil)
		service.SetTimePricingAdmission(nil)
	})
	settings := &billingSchedulingSettingsProbe{}
	ProvideBillingSchedulingAdmission(settings)
	group := &service.Group{
		SubscriptionType: service.SubscriptionTypeSubscription,
		PeakRateEnabled:  true, PeakStart: "00:00", PeakEnd: "23:59", PeakRateMultiplier: 3,
	}
	pricing := &service.ChannelTimePricing{
		Timezone: "UTC",
		Periods:  []service.ChannelTimePricingPeriod{{StartTime: "00:00", EndTime: "23:59", Multiplier: 2}},
	}
	originalGroup := *group
	originalPricing := *pricing
	originalPricing.Periods = append([]service.ChannelTimePricingPeriod(nil), pricing.Periods...)
	at := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	key := customize.Key(billingSchedulingModuleID)
	for _, state := range []struct {
		name            string
		value           string
		err             error
		peak, timePrice float64
	}{
		{name: "missing", peak: 1, timePrice: 1},
		{name: "enabled", value: "true", peak: 3, timePrice: 2},
		{name: "disabled", value: "false", peak: 1, timePrice: 1},
		{name: "malformed", value: "yes", peak: 1, timePrice: 1},
		{name: "unavailable", value: "true", err: errors.New("settings unavailable"), peak: 1, timePrice: 1},
		{name: "re-enabled", value: "true", peak: 3, timePrice: 2},
	} {
		t.Run(state.name, func(t *testing.T) {
			settings.values = map[string]string{}
			if state.value != "" {
				settings.values[key] = state.value
			}
			settings.err = state.err
			calls := settings.calls
			require.Equal(t, state.peak, group.PeakMultiplierAt(at))
			require.NotNil(t, settings.ctx, "legacy peak calculation must still check admission")
			require.Equal(t, state.timePrice, pricing.MultiplierAt(at))
			require.NotNil(t, settings.ctx, "legacy time pricing must still check admission")
			require.Equal(t, calls+2, settings.calls, "read current state on every pricing decision")
			require.Equal(t, []string{key}, settings.keys, "unrelated extension flags must not interrupt billing")
			require.Equal(t, originalGroup, *group, "turning off billing must preserve saved group configuration")
			require.Equal(t, originalPricing, *pricing, "turning off billing must preserve saved pricing")
		})
	}
}

func TestBillingSchedulingProductionAdmissionPreservesRequestContext(t *testing.T) {
	t.Cleanup(func() {
		service.SetRateMultiplierAdmission(nil)
		service.SetTimePricingAdmission(nil)
	})
	settings := &billingSchedulingSettingsProbe{values: map[string]string{customize.Key(billingSchedulingModuleID): "true"}}
	ProvideBillingSchedulingAdmission(settings)
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "transaction context")
	group := &service.Group{SubscriptionType: service.SubscriptionTypeSubscription, PeakRateEnabled: true, PeakStart: "00:00", PeakEnd: "23:59", PeakRateMultiplier: 3}
	at := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	require.Equal(t, 3.0, group.PeakMultiplierAtWithAdmission(ctx, at))
	require.Same(t, ctx, settings.ctx)
	pricing := &service.ChannelTimePricing{Timezone: "UTC", Periods: []service.ChannelTimePricingPeriod{{StartTime: "00:00", EndTime: "23:59", Multiplier: 2}}}
	require.Equal(t, 2.0, pricing.MultiplierAtWithAdmission(ctx, at))
	require.Same(t, ctx, settings.ctx)
}

func TestBillingSchedulingProductionAdmissionFailsClosedWithoutSettings(t *testing.T) {
	t.Cleanup(func() {
		service.SetRateMultiplierAdmission(nil)
		service.SetTimePricingAdmission(nil)
	})
	ProvideBillingSchedulingAdmission(nil)
	at := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	group := &service.Group{SubscriptionType: service.SubscriptionTypeSubscription, PeakRateEnabled: true, PeakStart: "00:00", PeakEnd: "23:59", PeakRateMultiplier: 3}
	pricing := &service.ChannelTimePricing{Timezone: "UTC", Periods: []service.ChannelTimePricingPeriod{{StartTime: "00:00", EndTime: "23:59", Multiplier: 2}}}
	require.Equal(t, 1.0, group.PeakMultiplierAt(at))
	require.Equal(t, 1.0, pricing.MultiplierAt(at))
}

func TestBillingSchedulingAdmissionRejectsBrokenRegistration(t *testing.T) {
	for _, gate := range []*billingSchedulingAdmission{
		nil,
		{},
		{lookup: func(string) (customize.Manifest, bool) { return customize.Manifest{}, false }},
		{lookup: func(string) (customize.Manifest, bool) { return customize.Manifest{ID: "other", Managed: true}, true }},
		{lookup: func(string) (customize.Manifest, bool) {
			return customize.Manifest{ID: billingSchedulingModuleID}, true
		}},
		{lookup: customize.Lookup},
	} {
		require.Error(t, gate.AllowRateMultiplier(context.Background()))
		require.Error(t, gate.AllowTimePricing(context.Background()))
	}
}

func TestBillingSchedulingAdmissionSkipsUnconfiguredPricing(t *testing.T) {
	t.Cleanup(func() {
		service.SetRateMultiplierAdmission(nil)
		service.SetTimePricingAdmission(nil)
	})
	settings := &billingSchedulingSettingsProbe{err: errors.New("unused settings must not be read")}
	ProvideBillingSchedulingAdmission(settings)
	at := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	var nilGroup *service.Group
	var nilPricing *service.ChannelTimePricing
	require.Equal(t, 1.0, nilGroup.PeakMultiplierAt(at))
	require.Equal(t, 1.0, (&service.Group{}).PeakMultiplierAt(at))
	require.Equal(t, 1.0, (&service.Group{SubscriptionType: service.SubscriptionTypeSubscription}).PeakMultiplierAt(at))
	require.Equal(t, 1.0, nilPricing.MultiplierAt(at))
	require.Equal(t, 1.0, (&service.ChannelTimePricing{}).MultiplierAt(at))
	require.Zero(t, settings.calls, "native pricing without an enhancement must not read extension settings")
}
