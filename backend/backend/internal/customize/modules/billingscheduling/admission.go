package billingscheduling

import (
	"context"
	"errors"
)

var (
	// ErrExtensionDisabled is returned when the billing-scheduling extension is disabled.
	ErrExtensionDisabled = errors.New("BILLING_SCHEDULING_EXTENSION_DISABLED")
)

// RateMultiplierAdmission controls access to custom rate multiplier features.
type RateMultiplierAdmission interface {
	// AllowRateMultiplier checks if custom rate multipliers are allowed.
	// Returns ErrExtensionDisabled when the extension is off.
	AllowRateMultiplier(ctx context.Context) error
}

// TimePricingAdmission controls access to time-based pricing features.
type TimePricingAdmission interface {
	// AllowTimePricing checks if time-based pricing is allowed.
	// Returns ErrExtensionDisabled when the extension is off.
	AllowTimePricing(ctx context.Context) error
}

// CustomUsageAdmission controls access to custom usage calculation features.
type CustomUsageAdmission interface {
	// AllowCustomUsage checks if custom usage calculations are allowed.
	// Returns ErrExtensionDisabled when the extension is off.
	AllowCustomUsage(ctx context.Context) error
}

// CompositeSchedulingAdmission controls access to composite scheduling features.
type CompositeSchedulingAdmission interface {
	// AllowCompositeScheduling checks if composite scheduling is allowed.
	// Returns ErrExtensionDisabled when the extension is off.
	AllowCompositeScheduling(ctx context.Context) error
}
