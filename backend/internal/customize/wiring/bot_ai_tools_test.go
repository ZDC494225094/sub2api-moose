package wiring

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/rechargecampaigns"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type stubBotData struct{}

func (*stubBotData) Channels(context.Context) ([]botnotify.Channel, error) {
	return []botnotify.Channel{{GroupName: "Codex-Pro", Model: "gpt-5.5", Status: "operational", Availability: 99.5}}, nil
}
func (*stubBotData) Announcements(context.Context) ([]botnotify.Announcement, error) {
	return []botnotify.Announcement{{ID: 1, Title: "维护通知", Content: "**今晚** 22:00 维护"}}, nil
}
func (*stubBotData) SiteName(context.Context) (string, error) { return "麋鹿云", nil }

type stubPlaza struct{ groups []service.PlazaGroup }

func (s stubPlaza) ListGroups(context.Context) ([]service.PlazaGroup, error) { return s.groups, nil }

func ptr(v float64) *float64 { return &v }

func TestModelPricesAreRateAdjustedAndSkipExclusiveGroups(t *testing.T) {
	plaza := stubPlaza{groups: []service.PlazaGroup{
		{Name: "Codex-Pro", RateMultiplier: 0.35, Models: []service.PlazaModel{{
			Name:            "gpt-5.5",
			Pricing:         &service.ChannelModelPricing{BillingMode: service.BillingModeToken, InputPrice: ptr(2e-6), OutputPrice: ptr(16e-6), CacheReadPrice: ptr(0.2e-6)},
			OfficialPricing: &service.PlazaOfficialPricing{InputPrice: ptr(2e-6), OutputPrice: ptr(16e-6)},
		}, {
			Name:    "gpt-image-2",
			Pricing: &service.ChannelModelPricing{BillingMode: service.BillingModeImage, Intervals: []service.PricingInterval{{TierLabel: "1K", PerRequestPrice: ptr(0.04)}}},
		}}},
		{Name: "VIP 专属", RateMultiplier: 0.1, IsExclusive: true, Models: []service.PlazaModel{{
			Name: "gpt-5.5", Pricing: &service.ChannelModelPricing{BillingMode: service.BillingModeToken, InputPrice: ptr(2e-6)},
		}}},
	}}
	tools := &botAITools{plaza: plaza, plazaPublic: func(context.Context) bool { return true }}
	got := tools.modelPrices(context.Background(), "gpt5.5", "")
	require.Contains(t, got, "【Codex-Pro】按量（扣余额），倍率 0.35")
	require.Contains(t, got, "gpt-5.5｜输入 $0.7/M，输出 $5.6/M，缓存读 $0.07/M｜官方价 输入 $2/M，输出 $16/M")
	require.NotContains(t, got, "VIP 专属", "exclusive groups are never disclosed")
	require.NotContains(t, got, "gpt-image-2", "model keyword filters")
	require.Contains(t, tools.modelPrices(context.Background(), "image", ""), "gpt-image-2｜1K $0.014/张")

	closed := &botAITools{plaza: plaza, plazaPublic: func(context.Context) bool { return false }}
	require.Contains(t, closed.modelPrices(context.Background(), "", ""), "不对外公开")
}

func TestCacheStatsAggregatePublicGroupsAndWithholdSmallSamples(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	tools := &botAITools{db: db, now: func() time.Time { return now }}
	mock.ExpectQuery("SELECT id, name FROM groups").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(1, "Codex-Pro").AddRow(2, "冷门分组"))
	mock.ExpectQuery("FROM usage_logs").WithArgs(sqlmock.AnyArg(), now.Add(-24*time.Hour)).WillReturnRows(
		sqlmock.NewRows([]string{"group_id", "count", "input", "create", "read"}).
			AddRow(1, 5000, 20_000_000, 5_000_000, 75_000_000).
			AddRow(2, 3, 1000, 0, 500))
	got := tools.cacheStats(context.Background(), "")
	require.Contains(t, got, "Codex-Pro｜近24小时缓存命中率 75.0%")
	require.Contains(t, got, "冷门分组｜近24小时请求量较少，暂无可靠统计")
	require.Contains(t, got, "个人命中率取决于使用方式")
	// Cached: a second question within the TTL does not query again.
	require.Contains(t, tools.cacheStats(context.Background(), "pro"), "75.0%")
	require.NoError(t, mock.ExpectationsWereMet())
}

