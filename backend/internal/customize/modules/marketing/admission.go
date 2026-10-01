package marketing

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// NewBusinessAdmission is a host-supplied, request-time gate. It must return an
// error when disabled or when switch state cannot be read. It is deliberately
// absent from historical reads, settlement, reservation restoration and release.
// Admission does not revoke work already in flight; the host transaction still
// owns commit/rollback. It must not be used as a substitute for that transaction.
type NewBusinessAdmission interface {
	RequireNewBusiness(context.Context) error
}

var ErrMarketingAdmissionUnavailable = infraerrors.ServiceUnavailable("MARKETING_ADMISSION_UNAVAILABLE", "marketing admission is not configured")

func requireNewBusiness(ctx context.Context, admission NewBusinessAdmission) error {
	if admission == nil {
		return ErrMarketingAdmissionUnavailable
	}
	return admission.RequireNewBusiness(ctx)
}
