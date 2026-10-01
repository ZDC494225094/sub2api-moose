package service

import (
	"context"
	"log/slog"
)

// A failed create response is not evidence that the provider rejected the order:
// it may have accepted it before a timeout, or only our detail persistence failed.
// Keep pending orders queryable and preserve every concurrently advanced state.
// In particular, do not release a coupon or use the fulfillment-retry FAILED
// state here. Normal callback, query, cancellation and expiry own the lifecycle.
func (s *PaymentService) recordPaymentCreationFailure(ctx context.Context, orderID int64, cause error) error {
	slog.Warn("payment creation response failed; order lifecycle unchanged", "orderID", orderID)
	s.writeAuditLog(ctx, orderID, "ORDER_CREATE_RESPONSE_FAILED", "system", map[string]any{
		"outcome":                      "unconfirmed",
		"state_preserved":              true,
		"coupon_reservation_preserved": true,
	})
	// Keep structured provider errors and their metadata intact. Do not persist
	// raw provider messages here: they can contain credentials or request URLs.
	return cause
}