type stubPayments struct {
	cfg   service.PaymentConfig
	plans []*dbent.SubscriptionPlan
}

func (s stubPayments) GetPaymentConfig(context.Context) (*service.PaymentConfig, error) {
	return &s.cfg, nil
}
func (s stubPayments) GetAvailableMethodLimits(context.Context) (*service.MethodLimitsResponse, error) {
	return &service.MethodLimitsResponse{Methods: map[string]service.MethodLimits{"alipay": {PaymentType: "alipay", SingleMin: 1, SingleMax: 5000}}}, nil
}
func (s stubPayments) ListPlansForSale(context.Context) ([]*dbent.SubscriptionPlan, error) {
	return s.plans, nil
}
func (s stubPayments) GetGroupInfoMap(context.Context, []*dbent.SubscriptionPlan) map[int64]service.PlanGroupInfo {
	return map[int64]service.PlanGroupInfo{3: {Name: "Codex-Pro", RateMultiplier: 0.35, MonthlyLimitUSD: ptr(100)}}
}

type stubCampaigns struct{ items []rechargecampaigns.Campaign }

func (s stubCampaigns) List(context.Context, bool) ([]rechargecampaigns.Campaign, error) {
	return s.items, nil
}

func TestRechargeInfoAndPlansDescribeLiveOffers(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	payments := stubPayments{
		cfg: service.PaymentConfig{Enabled: true, BalanceRechargeMultiplier: 1, RechargeBonusMode: service.RechargeBonusModeBonus,
			RechargeBonusTiers: []service.RechargeBonusTier{{MinAmount: 500, BonusPercent: 10}}},
		plans: []*dbent.SubscriptionPlan{{GroupID: 3, Name: "包月 100", Price: 31.5, OriginalPrice: ptr(45), ValidityDays: 30, ValidityUnit: "day", Features: "Pro 号池\n不降智"}},
	}
	campaignOn := true
	tools := &botAITools{
		payments:     payments,
		campaigns:    stubCampaigns{items: []rechargecampaigns.Campaign{{Name: "双十一", Kind: "bonus", Percent: 20, MinAmount: 100, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(48 * time.Hour)}}},
		extensionsOn: func(_ context.Context, id string) bool { return id == customize.RechargeCampaigns && campaignOn },
		now:          func() time.Time { return now },
	}
	info := tools.rechargeInfo(context.Background())
	require.Contains(t, info, "支付方式：支付宝（单笔 1-5000元）")
	require.Contains(t, info, "支付 1 元到账 $1 余额额度")
	require.Contains(t, info, "满 500 元额外赠送 10%")
	require.Contains(t, info, "双十一（进行中）：充值额外赠送 20%，单笔满 100 元可用")
	campaignOn = false
	require.NotContains(t, tools.rechargeInfo(context.Background()), "双十一", "campaigns follow their extension switch")

	plans := tools.plans(context.Background())
	require.Contains(t, plans, "包月 100｜¥31.5（原价 ¥45）｜有效期 30 天｜分组 Codex-Pro（倍率 0.35）｜额度 每月 $100｜Pro 号池；不降智")

	payments.cfg.Enabled = false
	tools.payments = payments
	require.Contains(t, tools.plans(context.Background()), "在线支付暂未开放")
}

func TestToolExecHonorsConfigSwitches(t *testing.T) {
	tools := &botAITools{data: &stubBotData{}}
	cfg := botnotify.DefaultAIConfig(1)
	require.Contains(t, tools.exec(context.Background(), cfg, botnotify.ToolGroupStatus, `{"group":"codex"}`), "Codex-Pro｜模型 gpt-5.5｜状态 正常")
	require.Equal(t, "维护通知：今晚 22:00 维护", tools.exec(context.Background(), cfg, botnotify.ToolAnnouncements, ""))
	cfg.AllowUsageStats, cfg.AllowRecharge = false, false
	require.Equal(t, "未知工具，无法查询。", tools.exec(context.Background(), cfg, botnotify.ToolCacheStats, ""))
	require.Equal(t, "未知工具，无法查询。", tools.exec(context.Background(), cfg, botnotify.ToolPlans, ""))
	require.Len(t, tools.definitions(cfg), 4)
}
