//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func validCampaign() RechargeCampaign {
	return RechargeCampaign{Name: "秋日充值礼", Enabled: true, Kind: "bonus", Percent: 10, StartsAt: time.Now().Add(-time.Hour), EndsAt: time.Now().Add(time.Hour), RewardCap: 10, FreezeHours: 72, NewInviteesOnly: true}
}

func TestRechargeCampaignSelectionGuardsAndSnapshot(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{configService: &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{customize.Key(customize.RechargeCampaigns): "true"}}}, entClient: client}
	_, err := client.ExecContext(ctx, `CREATE TABLE recharge_campaigns(id INTEGER PRIMARY KEY,config TEXT NOT NULL,created_at TEXT DEFAULT CURRENT_TIMESTAMP,updated_at TEXT DEFAULT CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	a := validCampaign()
	created, err := svc.rechargeCampaignCatalog().Save(ctx, a)
	require.NoError(t, err)
	cfg := &PaymentConfig{BalanceRechargeMultiplier: 1}
	req := CreateOrderRequest{CampaignID: created.ID, CampaignRevision: created.Revision, OrderType: payment.OrderTypeBalance, Amount: 50}
	snap, err := svc.prepareRechargeCampaign(ctx, req, cfg)
	require.NoError(t, err)
	require.Equal(t, 55.0, snap.Credited)
	autoReq := req
	autoReq.CampaignID = 0
	autoReq.CampaignRevision = ""
	autoSnap, err := svc.prepareRechargeCampaign(ctx, autoReq, cfg)
	require.NoError(t, err)
	require.Equal(t, created.ID, autoSnap.Campaign.ID)
	require.Equal(t, 55.0, autoSnap.Credited)
	req.UserCouponID = 10
	_, err = svc.prepareRechargeCampaign(ctx, req, cfg)
	require.Error(t, err)
	req.UserCouponID = 0
	req.OrderType = payment.OrderTypeSubscription
	_, err = svc.prepareRechargeCampaign(ctx, req, cfg)
	require.Error(t, err)
	req.OrderType = payment.OrderTypeBalance
	req.CampaignID = 999
	_, err = svc.prepareRechargeCampaign(ctx, req, cfg)
	require.Error(t, err)
	created.MinAmount = 100
	raw := *created
	raw.ID = 0
	other, err := svc.rechargeCampaignCatalog().Save(ctx, raw)
	require.NoError(t, err)
	req.CampaignID = other.ID
	req.CampaignRevision = other.Revision
	_, err = svc.prepareRechargeCampaign(ctx, req, cfg)
	require.Error(t, err)
	require.Equal(t, 10.0, snap.Campaign.Percent)
}

func TestRechargeCampaignRewardRetriesUseImmutableSnapshot(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	snap := &RechargeCampaignSnapshot{Campaign: validCampaign(), Principal: 50, Credited: 55, InviterID: 99, Reward: 2.5}
	order, err := client.PaymentOrder.UpdateOneID(order.ID).SetOrderType(payment.OrderTypeBalance).SetProviderSnapshot(map[string]any{"recharge_campaign": snap}).Save(ctx)
	require.NoError(t, err)
	repo := &paymentFulfillmentAffiliateRepoStub{}
	svc := &PaymentService{entClient: client, affiliateService: &AffiliateService{repo: repo}}
	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.Len(t, repo.accrueCalls, 1)
	require.Equal(t, 2.5, repo.accrueCalls[0].amount)
	require.Equal(t, 72, repo.accrueCalls[0].freezeHours)
	require.Equal(t, int64(99), repo.accrueCalls[0].inviterID)
}
