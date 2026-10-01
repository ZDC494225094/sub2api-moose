package wiring

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/billingscheduling"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

const billingSchedulingModuleID = "billing-scheduling"

type billingSchedulingStateReader interface {
	Enabled(context.Context, string) (bool, error)
}

// BillingSchedulingAdmission is a type-safe wrapper for billing-scheduling admission.
type BillingSchedulingAdmission struct {
	impl *billingSchedulingAdmission
}

// billingSchedulingAdmission implements all billing-scheduling admission interfaces.
type billingSchedulingAdmission struct {
	states billingSchedulingStateReader
	lookup func(string) (customize.Manifest, bool)
}

var (
	_ service.RateMultiplierAdmission = (*billingSchedulingAdmission)(nil)
	_ service.TimePricingAdmission    = (*billingSchedulingAdmission)(nil)
)

// AllowRateMultiplier checks if custom rate multipliers are allowed.
func (a *billingSchedulingAdmission) AllowRateMultiplier(ctx context.Context) error {
	return a.checkEnabled(ctx)
}

// AllowTimePricing checks if time-based pricing is allowed.
func (a *billingSchedulingAdmission) AllowTimePricing(ctx context.Context) error {
	return a.checkEnabled(ctx)
}

// AllowCustomUsage checks if custom usage calculations are allowed.
func (a *billingSchedulingAdmission) AllowCustomUsage(ctx context.Context) error {
	return a.checkEnabled(ctx)
}

// AllowCompositeScheduling checks if composite scheduling is allowed.
func (a *billingSchedulingAdmission) AllowCompositeScheduling(ctx context.Context) error {
	return a.checkEnabled(ctx)
}

func (a *billingSchedulingAdmission) checkEnabled(ctx context.Context) error {
	if a == nil || a.lookup == nil {
		return billingscheduling.ErrExtensionDisabled
	}
	manifest, exists := a.lookup(billingSchedulingModuleID)
	if !exists || manifest.ID != billingSchedulingModuleID {
		return billingscheduling.ErrExtensionDisabled
	}
	if !manifest.Managed {
		return billingscheduling.ErrExtensionDisabled
	}
	if a.states == nil {
		return billingscheduling.ErrExtensionDisabled
	}
	enabled, err := a.states.Enabled(ctx, billingSchedulingModuleID)
	if err != nil {
		return err
	}
	if !enabled {
		return billingscheduling.ErrExtensionDisabled
	}
	return nil
}

// ProvideBillingSchedulingAdmission returns a wrapped admission implementation
// for billing-scheduling extension checks.
func ProvideBillingSchedulingAdmission(settings service.SettingRepository) BillingSchedulingAdmission {
	impl := &billingSchedulingAdmission{
		states: customize.NewManager(billingSchedulingSettingReader{settings}),
		lookup: customize.Lookup,
	}
	// Inject into service layer for feature-specific admission checks
	service.SetTimePricingAdmission(impl)
	service.SetRateMultiplierAdmission(impl)
	return BillingSchedulingAdmission{impl: impl}
}

// Unwrap returns the underlying admission implementation as any.
func (w BillingSchedulingAdmission) Unwrap() any {
	return w.impl
}

// billingSchedulingSettingReader wraps the setting repository for extension state checks.
type billingSchedulingSettingReader struct{ service.SettingRepository }

func (r billingSchedulingSettingReader) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	if r.SettingRepository == nil {
		return nil, billingscheduling.ErrExtensionDisabled
	}
	return r.SettingRepository.GetMultiple(ctx, keys)
}
