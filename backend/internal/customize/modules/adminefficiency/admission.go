package adminefficiency

import (
	"context"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const ModuleID = "admin-efficiency"

var ErrExtensionDisabled = infraerrors.Forbidden("CUSTOM_EXTENSION_DISABLED", "extension admin-efficiency is disabled")

// StateReader is supplied by the host; no service, repository or framework dependency.
type StateReader interface {
	Enabled(context.Context, string) (bool, error)
}

// CheckWrite fails closed. Historical reads never use this admission point.
func CheckWrite(ctx context.Context, states StateReader) error {
	if states == nil {
		return ErrExtensionDisabled
	}
	enabled, err := states.Enabled(ctx, ModuleID)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrExtensionDisabled
	}
	return nil
}

// CheckUpstreamGroupChange permits unchanged historical labels during core edits.
func CheckUpstreamGroupChange(ctx context.Context, states StateReader, before, after string) error {
	if before == after {
		return nil
	}
	return CheckWrite(ctx, states)
}
