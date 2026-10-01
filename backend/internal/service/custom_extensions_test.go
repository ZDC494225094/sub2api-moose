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

func TestDisabledCampaignBypassesAutomaticBenefitsButRejectsStaleSelection(t *testing.T) {
	repo := &paymentConfigSettingRepoStub{values: map[string]string{customize.Key(customize.RechargeCampaigns): "false"}}
	svc := &PaymentService{configService: &PaymentConfigService{settingRepo: repo}}
	req := CreateOrderRequest{OrderType: payment.OrderTypeBalance, Amount: 50}
	// No campaign database/client: proves we return before selecting/applying benefits.
	snap, err := svc.prepareRechargeCampaign(context.Background(), req, &PaymentConfig{BalanceRechargeMultiplier: 1})
	require.NoError(t, err)
	require.Nil(t, snap)
	req.CampaignID = 1
	_, err = svc.prepareRechargeCampaign(context.Background(), req, &PaymentConfig{})
	require.ErrorContains(t, err, "disabled")
}

func TestCampaignStateFailureCannotSilentlyChangePrice(t *testing.T) {
	svc := &PaymentService{configService: &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{customize.Key(customize.RechargeCampaigns): "broken"}}}}
	_, err := svc.prepareRechargeCampaign(context.Background(), CreateOrderRequest{OrderType: payment.OrderTypeBalance, Amount: 50}, &PaymentConfig{})
	require.Error(t, err)
	_, err = (&PaymentService{}).rechargeCampaignExtensionEnabled(context.Background())
	require.Error(t, err)
}

func TestDisabledCampaignStillFulfillsExistingSnapshot(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	ensurePaymentAuditOrderActionUniqueIndex(t, ctx, client)
	order := createPaymentFulfillmentSubscriptionOrder(t, ctx, client, OrderStatusPaid, time.Now())
	snap := &RechargeCampaignSnapshot{Campaign: validCampaign(), Principal: 50, Credited: 55, InviterID: 99, Reward: 2.5}
	order, err := client.PaymentOrder.UpdateOneID(order.ID).SetOrderType(payment.OrderTypeBalance).SetProviderSnapshot(map[string]any{"recharge_campaign": snap}).Save(ctx)
	require.NoError(t, err)
	repo := &paymentFulfillmentAffiliateRepoStub{}
	svc := &PaymentService{entClient: client, affiliateService: &AffiliateService{repo: repo}, configService: &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{values: map[string]string{customize.Key(customize.RechargeCampaigns): "false"}}}}
	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.NoError(t, svc.applyAffiliateRebateForOrder(ctx, order))
	require.Len(t, repo.accrueCalls, 1)
	require.Equal(t, 2.5, repo.accrueCalls[0].amount)
	require.Equal(t, 55.0, PaymentOrderRechargeCampaign(order).Credited)
}

func TestUnconfiguredCampaignDefaultsToNativeCheckout(t *testing.T) {
	svc := &PaymentService{configService: &PaymentConfigService{settingRepo: &paymentConfigSettingRepoStub{}}}
	req := CreateOrderRequest{OrderType: payment.OrderTypeBalance, Amount: 50}
	snap, err := svc.prepareRechargeCampaign(context.Background(), req, &PaymentConfig{BalanceRechargeMultiplier: 1})
	require.NoError(t, err)
	require.Nil(t, snap, "no explicit flag must not activate campaign benefits")
	req.CampaignID = 1
	_, err = svc.prepareRechargeCampaign(context.Background(), req, &PaymentConfig{})
	require.ErrorContains(t, err, "disabled")
}
