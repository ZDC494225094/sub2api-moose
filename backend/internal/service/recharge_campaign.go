package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// Percent is a gift percentage for bonus, and the payable percentage for discount (90 = 九折).
type RechargeCampaign struct {
	Revision        string    `json:"revision"`
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Enabled         bool      `json:"enabled"`
	StartsAt        time.Time `json:"starts_at"`
	EndsAt          time.Time `json:"ends_at"`
	Kind            string    `json:"kind"`
	Percent         float64   `json:"percent"`
	MinAmount       float64   `json:"min_amount"`
	RewardPercent   float64   `json:"reward_percent"`
	RewardCap       float64   `json:"reward_cap"`
	FreezeHours     int       `json:"freeze_hours"`
	NewInviteesOnly bool      `json:"new_invitees_only"`
}

func rechargeCampaignRevision(a RechargeCampaign) string {
	a.ID = 0
	a.Revision = ""
	raw, _ := json.Marshal(a)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func (a RechargeCampaign) validate() error {
	if strings.TrimSpace(a.Name) == "" || len([]rune(a.Name)) > 60 || len([]rune(a.Description)) > 500 {
		return infraerrors.BadRequest("INVALID_CAMPAIGN", "活动名称须为1–60字，说明不超过500字")
	}
	if a.StartsAt.IsZero() || !a.EndsAt.After(a.StartsAt) {
		return infraerrors.BadRequest("INVALID_CAMPAIGN", "请选择有效的开始和结束时间；限时活动也需设置持续时间")
	}
	for _, v := range []float64{a.Percent, a.MinAmount, a.RewardPercent, a.RewardCap} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return infraerrors.BadRequest("INVALID_CAMPAIGN", "金额和比例必须是有限非负数")
		}
	}
	if (a.Kind != "bonus" && a.Kind != "discount") || a.Percent <= 0 || a.Percent > 100 || (a.Kind == "discount" && a.Percent >= 100) || a.MinAmount > 1000000 || a.RewardPercent > 100 || a.RewardCap > 1000000 || a.FreezeHours < 0 || a.FreezeHours > 8760 {
		return infraerrors.BadRequest("INVALID_CAMPAIGN", "活动比例、奖励或冻结时长超出范围")
	}
	if a.RewardPercent > 0 && a.RewardCap <= 0 {
		return infraerrors.BadRequest("INVALID_CAMPAIGN", "开启邀请奖励时必须设置每单奖励上限")
	}
	return nil
}
func (a RechargeCampaign) active(now time.Time) bool {
	return a.Enabled && !now.Before(a.StartsAt) && now.Before(a.EndsAt)
}

func (s *PaymentService) ListRechargeCampaigns(ctx context.Context, public bool) ([]RechargeCampaign, error) {
	rows, err := s.entClient.QueryContext(ctx, "SELECT id, config FROM recharge_campaigns ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RechargeCampaign{}
	for rows.Next() {
		var id int64
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return nil, err
		}
		var a RechargeCampaign
		if err := json.Unmarshal(raw, &a); err != nil {
			return nil, err
		}
		a.ID = id
		a.Revision = rechargeCampaignRevision(a)
		if !public || (a.Enabled && a.EndsAt.After(time.Now())) {
			items = append(items, a)
		}
	}
	return items, rows.Err()
}
func (s *PaymentService) SaveRechargeCampaign(ctx context.Context, a RechargeCampaign) (*RechargeCampaign, error) {
	if err := a.validate(); err != nil {
		return nil, err
	}
	if a.Enabled && a.RewardPercent > 0 && (s.affiliateService == nil || !s.affiliateService.IsEnabled(ctx)) {
		return nil, infraerrors.BadRequest("AFFILIATE_DISABLED", "请先在系统设置开启邀请返利，再配置活动邀请奖励")
	}
	a.Revision = ""
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	query := "INSERT INTO recharge_campaigns(config) VALUES($1) RETURNING id"
	args := []any{string(raw)}
	if a.ID > 0 {
		query = "UPDATE recharge_campaigns SET config=$1,updated_at=NOW() WHERE id=$2 RETURNING id"
		args = append(args, a.ID)
	}
	rows, err := s.entClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, infraerrors.NotFound("CAMPAIGN_NOT_FOUND", "活动不存在")
	}
	if err := rows.Scan(&a.ID); err != nil {
		return nil, err
	}
	a.Revision = rechargeCampaignRevision(a)
	return &a, rows.Err()
}

