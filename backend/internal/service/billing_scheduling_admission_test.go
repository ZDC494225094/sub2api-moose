package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type billingSchedulingAdmissionProbe struct {
	err error
	ctx context.Context
}

func (p *billingSchedulingAdmissionProbe) AllowRateMultiplier(ctx context.Context) error {
	p.ctx = ctx
	return p.err
}

func (p *billingSchedulingAdmissionProbe) AllowTimePricing(ctx context.Context) error {
	p.ctx = ctx
	return p.err
}

func TestBillingSchedulingAdmissionControlsTokenBilling(t *testing.T) {
	previous := rateMultiplierAdmission
	t.Cleanup(func() { SetRateMultiplierAdmission(previous) })
	gate := &billingSchedulingAdmissionProbe{}
	SetRateMultiplierAdmission(gate)
	key := &APIKey{Group: &Group{
		SubscriptionType: SubscriptionTypeSubscription,
		PeakRateEnabled:  true, PeakStart: "00:00", PeakEnd: "23:59", PeakRateMultiplier: 3,
		ImageRateIndependent: true, ImageRateMultiplier: 5,
	}}
	at := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	for _, enabled := range []bool{true, false, true} {
		gate.err = nil
		wantText := 12.0
		if !enabled {
			gate.err = errors.New("extension disabled or settings unavailable")
			wantText = 4
		}
		text, image := computePeakAwareMultipliers(key, 4, at)
		require.Equal(t, wantText, text, "switch must control the real token-billing path")
		require.Equal(t, 5.0, image, "existing independent image and base billing must be preserved")
		require.NotNil(t, gate.ctx)
	}
}

func TestBillingSchedulingAdmissionControlsPricingSchedule(t *testing.T) {
	previous := timePricingAdmission
	t.Cleanup(func() { SetTimePricingAdmission(previous) })
	gate := &billingSchedulingAdmissionProbe{}
	SetTimePricingAdmission(gate)
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "transaction context")
	resolved := &ResolvedPricing{
		Source: PricingSourceChannel,
		channelPricing: &ChannelModelPricing{TimePricing: &ChannelTimePricing{
			Timezone: "UTC", Periods: []ChannelTimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}},
		}},
	}
	schedule := resolvedTimePricingSchedule(ctx, resolved)
	require.NotNil(t, schedule)
	require.Equal(t, 2.0, schedule.Periods[0].Multiplier)
	require.Same(t, ctx, gate.ctx, "pricing previews must preserve request context")
	gate.err = errors.New("extension disabled")
	require.Nil(t, resolvedTimePricingSchedule(ctx, resolved), "preview must not advertise disabled time pricing")
	require.Same(t, ctx, gate.ctx)
}
