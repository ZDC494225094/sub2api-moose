//go:build unit

package service

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/Wei-Shaw/sub2api/migrations"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// Opt-in, isolated PostgreSQL test. Never point this at a business database.
func TestRechargeCampaignPostgres(t *testing.T) {
	dsn := os.Getenv("CAMPAIGN_TEST_DSN")
	if dsn == "" {
		t.Skip("set CAMPAIGN_TEST_DSN to an isolated campaign_qa database")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	require.Equal(t, "/campaign_qa", u.Path)
	ctx := context.Background()
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	schema := fmt.Sprintf("campaign_test_%d", time.Now().UnixNano())
	_, err = db.ExecContext(ctx, "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	defer db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE")
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	isolated, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, isolated)))
	defer client.Close()
	require.NoError(t, client.Schema.Create(ctx))
	for _, name := range []string{"130_add_user_affiliates.sql", "131_affiliate_rebate_hardening.sql", "132_affiliate_custom_settings.sql", "133_affiliate_rebate_freeze.sql", "134_affiliate_ledger_audit_snapshots.sql", "240_recharge_campaigns.sql", "240_recharge_campaigns.sql"} {
		raw, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		_, err = client.ExecContext(ctx, string(raw))
		require.NoError(t, err, name)
	}
	inviter, err := client.User.Create().SetEmail("inviter@campaign.test").SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	buyer, err := client.User.Create().SetEmail("buyer@campaign.test").SetPasswordHash("test").Save(ctx)
	require.NoError(t, err)
	_, err = client.ExecContext(ctx, `INSERT INTO user_affiliates(user_id,aff_code,inviter_id,invited_at) VALUES($1,'INVITER',NULL,NULL),($2,'BUYER',$1,NOW())`, inviter.ID, buyer.ID)
	require.NoError(t, err)
	settings := NewSettingService(&paymentFulfillmentSettingRepoStub{values: map[string]string{SettingKeyAffiliateEnabled: "true"}}, nil)
	svc := &PaymentService{entClient: client, affiliateService: NewAffiliateService(&paymentFulfillmentAffiliateRepoStub{}, settings, nil, nil)}
	a := validCampaign()
	a.RewardPercent = 5
	a.RewardCap = 3
	created, err := svc.SaveRechargeCampaign(ctx, a)
	require.NoError(t, err)
	req := CreateOrderRequest{UserID: buyer.ID, CampaignID: created.ID, CampaignRevision: created.Revision, OrderType: payment.OrderTypeBalance, Amount: 100, PaymentType: payment.TypeAlipay}
	cfg := &PaymentConfig{BalanceRechargeMultiplier: 1}
	snap, err := svc.prepareRechargeCampaign(ctx, req, cfg)
	require.NoError(t, err)
	require.Equal(t, 3.0, snap.Reward)
	require.Equal(t, inviter.ID, snap.InviterID)
	require.Equal(t, 110.0, snap.Credited)
	// Unknown historical binding timestamps cannot earn "new invitee" rewards.
	_, err = client.ExecContext(ctx, "UPDATE user_affiliates SET invited_at=NULL WHERE user_id=$1", buyer.ID)
	require.NoError(t, err)
	noReward, err := svc.prepareRechargeCampaign(ctx, req, cfg)
	require.NoError(t, err)
	require.Zero(t, noReward.Reward)
	_, err = client.ExecContext(ctx, "UPDATE user_affiliates SET invited_at=NOW() WHERE user_id=$1", buyer.ID)
	require.NoError(t, err)
	req.campaign = snap
	order, err := svc.createOrderInTx(ctx, req, &User{ID: buyer.ID, Email: buyer.Email}, nil, cfg, snap.Credited, snap.Principal, 0, 100, nil)
	require.NoError(t, err)
	loaded, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	saved, err := campaignSnapshot(loaded)
	require.NoError(t, err)
	require.Equal(t, 3.0, saved.Reward)
	// Editing a campaign invalidates stale quotes but never modifies an existing order snapshot.
	created.Percent = 20
	_, err = svc.SaveRechargeCampaign(ctx, *created)
	require.NoError(t, err)
	_, err = svc.prepareRechargeCampaign(ctx, req, cfg)
	require.Error(t, err)
	require.Equal(t, 10.0, saved.Campaign.Percent)
	for _, frozen := range []bool{true, false} {
		t.Run(fmt.Sprintf("refund_frozen_%v", frozen), func(t *testing.T) {
			o := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusRefunding, time.Now())
			o, err = client.PaymentOrder.UpdateOneID(o.ID).SetOrderType(payment.OrderTypeBalance).SetAmount(110).SetPayAmount(100).SetProviderSnapshot(map[string]any{"recharge_campaign": snap}).Save(ctx)
			require.NoError(t, err)
			var until any
			available := 3.0
			frozenQuota := 0.0
			if frozen {
				until = time.Now().Add(time.Hour)
				available = 0
				frozenQuota = 3
			}
			_, err = client.ExecContext(ctx, `UPDATE user_affiliates SET aff_quota=$1,aff_frozen_quota=$2,aff_history_quota=3 WHERE user_id=$3`, available, frozenQuota, inviter.ID)
			require.NoError(t, err)
			_, err = client.ExecContext(ctx, `INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,source_order_id,frozen_until) VALUES($1,'accrue',3,$2,$3,$4)`, inviter.ID, o.UserID, o.ID, until)
			require.NoError(t, err)
			p := &RefundPlan{Order: o, OrderID: o.ID, RefundAmount: 55, Reason: "half refund"}
			result, err := svc.markRefundOk(ctx, p)
			require.NoError(t, err)
			require.True(t, result.Success)
			_, err = svc.markRefundOk(ctx, p)
			require.Error(t, err, "a duplicate refund must not claw back twice")
			var quota, total, ledger float64
			require.NoError(t, isolated.QueryRowContext(ctx, "SELECT aff_quota+aff_frozen_quota,aff_history_quota FROM user_affiliates WHERE user_id=$1", inviter.ID).Scan(&quota, &total))
			require.InDelta(t, 1.5, quota, 1e-8)
			require.InDelta(t, 1.5, total, 1e-8)
			require.NoError(t, isolated.QueryRowContext(ctx, "SELECT SUM(amount) FROM user_affiliate_ledger WHERE source_order_id=$1", o.ID).Scan(&ledger))
			require.InDelta(t, 1.5, ledger, 1e-8)
			var pending float64
			require.NoError(t, isolated.QueryRowContext(ctx, "SELECT COALESCE(SUM(amount),0) FROM user_affiliate_ledger WHERE source_order_id=$1 AND frozen_until IS NOT NULL", o.ID).Scan(&pending))
			if frozen {
				require.InDelta(t, 1.5, pending, 1e-8)
			} else {
				require.Zero(t, pending)
			}
		})
	}
}