type RechargeCampaignSnapshot struct {
	Campaign  RechargeCampaign `json:"campaign"`
	Principal float64          `json:"principal"`
	Credited  float64          `json:"credited"`
	InviterID int64            `json:"inviter_id"`
	Reward    float64          `json:"reward"`
}

func campaignAmounts(a RechargeCampaign, amount, multiplier float64) (principal, credited float64) {
	base := decimal.NewFromFloat(amount)
	principal = amount
	credited = calculateCreditedBalance(amount, multiplier)
	if a.Kind == "bonus" {
		credited = base.Mul(decimal.NewFromFloat(multiplier)).Mul(decimal.NewFromInt(1).Add(decimal.NewFromFloat(a.Percent).Div(decimal.NewFromInt(100)))).Round(2).InexactFloat64()
	}
	if a.Kind == "discount" {
		principal = base.Mul(decimal.NewFromFloat(a.Percent)).Div(decimal.NewFromInt(100)).Round(2).InexactFloat64()
	}
	return
}

func automaticRechargeCampaign(items []RechargeCampaign, amount float64, now time.Time) *RechargeCampaign {
	var best *RechargeCampaign
	benefit := func(a RechargeCampaign) decimal.Decimal {
		if a.Kind == "bonus" {
			return decimal.NewFromInt(1).Add(decimal.NewFromFloat(a.Percent).Div(decimal.NewFromInt(100)))
		}
		return decimal.NewFromInt(100).Div(decimal.NewFromFloat(a.Percent))
	}
	for i := range items {
		a := &items[i]
		if amount <= 0 || !a.active(now) || amount < a.MinAmount {
			continue
		}
		if best == nil || benefit(*a).GreaterThan(benefit(*best)) || (benefit(*a).Equal(benefit(*best)) && a.ID > best.ID) {
			best = a
		}
	}
	return best
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
	items, err := s.ListRechargeCampaigns(ctx, false)
	if err != nil {
		return nil, err
	}
	selected := automaticRechargeCampaign(items, req.Amount, time.Now())
	if selected == nil {
		if req.CampaignID > 0 {
			return nil, infraerrors.Conflict("CAMPAIGN_CHANGED", "活动已结束或不满足门槛，请刷新页面确认金额后重试")
		}
		return nil, nil
	}
	if req.UserCouponID > 0 {
		return nil, infraerrors.BadRequest("CAMPAIGN_CONFLICT", "当前充值已自动享受活动，不能与优惠券叠加")
	}
	{
		a := *selected
		if req.CampaignID > 0 && (req.CampaignID != a.ID || req.CampaignRevision != a.Revision) {
			return nil, infraerrors.Conflict("CAMPAIGN_CHANGED", "活动规则已更新，请刷新页面确认金额后重试")
		}
		if !a.active(time.Now()) || req.Amount < a.MinAmount {
			return nil, infraerrors.BadRequest("CAMPAIGN_UNAVAILABLE", "活动未开始、已结束或未达到充值门槛，请刷新活动")
		}
		principal, credited := campaignAmounts(a, req.Amount, normalizeBalanceRechargeMultiplier(cfg.BalanceRechargeMultiplier))
		if principal <= 0 {
			return nil, infraerrors.BadRequest("INVALID_AMOUNT", "折扣后金额过小")
		}
		snap := &RechargeCampaignSnapshot{Campaign: a, Principal: principal, Credited: credited}
		if a.RewardPercent > 0 && s.affiliateService != nil && s.affiliateService.IsEnabled(ctx) {
			rows, err := s.entClient.QueryContext(ctx, `SELECT af.inviter_id FROM user_affiliates af JOIN users u ON u.id=af.inviter_id WHERE af.user_id=$1 AND af.inviter_id<>$1 AND u.status='active' AND u.deleted_at IS NULL AND ($2=FALSE OR (af.invited_at >= $3 AND af.invited_at < $4))`, req.UserID, a.NewInviteesOnly, a.StartsAt, a.EndsAt)
			if err != nil {
				return nil, err
			}
			if rows.Next() {
				err = rows.Scan(&snap.InviterID)
			}
			rowErr := rows.Err()
			rows.Close()
			if err != nil {
				return nil, err
			}
			if rowErr != nil {
				return nil, rowErr
			}
			if snap.InviterID > 0 {
				snap.Reward = decimal.NewFromFloat(math.Min(principal*normalizeBalanceRechargeMultiplier(cfg.BalanceRechargeMultiplier)*a.RewardPercent/100, a.RewardCap)).Round(8).InexactFloat64()
			}
		}
		return snap, nil
	}
}

