//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

// Refund finalization is historical business, not a new coupon admission. Even
// attempting to read admission here is a regression, including when unavailable.
type refundAdmissionMustNotRun struct{ t *testing.T }

func (a refundAdmissionMustNotRun) RequireNewBusiness(context.Context) error {
	a.t.Fatal("historical refund must not read marketing new-business admission")
	return fmt.Errorf("marketing admission unavailable")
}

func seedRefundCouponHistory(t *testing.T, ctx context.Context, client *dbent.Client, order *dbent.PaymentOrder) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "138_marketing_lottery_coupon.sql"))
	require.NoError(t, err)
	// Use the real three coupon table definitions, with only SQLite dialect
	// substitutions. This does not modify/run the production migration or claim PG QA.
	schema, _, found := strings.Cut(string(raw), "CREATE TABLE IF NOT EXISTS lottery_activities")
	require.True(t, found)
	schema = strings.ReplaceAll(schema, "BIGSERIAL", "INTEGER")
	schema = strings.ReplaceAll(schema, "DEFAULT NOW()", "DEFAULT CURRENT_TIMESTAMP")
	_, err = client.ExecContext(ctx, schema)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `INSERT INTO coupon_templates
 (id,name,scope,discount_amount,threshold_amount,status) VALUES (7,'Historical coupon','universal',20,80,'active')`)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `INSERT INTO user_coupons
 (id,template_id,user_id,coupon_code,source_type,scope,discount_amount,threshold_amount,status,used_order_id,used_at)
 VALUES (9,7,$1,'REFUND-HISTORY','lottery','universal',20,80,'used',$2,'2026-09-25 01:02:03')`, order.UserID, order.ID)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `INSERT INTO payment_order_discounts
 (id,order_id,user_coupon_id,coupon_template_id,coupon_code,scope,discount_amount,threshold_amount,original_amount,discounted_amount,status,used_at)
 VALUES (11,$1,9,7,'REFUND-HISTORY','universal',20,80,80,60,'used','2026-09-25 01:02:03')`, order.ID)
	require.NoError(t, err)
}

func refundCouponHistorySnapshot(t *testing.T, ctx context.Context, client *dbent.Client) string {
	t.Helper()
	result := map[string][][]any{}
	for _, table := range []string{"coupon_templates", "user_coupons", "payment_order_discounts"} {
		rows, err := client.QueryContext(ctx, "SELECT * FROM "+table+" ORDER BY id")
		require.NoError(t, err)
		columns, err := rows.Columns()
		require.NoError(t, err)
		for rows.Next() {
			row := make([]any, len(columns))
			refs := make([]any, len(columns))
			for i := range row {
				refs[i] = &row[i]
			}
			require.NoError(t, rows.Scan(refs...))
			result[table] = append(result[table], row)
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		require.Len(t, result[table], 1)
	}
	raw, err := json.Marshal(result)
	require.NoError(t, err)
	return string(raw)
}

func TestMarketingCouponRefundPreservesConsumedHistory(t *testing.T) {
	for _, orderType := range []string{payment.OrderTypeBalance, payment.OrderTypeSubscription} {
		for _, amount := range []float64{80, 40} {
			for _, failAudit := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/refund=%v/auditFailure=%v", orderType, amount, failAudit), func(t *testing.T) {
					ctx := context.Background()
					client := newPaymentConfigServiceTestClient(t)
					order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusRefunding, time.Now())
					var err error
					order, err = client.PaymentOrder.UpdateOneID(order.ID).SetOrderType(orderType).SetPayAmount(60).SetCompletedAt(time.Now().Add(-time.Minute)).Save(ctx)
					require.NoError(t, err)
					seedRefundCouponHistory(t, ctx, client, order)
					before := refundCouponHistorySnapshot(t, ctx, client)
					svc := &PaymentService{entClient: client, couponService: marketing.NewCouponServiceWithAdmission(nil, nil, nil, refundAdmissionMustNotRun{t})}
					plan := &RefundPlan{OrderID: order.ID, Order: order, RefundAmount: amount, GatewayAmount: amount * 60 / 80, Reason: "refund historical coupon order"}
					finalize := func() (*RefundResult, error) {
						return svc.finishRefund(ctx, plan, &payment.RefundResponse{Status: payment.ProviderStatusSuccess})
					}
					if failAudit {
						_, err = client.ExecContext(ctx, `CREATE TRIGGER reject_refund_audit BEFORE INSERT ON payment_audit_logs
       WHEN NEW.action = 'REFUND_SUCCESS' BEGIN SELECT RAISE(ABORT, 'injected refund audit failure'); END`)
						require.NoError(t, err)
						_, err = finalize()
						require.ErrorContains(t, err, "injected refund audit failure")
						current, err := client.PaymentOrder.Get(ctx, order.ID)
						require.NoError(t, err)
						require.Equal(t, OrderStatusRefunding, current.Status)
						require.Nil(t, current.RefundAt)
						require.Equal(t, before, refundCouponHistorySnapshot(t, ctx, client))
						_, err = client.ExecContext(ctx, "DROP TRIGGER reject_refund_audit")
						require.NoError(t, err)
					}
					result, err := finalize()
					require.NoError(t, err)
					require.True(t, result.Success)
					current, err := client.PaymentOrder.Get(ctx, order.ID)
					require.NoError(t, err)
					expected := OrderStatusRefunded
					if amount < 80 {
						expected = OrderStatusPartiallyRefunded
					}
					require.Equal(t, expected, current.Status)
					require.NotNil(t, current.RefundAt)
					require.Equal(t, 80.0, current.Amount)
					require.Equal(t, 60.0, current.PayAmount)
					require.Equal(t, before, refundCouponHistorySnapshot(t, ctx, client), "no automatic return, repricing, or template mutation")
					_, err = finalize()
					require.Error(t, err, "a repeated finalization must not issue new rights")
					require.Equal(t, before, refundCouponHistorySnapshot(t, ctx, client))
					count, err := client.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(order.ID, 10)), paymentauditlog.ActionEQ("REFUND_SUCCESS")).Count(ctx)
					require.NoError(t, err)
					require.Equal(t, 1, count)
				})
			}
		}
	}
}
