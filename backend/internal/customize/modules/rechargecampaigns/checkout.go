package rechargecampaigns

import (
	"context"
	"math"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
	"time"
)

// QuoteInput contains the native balance quote, not the upstream payment DTO.
// The host owns order-type/activation checks, multiplier normalization, fees and
// currency conversion. NativeCredited avoids copying its baseline price policy.
type QuoteInput struct {
	UserID           int64
	Amount           float64
	Multiplier       float64
	NativeCredited   float64
	UserCouponID     int64
	CampaignID       int64
	CampaignRevision string
}

// InviterLookup is read-only. No reservation or reward is written while quoting.
type InviterLookup interface {
	EligibleInviter(context.Context, int64, Campaign) (int64, error)
}

func campaignAmounts(a Campaign, input QuoteInput) (principal, credited float64) {
	base := decimal.NewFromFloat(input.Amount)
	principal, credited = input.Amount, input.NativeCredited
	if a.Kind == "bonus" {
		credited = base.Mul(decimal.NewFromFloat(input.Multiplier)).Mul(decimal.NewFromInt(1).Add(decimal.NewFromFloat(a.Percent).Div(decimal.NewFromInt(100)))).Round(2).InexactFloat64()
	}
	if a.Kind == "discount" {
		principal = base.Mul(decimal.NewFromFloat(a.Percent)).Div(decimal.NewFromInt(100)).Round(2).InexactFloat64()
	}
	return
}

func automaticCampaign(items []Campaign, amount float64, now time.Time) *Campaign {
	var best *Campaign
	benefit := func(a Campaign) decimal.Decimal {
		if a.Kind == "bonus" {
			return decimal.NewFromInt(1).Add(decimal.NewFromFloat(a.Percent).Div(decimal.NewFromInt(100)))
		}
		return decimal.NewFromInt(100).Div(decimal.NewFromFloat(a.Percent))
	}
	for i := range items {
		a := &items[i]
		if amount <= 0 || !a.Active(now) || amount < a.MinAmount {
			continue
		}
		if best == nil || benefit(*a).GreaterThan(benefit(*best)) || (benefit(*a).Equal(benefit(*best)) && a.ID > best.ID) {
			best = a
		}
	}
	return best
}

// Quote is called only for enabled NEW balance orders. Historical fulfillment
// must read the saved Snapshot instead, even after flags or catalog rules change.
func (s *Service) Quote(ctx context.Context, input QuoteInput, inviters InviterLookup) (*Snapshot, error) {
	items, err := s.List(ctx, false)
	if err != nil {
		return nil, err
	}
	selected := automaticCampaign(items, input.Amount, s.now())
	if selected == nil {
		if input.CampaignID > 0 {
			return nil, infraerrors.Conflict("CAMPAIGN_CHANGED", "活动已结束或不满足门槛，请刷新页面确认金额后重试")
		}
		return nil, nil
	}
	if input.UserCouponID > 0 {
		return nil, infraerrors.BadRequest("CAMPAIGN_CONFLICT", "当前充值已自动享受活动，不能与优惠券叠加")
	}
	a := *selected
	if input.CampaignID > 0 && (input.CampaignID != a.ID || input.CampaignRevision != a.Revision) {
		return nil, infraerrors.Conflict("CAMPAIGN_CHANGED", "活动规则已更新，请刷新页面确认金额后重试")
	}
	if !a.Active(s.now()) || input.Amount < a.MinAmount {
		return nil, infraerrors.BadRequest("CAMPAIGN_UNAVAILABLE", "活动未开始、已结束或未达到充值门槛，请刷新活动")
	}
	principal, credited := campaignAmounts(a, input)
	if principal <= 0 {
		return nil, infraerrors.BadRequest("INVALID_AMOUNT", "折扣后金额过小")
	}
	snap := &Snapshot{Campaign: a, Principal: principal, Credited: credited}
	if a.RewardPercent > 0 && s.affiliateEnabled != nil && s.affiliateEnabled(ctx) {
		if inviters == nil {
			return nil, infraerrors.ServiceUnavailable("CAMPAIGN_INVITER_UNAVAILABLE", "活动邀请关系查询不可用")
		}
		snap.InviterID, err = inviters.EligibleInviter(ctx, input.UserID, a)
		if err != nil {
			return nil, err
		}
		if snap.InviterID > 0 {
			// Preserve historical reward math/precision; changing this policy needs
			// a versioned snapshot, not a silent change during extraction.
			snap.Reward = decimal.NewFromFloat(math.Min(principal*input.Multiplier*a.RewardPercent/100, a.RewardCap)).Round(8).InexactFloat64()
		}
	}
	return snap, nil
}