func campaignSnapshot(o *dbent.PaymentOrder) (*RechargeCampaignSnapshot, error) {
	if o == nil || o.ProviderSnapshot == nil {
		return nil, nil
	}
	v, ok := o.ProviderSnapshot["recharge_campaign"]
	if !ok {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var snap RechargeCampaignSnapshot
	if err = json.Unmarshal(raw, &snap); err != nil {
		return nil, fmt.Errorf("invalid campaign snapshot: %w", err)
	}
	return &snap, nil
}

func (s *PaymentService) accrueCampaignReward(ctx context.Context, snap *RechargeCampaignSnapshot, o *dbent.PaymentOrder) (float64, error) {
	if snap.InviterID <= 0 || snap.Reward <= 0 {
		return 0, nil
	}
	applied, err := s.affiliateService.repo.AccrueQuota(ctx, snap.InviterID, o.UserID, snap.Reward, snap.Campaign.FreezeHours, &o.ID)
	if err != nil {
		return 0, err
	}
	if !applied {
		return 0, fmt.Errorf("campaign inviter %d unavailable", snap.InviterID)
	}
	return snap.Reward, nil
}

// Runs in the refund transaction. A negative available quota records debt if the reward was already transferred.
func reverseCampaignReward(ctx context.Context, client *dbent.Client, p *RefundPlan) error {
	snap, err := campaignSnapshot(p.Order)
	if err != nil {
		return err
	}
	if snap == nil || snap.Reward <= 0 {
		return nil
	}
	if _, err := client.ExecContext(ctx, "UPDATE user_affiliates SET updated_at=updated_at WHERE user_id=$1", snap.InviterID); err != nil {
		return err
	}
	rows, err := client.QueryContext(ctx, `SELECT id,user_id,amount,frozen_until FROM user_affiliate_ledger WHERE source_order_id=$1 AND action='accrue' FOR UPDATE`, p.OrderID)
	if err != nil {
		return err
	}
	var id, uid int64
	var amount float64
	var frozenUntil sql.NullTime
	if !rows.Next() {
		err = rows.Err()
		rows.Close()
		return err
	}
	err = rows.Scan(&id, &uid, &amount, &frozenUntil)
	rows.Close()
	if err != nil {
		return err
	}
	reversal := decimal.NewFromFloat(amount).Mul(decimal.NewFromFloat(math.Min(1, p.RefundAmount/p.Order.Amount))).Round(8).InexactFloat64()
	if reversal <= 0 {
		return nil
	}
	column := "aff_quota"
	if frozenUntil.Valid {
		column = "aff_frozen_quota"
	}
	if _, err = client.ExecContext(ctx, fmt.Sprintf("UPDATE user_affiliates SET %s=%s-$1,aff_history_quota=aff_history_quota-$1,updated_at=NOW() WHERE user_id=$2", column, column), reversal, uid); err != nil {
		return err
	}
	_, err = client.ExecContext(ctx, `INSERT INTO user_affiliate_ledger(user_id,action,amount,source_user_id,source_order_id,frozen_until,created_at,updated_at) VALUES($1,'campaign_refund',$2,$3,$4,$5,NOW(),NOW())`, uid, -reversal, p.Order.UserID, p.OrderID, frozenUntil)
	return err
}

// PaymentOrderRechargeCampaign exposes only campaign rules, not provider credentials.
func PaymentOrderRechargeCampaign(o *dbent.PaymentOrder) *RechargeCampaignSnapshot {
	snap, _ := campaignSnapshot(o)
	return snap
}
