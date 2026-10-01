package service

import (
	"context"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/rechargecampaigns"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// Compatibility aliases preserve existing handler/order DTOs and persisted JSON.
// Business rules and SQL belong to the module; this file only adapts host ports.
type RechargeCampaign = rechargecampaigns.Campaign
type RechargeCampaignSnapshot = rechargecampaigns.Snapshot

func (s *PaymentService) rechargeCampaignCatalog() *rechargecampaigns.Service {
	return rechargecampaigns.NewService(rechargecampaigns.NewSQLRepository(s.entClient), func(ctx context.Context) bool {
		return s.affiliateService != nil && s.affiliateService.IsEnabled(ctx)
	})
}

func (s *PaymentService) prepareRechargeCampaign(ctx context.Context, req CreateOrderRequest, cfg *PaymentConfig) (*RechargeCampaignSnapshot, error) {
	if req.OrderType != payment.OrderTypeBalance {
		if req.CampaignID > 0 {
			return nil, infraerrors.BadRequest("CAMPAIGN_CONFLICT", "充值活动仅支持余额充值")
		}
		return nil, nil
	}
	if !isValidProviderAmount(req.Amount) {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "amount must be a positive finite number")
	}
	// Gate new quotes only, including requests through the ordinary payment API.
	// Never consult this flag while replaying an existing order snapshot.
	enabled, err := s.rechargeCampaignExtensionEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		if req.CampaignID > 0 {
			return nil, customize.Disabled(customize.RechargeCampaigns)
		}
		return nil, nil
	}
	return s.rechargeCampaignCatalog().Quote(ctx, rechargecampaigns.QuoteInput{
		UserID: req.UserID, Amount: req.Amount,
		Multiplier:       normalizeBalanceRechargeMultiplier(cfg.BalanceRechargeMultiplier),
		NativeCredited:   calculateCreditedBalance(req.Amount, cfg.BalanceRechargeMultiplier),
		UserCouponID:     req.UserCouponID,
		CampaignID:       req.CampaignID,
		CampaignRevision: req.CampaignRevision,
	}, rechargecampaigns.NewSQLInviterLookup(s.entClient))
}

func attachRechargeCampaign(providerSnapshot map[string]any, snap *RechargeCampaignSnapshot, now time.Time) (map[string]any, error) {
	return rechargecampaigns.AttachSnapshot(providerSnapshot, snap, now)
}

func campaignSnapshot(o *dbent.PaymentOrder) (*RechargeCampaignSnapshot, error) {
	if o == nil {
		return nil, nil
	}
	return rechargecampaigns.ReadSnapshot(o.ProviderSnapshot)
}

func (s *PaymentService) accrueCampaignReward(ctx context.Context, snap *RechargeCampaignSnapshot, o *dbent.PaymentOrder) (float64, error) {
	var rewards rechargecampaigns.RewardAccruer
	if s.affiliateService != nil {
		rewards = s.affiliateService.repo
	}
	return rechargecampaigns.AccrueReward(ctx, rewards, snap, o.ID, o.UserID)
}

// The caller already claimed the refund status in this same transaction. Do not
// substitute the root client or introduce a separate transaction in this bridge.
func reverseCampaignReward(ctx context.Context, client *dbent.Client, p *RefundPlan) error {
	snap, err := campaignSnapshot(p.Order)
	if err != nil || snap == nil {
		return err
	}
	return rechargecampaigns.ReverseReward(ctx, client, rechargecampaigns.Refund{
		Snapshot: snap, OrderID: p.OrderID, UserID: p.Order.UserID,
		OrderAmount: p.Order.Amount, RefundAmount: p.RefundAmount,
	})
}

// PaymentOrderRechargeCampaign exposes only campaign rules, not provider credentials.
func PaymentOrderRechargeCampaign(o *dbent.PaymentOrder) *RechargeCampaignSnapshot {
	snap, _ := campaignSnapshot(o)
	return snap
}
