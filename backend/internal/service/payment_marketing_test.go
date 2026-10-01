package service

import (
	"context"
	"errors"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

type orderCouponProbe struct {
	UserCouponRepository
	t           *testing.T
	coupon      UserCoupon
	unavailable bool
}

func (r *orderCouponProbe) GetByID(ctx context.Context, id int64) (*UserCoupon, error) {
	require.NotNil(r.t, dbent.TxFromContext(ctx))
	require.Equal(r.t, r.coupon.ID, id)
	copy := r.coupon
	return &copy, nil
}
func (r *orderCouponProbe) ReserveForOrder(ctx context.Context, couponID, orderID int64, at time.Time) (bool, error) {
	tx := dbent.TxFromContext(ctx)
	require.NotNil(r.t, tx)
	// The order exists in this very transaction, not in an earlier committed one.
	_, err := tx.Client().PaymentOrder.Get(ctx, orderID)
	require.NoError(r.t, err)
	if r.unavailable {
		return false, nil
	}
	_, err = tx.Client().ExecContext(ctx, "INSERT INTO marketing_order_probe(kind, order_id) VALUES ('coupon', ?)", orderID)
	return err == nil, err
}
func (r *orderCouponProbe) ReleaseReservationByOrderID(ctx context.Context, orderID int64, at time.Time) error {
	tx := dbent.TxFromContext(ctx)
	require.NotNil(r.t, tx)
	_, err := tx.Client().ExecContext(ctx, "DELETE FROM marketing_order_probe WHERE kind='coupon' AND order_id=?", orderID)
	return err
}

type orderDiscountProbe struct {
	PaymentOrderDiscountRepository
	t    *testing.T
	fail bool
}

func (r *orderDiscountProbe) Create(ctx context.Context, discount *PaymentOrderDiscount) error {
	tx := dbent.TxFromContext(ctx)
	require.NotNil(r.t, tx)
	if r.fail {
		return errors.New("discount snapshot insert failed")
	}
	_, err := tx.Client().ExecContext(ctx, "INSERT INTO marketing_order_probe(kind, order_id) VALUES ('discount', ?)", discount.OrderID)
	return err
}

// Real SQLite host transactions prove rollback of the saved order and both
// extension writes. PostgreSQL repository SQL and connection selection are
// separately exercised in repository/marketing_tx_test.go.
func TestMarketingOrderAndCouponReservationCommitTogether(t *testing.T) {
	for _, scenario := range []string{"commit", "unavailable", "discount_failure", "quote_changed", "quote_missing", "service_missing", "native"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			_, err := client.ExecContext(ctx, "CREATE TABLE marketing_order_probe(kind TEXT PRIMARY KEY, order_id INTEGER NOT NULL)")
			require.NoError(t, err)
			user, err := client.User.Create().SetEmail("coupon-order@example.com").SetPasswordHash("hash").Save(ctx)
			require.NoError(t, err)
			couponRepo := &orderCouponProbe{t: t, coupon: UserCoupon{ID: 7, TemplateID: 3, UserID: user.ID, CouponCode: "test", Scope: CouponScopeBalance, DiscountAmount: 5, Status: UserCouponStatusUnused}, unavailable: scenario == "unavailable"}
			discounts := &orderDiscountProbe{t: t, fail: scenario == "discount_failure"}
			svc := &PaymentService{entClient: client, couponService: newMarketingTestCouponService(nil, couponRepo, discounts)}
			req := CreateOrderRequest{UserID: user.ID, OrderType: payment.OrderTypeBalance, PaymentType: payment.TypeAlipay, Amount: 50, UserCouponID: 7, couponQuote: &ApplyPaymentCouponResult{OriginalAmount: 50, DiscountAmount: 5, DiscountedAmount: 45}}
			if scenario == "quote_changed" {
				couponRepo.coupon.DiscountAmount = 6
			}
			if scenario == "quote_missing" {
				req.couponQuote = nil
			}
			if scenario == "service_missing" {
				svc.couponService = nil
			}
			payAmount := float64(45)
			if scenario == "native" {
				req.UserCouponID = 0
				req.couponQuote = nil
				svc.couponService = nil
				payAmount = 50
			}
			order, err := svc.createOrderInTx(ctx, req, &User{ID: user.ID, Email: user.Email}, nil, &PaymentConfig{OrderTimeoutMin: 30}, 50, payAmount, 0, payAmount, nil)
			success := scenario == "commit" || scenario == "native"
			if success {
				require.NoError(t, err)
				require.NotNil(t, order)
				require.Equal(t, payAmount, order.PayAmount)
			} else {
				require.Error(t, err)
				require.Nil(t, order)
			}
			count, err := client.PaymentOrder.Query().Count(ctx)
			require.NoError(t, err)
			if success {
				require.Equal(t, 1, count)
			} else {
				require.Zero(t, count)
			}
			rows, err := client.QueryContext(ctx, "SELECT COUNT(*) FROM marketing_order_probe")
			require.NoError(t, err)
			defer rows.Close()
			require.True(t, rows.Next())
			var writes int
			require.NoError(t, rows.Scan(&writes))
			require.NoError(t, rows.Err())
			if scenario == "commit" {
				require.Equal(t, 2, writes)
			} else {
				require.Zero(t, writes)
			}
		})
	}
}

