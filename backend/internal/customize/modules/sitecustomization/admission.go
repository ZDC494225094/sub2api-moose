package sitecustomization

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ErrExtensionDisabled is returned when attempting to create new site customization
// content while the extension is disabled.
var ErrExtensionDisabled = infraerrors.Forbidden(
	"CUSTOM_EXTENSION_DISABLED",
	"Site customization extension is disabled; cannot create new custom content",
)

// SettingsFilterAdmission controls whether custom UI settings are exposed in public APIs.
type SettingsFilterAdmission interface {
	// ShouldExposeCustomUI returns true if footer links should be included in public settings responses.
	// Returns false when the extension is disabled.
	ShouldExposeCustomUI(ctx context.Context) bool
}
