//go:build unit

package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func validCampaign() RechargeCampaign {
	return RechargeCampaign{Name: "秋日充值礼", Enabled: true, Kind: "bonus", Percent: 10, StartsAt: time.Now().Add(-time.Hour), EndsAt: time.Now().Add(time.Hour), RewardCap: 10, FreezeHours: 72, NewInviteesOnly: true}
}

func TestRechargeCampaignAmounts(t *testing.T) {
	for _, tc := range []struct {
		kind                                             string
		percent, amount, multiplier, principal, credited float64
	}{
		{"bonus", 10, 50, 1, 50, 55}, {"bonus", 10, 100, 1, 100, 110}, {"discount", 90, 100, 1, 90, 100},
		{"bonus", 10, 50, 2, 50, 110}, {"discount", 90, 100, 2, 90, 200}, {"discount", 90, 0.05, 1, 0.05, 0.05},
	} {
		a := validCampaign()
		a.Kind = tc.kind
		a.Percent = tc.percent
		p, c := campaignAmounts(a, tc.amount, tc.multiplier)
		require.Equal(t, tc.principal, p)
		require.Equal(t, tc.credited, c)
	}
}
func TestRechargeCampaignValidationAndTimeBoundaries(t *testing.T) {
	a := validCampaign()
	require.NoError(t, a.validate())
	require.True(t, a.active(a.StartsAt))
	require.False(t, a.active(a.EndsAt))
	require.False(t, a.active(a.StartsAt.Add(-time.Nanosecond)))
	for _, change := range []func(*RechargeCampaign){
		func(a *RechargeCampaign) { a.Percent = math.NaN() }, func(a *RechargeCampaign) { a.Percent = math.Inf(1) },
		func(a *RechargeCampaign) { a.Kind = "invalid" }, func(a *RechargeCampaign) { a.Kind = "discount"; a.Percent = 100 },
		func(a *RechargeCampaign) { a.RewardPercent = 5; a.RewardCap = 0 }, func(a *RechargeCampaign) { a.EndsAt = a.StartsAt },
		func(a *RechargeCampaign) { a.FreezeHours = -1 }, func(a *RechargeCampaign) { a.MinAmount = -1 },
	} {
		b := a
		change(&b)
		require.Error(t, b.validate())
	}
}
func TestRechargeCampaignSelectionGuardsAndSnapshot(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := &PaymentService{entClient: client}
	_, err := client.ExecContext(ctx, `CREATE TABLE recharge_campaigns(id INTEGER PRIMARY KEY,config TEXT NOT NULL,created_at TEXT DEFAULT CURRENT_TIMESTAMP,updated_at TEXT DEFAULT CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	a := validCampaign()
	created, err := svc.SaveRechargeCampaign(ctx, a)
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
	other, err := svc.SaveRechargeCampaign(ctx, raw)
	require.NoError(t, err)
	req.CampaignID = other.ID
	req.CampaignRevision = other.Revision
	_, err = svc.prepareRechargeCampaign(ctx, req, cfg)
	require.Error(t, err)
	require.Equal(t, 10.0, snap.Campaign.Percent)
}

func TestRechargeCampaignAutomaticSelection(t *testing.T) {
	now := time.Now()
	bonus := validCampaign()
	bonus.ID = 1
	discount := bonus
	discount.ID, discount.Kind, discount.Percent, discount.MinAmount = 2, "discount", 90, 100
	require.Equal(t, int64(1), automaticRechargeCampaign([]RechargeCampaign{discount, bonus}, 50, now).ID)
	require.Equal(t, int64(2), automaticRechargeCampaign([]RechargeCampaign{bonus, discount}, 100, now).ID)
	newest := bonus
	newest.ID = 3
	require.Equal(t, int64(3), automaticRechargeCampaign([]RechargeCampaign{newest, bonus}, 50, now).ID)
	require.Nil(t, automaticRechargeCampaign([]RechargeCampaign{bonus}, 50, bonus.EndsAt))
	require.Nil(t, automaticRechargeCampaign([]RechargeCampaign{discount}, 50, now))
	bonus.Enabled = false
	require.Nil(t, automaticRechargeCampaign([]RechargeCampaign{bonus}, 50, now))
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
