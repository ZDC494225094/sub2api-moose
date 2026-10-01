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

// AnnouncementAdmission controls whether administrators can create new announcements.
type AnnouncementAdmission interface {
	// CheckCreate returns nil if new announcements may be created, or
	// ErrExtensionDisabled if the extension is turned off.
	CheckCreate(ctx context.Context) error
}

// SettingsFilterAdmission controls whether custom UI settings are exposed in public APIs.
type SettingsFilterAdmission interface {
	// ShouldExposeCustomUI returns true if custom menus, footer links, and
	// custom endpoints should be included in public settings responses.
	// Returns false when the extension is disabled.
	ShouldExposeCustomUI(ctx context.Context) bool
}