func TestMarketingOrderReservationRequiresHostTransaction(t *testing.T) {
	svc := &PaymentService{couponService: newMarketingTestCouponService(nil, nil, nil)}
	err := svc.reserveOrderCoupon(context.Background(), CreateOrderRequest{UserCouponID: 7}, 11)
	require.ErrorContains(t, err, "requires the order transaction")
	// Ordinary upstream orders never depend on an extension or its transaction.
	require.NoError(t, (&PaymentService{}).reserveOrderCoupon(context.Background(), CreateOrderRequest{}, 11))
}

// These probes require the real transaction client; the second write can fail
// after the first succeeds, proving that a retry cannot leave a half-used coupon.
type consumeCouponProbe struct {
	UserCouponRepository
	t    *testing.T
	fail bool
}

func (r *consumeCouponProbe) MarkUsedByOrderID(ctx context.Context, id int64, at time.Time) error {
	tx := dbent.TxFromContext(ctx)
	require.NotNil(r.t, tx)
	if r.fail {
		return errors.New("coupon consume failed")
	}
	_, err := tx.Client().ExecContext(ctx, "INSERT OR IGNORE INTO marketing_order_probe(kind, order_id) VALUES ('coupon', ?)", id)
	return err
}

type consumeDiscountProbe struct {
	PaymentOrderDiscountRepository
	t *testing.T
}

func (r *consumeDiscountProbe) MarkUsedByOrderID(ctx context.Context, id int64, at time.Time) error {
	tx := dbent.TxFromContext(ctx)
	require.NotNil(r.t, tx)
	_, err := tx.Client().ExecContext(ctx, "INSERT OR IGNORE INTO marketing_order_probe(kind, order_id) VALUES ('discount', ?)", id)
	return err
}
func TestMarketingConsumptionTransactionAndRetry(t *testing.T) {
	for _, mode := range []string{"commit", "rollback", "caller_rollback"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			_, err := client.ExecContext(ctx, "CREATE TABLE marketing_order_probe(kind TEXT PRIMARY KEY, order_id INTEGER NOT NULL)")
			require.NoError(t, err)
			coupons := &consumeCouponProbe{t: t, fail: mode == "rollback"}
			svc := &PaymentService{entClient: client, couponService: newMarketingTestCouponService(nil, coupons, &consumeDiscountProbe{t: t})}
			if mode == "caller_rollback" {
				tx, err := client.Tx(ctx)
				require.NoError(t, err)
				require.NoError(t, svc.consumePaymentCoupon(dbent.NewTxContext(ctx, tx), 42))
				require.NoError(t, tx.Rollback())
			} else {
				err = svc.consumePaymentCoupon(ctx, 42)
				if mode == "rollback" {
					require.ErrorContains(t, err, "coupon consume failed")
				} else {
					require.NoError(t, err)
				}
			}
			count := func() int {
				rows, err := client.QueryContext(ctx, "SELECT COUNT(*) FROM marketing_order_probe")
				require.NoError(t, err)
				defer rows.Close()
				require.True(t, rows.Next())
				var n int
				require.NoError(t, rows.Scan(&n))
				require.NoError(t, rows.Err())
				return n
			}
			if mode == "commit" {
				require.Equal(t, 2, count())
			} else {
				require.Zero(t, count())
			}
			coupons.fail = false
			require.NoError(t, svc.consumePaymentCoupon(ctx, 42))
			require.NoError(t, svc.consumePaymentCoupon(ctx, 42))
			require.Equal(t, 2, count())
		})
	}
	require.NoError(t, (&PaymentService{}).consumePaymentCoupon(context.Background(), 42))
	svc := &PaymentService{couponService: newMarketingTestCouponService(nil, nil, nil)}
	require.ErrorContains(t, svc.consumePaymentCoupon(context.Background(), 42), "requires the host database")
}
