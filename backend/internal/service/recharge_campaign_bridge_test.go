//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"regexp"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

// The real PostgreSQL regression remains opt-in. These tests verify the host's
// transaction/status protocol with a mock driver, not database lock semantics.
func TestCampaignRefundBridgeSharesHostTransactionAndRollsBackLedgerFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	db.SetMaxOpenConns(1) // A mistaken root-client SQL call cannot escape the in-use transaction.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer client.Close()
	svc := &PaymentService{entClient: client, configService: &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{customize.Key(customize.RechargeCampaigns): "false"}}}}
	snap := &RechargeCampaignSnapshot{Campaign: validCampaign(), InviterID: 99, Reward: 3}
	order := &dbent.PaymentOrder{ID: 7, UserID: 42, Amount: 110, ProviderSnapshot: map[string]any{"recharge_campaign": snap}}
	p := &RefundPlan{Order: order, OrderID: 7, RefundAmount: 55, Reason: "partial"}
	wantErr := errors.New("ledger write failed")
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "payment_orders"`)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE user_affiliates SET updated_at=updated_at WHERE user_id=$1")).WithArgs(int64(99)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id,user_id,amount,frozen_until FROM user_affiliate_ledger WHERE source_order_id=$1 AND action='accrue' FOR UPDATE")).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "amount", "frozen_until"}).AddRow(1, 99, 3, nil)).RowsWillBeClosed()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE user_affiliates SET aff_quota=aff_quota-$1,aff_history_quota=aff_history_quota-$1,updated_at=NOW() WHERE user_id=$2")).WithArgs(1.5, int64(99)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO user_affiliate_ledger").WithArgs(int64(99), -1.5, int64(42), int64(7), nil).WillReturnError(wantErr)
	mock.ExpectRollback()
	result, err := svc.markRefundOk(ctx, p)
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, result)
	require.NoError(t, mock.ExpectationsWereMet(), "status, quota and ledger must share the host transaction, even when the module is off")
}

func TestCampaignRefundBridgeRejectsDuplicateBeforeTouchingRewardLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	db.SetMaxOpenConns(1) // A mistaken root-client SQL call cannot escape the in-use transaction.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer client.Close()
	svc := &PaymentService{entClient: client}
	order := &dbent.PaymentOrder{ID: 7, UserID: 42, Amount: 110, ProviderSnapshot: map[string]any{"recharge_campaign": &RechargeCampaignSnapshot{Campaign: validCampaign(), InviterID: 99, Reward: 3}}}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE "payment_orders"`)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	result, err := svc.markRefundOk(ctx, &RefundPlan{Order: order, OrderID: 7, RefundAmount: 55})
	require.ErrorContains(t, err, "refund already finalized")
	require.Nil(t, result)
	require.NoError(t, mock.ExpectationsWereMet(), "no reward SQL should run after a failed order-status claim")
}

func TestCampaignQuoteBridgeUsesNativeMultiplierNormalization(t *testing.T) {
	for _, kind := range []string{"bonus", "discount"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			client := newPaymentConfigServiceTestClient(t)
			_, err := client.ExecContext(ctx, "CREATE TABLE recharge_campaigns(id INTEGER PRIMARY KEY,config TEXT NOT NULL,created_at TEXT DEFAULT CURRENT_TIMESTAMP,updated_at TEXT DEFAULT CURRENT_TIMESTAMP)")
			require.NoError(t, err)
			svc := &PaymentService{entClient: client, configService: &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{customize.Key(customize.RechargeCampaigns): "true"}}}}
			a := validCampaign()
			a.Kind = kind
			if kind == "discount" {
				a.Percent = 90
			}
			_, err = svc.rechargeCampaignCatalog().Save(ctx, a)
			require.NoError(t, err)
			for _, multiplier := range []float64{0, -1, math.NaN(), math.Inf(1), 1, 2} {
				snap, err := svc.prepareRechargeCampaign(ctx, CreateOrderRequest{OrderType: payment.OrderTypeBalance, Amount: 100}, &PaymentConfig{BalanceRechargeMultiplier: multiplier})
				require.NoError(t, err)
				if kind == "discount" {
					require.Equal(t, 90.0, snap.Principal)
					require.Equal(t, calculateCreditedBalance(100, multiplier), snap.Credited)
				} else {
					require.Equal(t, 100.0, snap.Principal)
					require.Equal(t, 110*normalizeBalanceRechargeMultiplier(multiplier), snap.Credited)
				}
			}
		})
	}
}
