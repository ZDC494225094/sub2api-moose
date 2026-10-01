package service

import (
	"context"
	"fmt"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// reserveOrderCoupon is the host's order-creation seam. Coupon rules live in the
// marketing module; the host supplies the order transaction and ensures the
// reserved quote is the one used to calculate the provider's payment amount.
// No coupon request means no module dependency and no changed native behavior.
func (s *PaymentService) reserveOrderCoupon(ctx context.Context, req CreateOrderRequest, orderID int64) error {
	if req.UserCouponID <= 0 {
		return nil
	}
	if s.couponService == nil {
		return infraerrors.ServiceUnavailable("PAYMENT_COUPON_NOT_READY", "coupon service is not configured")
	}
	if dbent.TxFromContext(ctx) == nil {
		return infraerrors.InternalServer("PAYMENT_COUPON_TRANSACTION_REQUIRED", "coupon reservation requires the order transaction")
	}
	if req.couponQuote == nil {
		return infraerrors.Conflict("PAYMENT_COUPON_QUOTE_REQUIRED", "coupon must be quoted before creating an order")
	}
	reserved, err := s.couponService.ReserveCouponForOrder(ctx, orderID, ApplyPaymentCouponInput{
		UserID: req.UserID, OrderType: req.OrderType,
		OrderAmount: req.couponQuote.OriginalAmount, UserCouponID: req.UserCouponID,
	})
	if err != nil {
		return err
	}
	if reserved.DiscountedAmount != req.couponQuote.DiscountedAmount || reserved.DiscountAmount != req.couponQuote.DiscountAmount {
		return infraerrors.Conflict("PAYMENT_COUPON_QUOTE_CHANGED", "coupon changed; confirm the new price before retrying")
	}
	return nil
}

// consumePaymentCoupon also repairs already-completed historical orders. Both
// coupon records must change in one real host transaction, including on retries.
// When a caller owns a transaction, its commit/rollback remains the caller's duty.
func (s *PaymentService) consumePaymentCoupon(ctx context.Context, orderID int64) error {
	if s == nil || s.couponService == nil {
		return nil
	}
	if dbent.TxFromContext(ctx) != nil {
		if err := s.couponService.ConsumeReservedCouponByOrderID(ctx, orderID); err != nil {
			return fmt.Errorf("consume reserved coupon: %w", err)
		}
		return nil
	}
	if s.entClient == nil {
		return infraerrors.InternalServer("PAYMENT_COUPON_TRANSACTION_REQUIRED", "coupon consumption requires the host database")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin coupon consumption: %w", err)
	}
	defer tx.Rollback()
	if err := s.consumePaymentCoupon(dbent.NewTxContext(ctx, tx), orderID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit coupon consumption: %w", err)
	}
	return nil
}

// withPaymentCouponTransition keeps the native path transaction-free when no
// marketing service is wired. External provider calls and notifications belong
// outside this boundary. The callback uses the supplied client, never the root.
func (s *PaymentService) withPaymentCouponTransition(ctx context.Context, change func(context.Context, *dbent.Client) error) error {
	if s.couponService == nil {
		return change(ctx, s.entClient)
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return change(ctx, tx.Client())
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin payment coupon transition: %w", err)
	}
	defer tx.Rollback()
	if err := change(dbent.NewTxContext(ctx, tx), tx.Client()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit payment coupon transition: %w", err)
	}
	return nil
}

func (s *PaymentService) reconcilePaidOrderCoupon(ctx context.Context, o *dbent.PaymentOrder) error {
	if s.couponService == nil {
		return nil
	}
	return s.withPaymentCouponTransition(ctx, func(txCtx context.Context, _ *dbent.Client) error {
		return s.couponService.ReconcilePaidOrderCoupon(txCtx, o.ID, o.UserID)
	})
}

// Completed orders may predate atomic coupon consumption. Repair only coupon
// bookkeeping, never re-credit the wallet or re-issue subscription rights.
func (s *PaymentService) repairCompletedOrderCoupon(ctx context.Context, o *dbent.PaymentOrder) error {
	if s.couponService == nil {
		return nil
	}
	return s.withPaymentCouponTransition(ctx, func(txCtx context.Context, _ *dbent.Client) error {
		if err := s.couponService.ReconcilePaidOrderCoupon(txCtx, o.ID, o.UserID); err != nil {
			return err
		}
		return s.consumePaymentCoupon(txCtx, o.ID)
	})
}
