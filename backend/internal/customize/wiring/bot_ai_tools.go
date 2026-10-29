package wiring

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/botnotify"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/rechargecampaigns"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// botAITools implements the read-only "skills" the AI may call. Every tool
// returns public information only: what the model plaza, the plans page or
// the public status page already show visitors, or site-wide aggregates.
// Arguments are plain filter keywords, never queries, so a chat message has
// no path to arbitrary data. Exclusive (private) groups are always skipped.
type botAITools struct {
	db           *sql.DB
	data         botnotify.Data
	plaza        plazaSource
	plazaPublic  func(context.Context) bool
	payments     paymentSource
	campaigns    campaignSource
	extensionsOn func(context.Context, string) bool
	now          func() time.Time

	statsMu sync.Mutex
	stats   *cacheStatsSnapshot
}

type plazaSource interface {
	ListGroups(context.Context) ([]service.PlazaGroup, error)
}

type paymentSource interface {
	GetPaymentConfig(context.Context) (*service.PaymentConfig, error)
	GetAvailableMethodLimits(context.Context) (*service.MethodLimitsResponse, error)
	ListPlansForSale(context.Context) ([]*dbent.SubscriptionPlan, error)
	GetGroupInfoMap(context.Context, []*dbent.SubscriptionPlan) map[int64]service.PlanGroupInfo
}

type campaignSource interface {
	List(context.Context, bool) ([]rechargecampaigns.Campaign, error)
}

func toolSchema(name, description string, props map[string]any) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        name,
			"description": description,
			"parameters":  map[string]any{"type": "object", "properties": props, "additionalProperties": false},
		},
	}
}

func keywordProp(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

var aiToolSchemas = map[string]map[string]any{
	botnotify.ToolGroupStatus: toolSchema(botnotify.ToolGroupStatus, "查询各分组（渠道）的当前状态、主模型和近7天可用性。用户问某个分组或模型是否正常、稳不稳定时调用。",
		map[string]any{"group": keywordProp("可选，分组名关键词，只返回名称包含该词的分组")}),
	botnotify.ToolGroups: toolSchema(botnotify.ToolGroups, "查询公开分组的平台、计费方式（扣余额或订阅额度）、倍率和说明。用户问有哪些分组、哪个便宜、分组区别、该选哪个时调用。", nil),
	botnotify.ToolModelPrices: toolSchema(botnotify.ToolModelPrices, "查询模型实付价格：每百万 token 的输入、输出、缓存读取、缓存写入价（已按分组倍率折算）及按次计费价。用户问某个模型多少钱、缓存怎么计费时调用。",
		map[string]any{"model": keywordProp("可选，模型名关键词，例如 gpt-5.5、sonnet、opus"), "group": keywordProp("可选，分组名关键词")}),
	botnotify.ToolPlans:        toolSchema(botnotify.ToolPlans, "查询在售包月套餐的价格、有效期、对应分组和额度。用户问套餐、包月、订阅多少钱时调用。", nil),
	botnotify.ToolRechargeInfo: toolSchema(botnotify.ToolRechargeInfo, "查询支付方式、单笔金额范围、到账比例、充值优惠阶梯和充值活动。用户问怎么充值、充多少送多少、有没有活动时调用。", nil),
	botnotify.ToolCacheStats: toolSchema(botnotify.ToolCacheStats, "查询分组近24小时整体缓存命中率（全站该分组所有请求的聚合统计）。用户问缓存命中率时调用。",
		map[string]any{"group": keywordProp("可选，分组名关键词")}),
	botnotify.ToolAnnouncements: toolSchema(botnotify.ToolAnnouncements, "查询站点最新公告。用户问维护、活动、变更、通知时调用。", nil),
}

// definitions returns the schemas of the tools this configuration enables.
func (t *botAITools) definitions(cfg botnotify.AIConfig) []map[string]any {
	names := botnotify.AIToolsFor(cfg)
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		out = append(out, aiToolSchemas[name])
	}
	return out
}

const toolResultLimit = 3000

// exec runs one enabled tool and returns compact text for the model.
func (t *botAITools) exec(ctx context.Context, cfg botnotify.AIConfig, name, argsJSON string) string {
	enabled := false
	for _, n := range botnotify.AIToolsFor(cfg) {
		enabled = enabled || n == name
	}
	if !enabled {
		return "未知工具，无法查询。"
	}
	var args struct {
		Group string `json:"group"`
		Model string `json:"model"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)
	group := truncateRunes(strings.TrimSpace(args.Group), 40)
	model := truncateRunes(strings.TrimSpace(args.Model), 60)
	toolCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	var result string
	switch name {
	case botnotify.ToolGroupStatus:
		result = t.groupStatus(toolCtx, group)
	case botnotify.ToolGroups:
		result = t.groups(toolCtx)
	case botnotify.ToolModelPrices:
		result = t.modelPrices(toolCtx, model, group)
	case botnotify.ToolPlans:
		result = t.plans(toolCtx)
	case botnotify.ToolRechargeInfo:
		result = t.rechargeInfo(toolCtx)
	case botnotify.ToolCacheStats:
		result = t.cacheStats(toolCtx, group)
	case botnotify.ToolAnnouncements:
		result = t.announcements(toolCtx)
	}
	return truncateRunes(result, toolResultLimit)
}

const toolFailed = "查询失败，请稍后再试。"

func containsFold(s, sub string) bool {
	if sub == "" {
		return true
	}
	norm := func(v string) string {
		return strings.NewReplacer(" ", "", "-", "", "_", "", ".", "").Replace(strings.ToLower(v))
	}
	return strings.Contains(norm(s), norm(sub))
}

func (t *botAITools) groupStatus(ctx context.Context, group string) string {
	channels, err := t.data.Channels(ctx)
	if err != nil {
		return toolFailed
	}
	var lines []string
	for _, c := range channels {
		if !containsFold(c.GroupName, group) {
			continue
		}
		status := "异常"
		switch c.Status {
		case "operational":
			status = "正常"
		case "degraded":
			status = "降级"
		}
		lines = append(lines, fmt.Sprintf("%s｜模型 %s｜状态 %s｜近7天可用性 %.1f%%", c.GroupName, c.Model, status, c.Availability))
	}
	if len(lines) == 0 {
		return "没有找到匹配的分组状态。"
	}
	return strings.Join(lines, "\n")
}

func platformLabel(platform string) string {
	switch platform {
	case service.PlatformAnthropic:
		return "Claude"
	case service.PlatformOpenAI:
		return "OpenAI/GPT"
	case service.PlatformGemini:
		return "Gemini"
	case service.PlatformAntigravity:
		return "Antigravity"
	case service.PlatformGrok:
		return "Grok"
	case service.PlatformComposite:
		return "组合"
	}
	return platform
}

func billingLabel(subscriptionType string) string {
	if subscriptionType == service.SubscriptionTypeSubscription {
		return "订阅分组（扣套餐额度）"
	}
	return "按量（扣余额）"
}

func (t *botAITools) groups(ctx context.Context) string {
	rows, err := t.db.QueryContext(ctx, `
SELECT name, platform, subscription_type, rate_multiplier, COALESCE(description, '')
FROM groups
WHERE deleted_at IS NULL AND status = 'active' AND is_exclusive = false
ORDER BY rate_multiplier ASC, id ASC
LIMIT 50`)
	if err != nil {
		return toolFailed
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var name, platform, subscription, desc string
		var rate float64
		if err := rows.Scan(&name, &platform, &subscription, &rate, &desc); err != nil {
			return toolFailed
		}
		line := fmt.Sprintf("%s｜%s｜%s｜倍率 %s", name, platformLabel(platform), billingLabel(subscription), formatNumber(rate))
		if d := truncateRunes(strings.TrimSpace(botnotify.PlainText(desc)), 80); d != "" {
			line += "｜" + d
		}
		lines = append(lines, line)
	}
	if rows.Err() != nil {
		return toolFailed
	}
	if len(lines) == 0 {
		return "暂时没有可用的分组信息。"
	}
	return strings.Join(lines, "\n")
}

// formatNumber prints a price or rate without float noise.
func formatNumber(v float64) string {
	if v == 0 {
		return "0"
	}
	digits := 2
	if math.Abs(v) < 1 {
		digits = 4
	}
	s := strconv.FormatFloat(v, 'f', digits, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	return s
}

func perMillion(price *float64, rate float64) string {
	if price == nil {
		return ""
	}
	return "$" + formatNumber(*price*rate*1e6)
}

func (t *botAITools) modelPrices(ctx context.Context, model, group string) string {
	if t.plaza == nil || t.plazaPublic == nil || !t.plazaPublic(ctx) {
		return "模型价格目前不对外公开查询，请让用户登录后在控制台的模型广场查看。"
	}
	groups, err := t.plaza.ListGroups(ctx)
	if err != nil {
		return toolFailed
	}
	const maxLines = 40
	var lines []string
	matched := 0
	for _, g := range groups {
		if g.IsExclusive || !containsFold(g.Name, group) {
			continue
		}
		var groupLines []string
		for _, m := range g.Models {
			if !containsFold(m.Name, model) {
				continue
			}
			matched++
			if line := plazaPriceLine(g, m); line != "" {
				groupLines = append(groupLines, line)
			}
		}
		if len(groupLines) == 0 {
			continue
		}
		header := fmt.Sprintf("【%s】%s，倍率 %s", g.Name, billingLabel(g.SubscriptionType), formatNumber(g.RateMultiplier))
		if g.PeakRateEnabled && g.PeakRateMultiplier > 0 && g.PeakRateMultiplier != 1 {
			header += fmt.Sprintf("；高峰时段 %s-%s 再乘 %s", g.PeakStart, g.PeakEnd, formatNumber(g.PeakRateMultiplier))
		}
		lines = append(lines, header)
		lines = append(lines, groupLines...)
		if len(lines) >= maxLines {
			break
		}
	}
	if matched == 0 {
		if model != "" {
			return "公开分组里没有找到名称包含「" + model + "」的模型。可以去掉关键词再查，或到模型广场查看。"
		}
		return "没有找到匹配的分组或模型。"
	}
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "……结果较多，请指定模型名或分组名再查。")
	}
	return "以下为实付价（已乘分组倍率，单位美元）：\n" + strings.Join(lines, "\n")
}

// plazaPriceLine renders one model's display price exactly as the plaza
// shows it: token prices per million, media prices per unit.
func plazaPriceLine(g service.PlazaGroup, m service.PlazaModel) string {
	p := m.Pricing
	if p == nil {
		return ""
	}
	rate := g.RateMultiplier
	switch p.BillingMode {
	case service.BillingModeImage, service.BillingModeVideo, service.BillingModePerRequest:
		unit := "次"
		if p.BillingMode == service.BillingModeImage {
			unit = "张"
			if g.ImageRateIndependent {
				rate = g.ImageRateMultiplier
			}
		} else if p.BillingMode == service.BillingModeVideo {
			unit = "秒"
			if g.VideoRateIndependent {
				rate = g.VideoRateMultiplier
			}
		}
		var tiers []string
		for _, iv := range p.Intervals {
			if iv.PerRequestPrice != nil {
				label := iv.TierLabel
				if label == "" {
					label = "档位"
				}
				tiers = append(tiers, fmt.Sprintf("%s $%s/%s", label, formatNumber(*iv.PerRequestPrice*rate), unit))
			}
		}
		if len(tiers) == 0 && p.PerRequestPrice != nil {
			tiers = append(tiers, fmt.Sprintf("$%s/%s", formatNumber(*p.PerRequestPrice*rate), unit))
		}
		if len(tiers) == 0 {
			return ""
		}
		return m.Name + "｜" + strings.Join(tiers, "，")
	}
	var parts []string
	for _, f := range []struct {
		label string
		price *float64
	}{{"输入", p.InputPrice}, {"输出", p.OutputPrice}, {"缓存读", p.CacheReadPrice}, {"缓存写", p.CacheWritePrice}} {
		if v := perMillion(f.price, rate); v != "" {
			parts = append(parts, f.label+" "+v+"/M")
		}
	}
	if len(parts) == 0 {
		return ""
	}
	line := m.Name + "｜" + strings.Join(parts, "，")
	if len(p.Intervals) > 1 {
		line += "（长上下文分档计费，以上为基础档）"
	}
	if o := m.OfficialPricing; o != nil && o.InputPrice != nil && o.OutputPrice != nil {
		line += fmt.Sprintf("｜官方价 输入 %s/M，输出 %s/M", perMillion(o.InputPrice, 1), perMillion(o.OutputPrice, 1))
	}
	if tp := m.TimePricing; tp != nil && len(tp.Periods) > 0 {
		var periods []string
		for _, period := range tp.Periods {
			periods = append(periods, fmt.Sprintf("%s-%s 乘 %s", period.StartTime, period.EndTime, formatNumber(period.Multiplier)))
		}
		line += "｜分时倍率（" + tp.Timezone + "）：" + strings.Join(periods, "，")
	}
	return line
}

func (t *botAITools) plans(ctx context.Context) string {
	if t.payments == nil {
		return toolFailed
	}
	cfg, err := t.payments.GetPaymentConfig(ctx)
	if err != nil {
		return toolFailed
	}
	if !cfg.Enabled {
		return "在线支付暂未开放，暂时不能在线购买套餐。"
	}
	plans, err := t.payments.ListPlansForSale(ctx)
	if err != nil {
		return toolFailed
	}
	if len(plans) == 0 {
		return "目前没有在售的包月套餐。"
	}
	info := t.payments.GetGroupInfoMap(ctx, plans)
	var lines []string
	for _, p := range plans {
		currency := "¥"
		if p.Currency != "" && !strings.EqualFold(p.Currency, "CNY") {
			currency = p.Currency + " "
		}
		line := fmt.Sprintf("%s｜%s%s", p.Name, currency, formatNumber(p.Price))
		if p.OriginalPrice != nil && *p.OriginalPrice > p.Price {
			line += fmt.Sprintf("（原价 %s%s）", currency, formatNumber(*p.OriginalPrice))
		}
		line += "｜" + planValidity(p.ValidityDays, p.ValidityUnit)
		if gi, ok := info[p.GroupID]; ok {
			line += fmt.Sprintf("｜分组 %s（倍率 %s）", gi.Name, formatNumber(gi.RateMultiplier))
			var limits []string
			for _, l := range []struct {
				label string
				value *float64
			}{{"每日", gi.DailyLimitUSD}, {"每周", gi.WeeklyLimitUSD}, {"每月", gi.MonthlyLimitUSD}} {
				if l.value != nil && *l.value > 0 {
					limits = append(limits, fmt.Sprintf("%s $%s", l.label, formatNumber(*l.value)))
				}
			}
			if len(limits) > 0 {
				line += "｜额度 " + strings.Join(limits, "，")
			}
		}
		if f := planFeatures(p.Features); f != "" {
			line += "｜" + f
		}
		if d := truncateRunes(strings.TrimSpace(botnotify.PlainText(p.Description)), 80); d != "" {
			line += "｜" + d
		}
		lines = append(lines, line)
	}
	return "在售套餐（在「充值/订阅」页的订阅页签购买）：\n" + strings.Join(lines, "\n")
}

func planValidity(n int, unit string) string {
	switch unit {
	case "month", "months":
		return fmt.Sprintf("有效期 %d 个月", n)
	case "week", "weeks":
		return fmt.Sprintf("有效期 %d 周", n)
	}
	return fmt.Sprintf("有效期 %d 天", n)
}

// planFeatures joins the plan's feature lines (one per line in the admin form).
func planFeatures(raw string) string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			out = append(out, s)
		}
	}
	return truncateRunes(strings.Join(out, "；"), 120)
}

func currencyLabel(code string) string {
	if code == "" || strings.EqualFold(code, "CNY") {
		return "元"
	}
	return " " + strings.ToUpper(code)
}

func (t *botAITools) rechargeInfo(ctx context.Context) string {
	if t.payments == nil {
		return toolFailed
	}
	cfg, err := t.payments.GetPaymentConfig(ctx)
	if err != nil {
		return toolFailed
	}
	if !cfg.Enabled {
		return "在线支付暂未开放，可以用卡密兑换充值。"
	}
	var lines []string
	if limits, err := t.payments.GetAvailableMethodLimits(ctx); err == nil && limits != nil {
		keys := make([]string, 0, len(limits.Methods))
		for k := range limits.Methods {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var methods []string
		for _, k := range keys {
			m := limits.Methods[k]
			name := m.DisplayName
			if name == "" {
				name = paymentTypeLabel(m.PaymentType)
			}
			if m.SingleMin > 0 || m.SingleMax > 0 {
				name += fmt.Sprintf("（单笔 %s-%s%s）", formatNumber(m.SingleMin), rangeMax(m.SingleMax), currencyLabel(m.Currency))
			}
			if m.FeeRate > 0 {
				name += fmt.Sprintf("，手续费 %s%%", formatNumber(m.FeeRate))
			}
			methods = append(methods, name)
		}
		if len(methods) > 0 {
			lines = append(lines, "支付方式："+strings.Join(methods, "；"))
		}
	}
	multiplier := cfg.BalanceRechargeMultiplier
	if multiplier <= 0 {
		multiplier = 1
	}
	lines = append(lines, fmt.Sprintf("到账比例：支付 1 元到账 $%s 余额额度", formatNumber(multiplier)))
	if cfg.MinAmount > 0 || cfg.MaxAmount > 0 {
		lines = append(lines, fmt.Sprintf("单笔充值金额：%s-%s 元", formatNumber(cfg.MinAmount), rangeMax(cfg.MaxAmount)))
	}
	if cfg.RechargeFeeRate > 0 {
		lines = append(lines, fmt.Sprintf("充值手续费：%s%%", formatNumber(cfg.RechargeFeeRate)))
	}
	if len(cfg.RechargeBonusTiers) > 0 {
		var tiers []string
		for _, tier := range cfg.RechargeBonusTiers {
			if cfg.RechargeBonusMode == service.RechargeBonusModeDiscount {
				tiers = append(tiers, fmt.Sprintf("满 %s 元实付打 %s 折", formatNumber(tier.MinAmount), formatNumber((100-tier.BonusPercent)/10)))
			} else {
				tiers = append(tiers, fmt.Sprintf("满 %s 元额外赠送 %s%%", formatNumber(tier.MinAmount), formatNumber(tier.BonusPercent)))
			}
		}
		lines = append(lines, "充值优惠阶梯（按单笔金额命中最高一档）："+strings.Join(tiers, "；"))
	}
	if notice := truncateRunes(strings.TrimSpace(botnotify.PlainText(cfg.RechargeBonusNotice)), 300); notice != "" {
		lines = append(lines, "充值页活动说明："+notice)
	}
	lines = append(lines, t.campaignLines(ctx)...)
	return strings.Join(lines, "\n")
}

func paymentTypeLabel(kind string) string {
	switch {
	case strings.Contains(kind, "alipay"):
		return "支付宝"
	case strings.Contains(kind, "wxpay") || strings.Contains(kind, "wechat"):
		return "微信支付"
	case strings.Contains(kind, "stripe"):
		return "银行卡（Stripe）"
	}
	return kind
}

func rangeMax(v float64) string {
	if v <= 0 {
		return "不限"
	}
	return formatNumber(v)
}

// campaignLines lists running and upcoming recharge campaigns, honoring the
// recharge-campaigns extension switch like the public campaign endpoint.
func (t *botAITools) campaignLines(ctx context.Context) []string {
	if t.campaigns == nil || t.extensionsOn == nil || !t.extensionsOn(ctx, customize.RechargeCampaigns) {
		return nil
	}
	items, err := t.campaigns.List(ctx, true)
	if err != nil || len(items) == 0 {
		return nil
	}
	now := t.now()
	lines := []string{"充值活动："}
	for i, a := range items {
		if i >= 5 {
			break
		}
		state := "进行中"
		if now.Before(a.StartsAt) {
			state = "未开始"
		}
		offer := fmt.Sprintf("充值额外赠送 %s%%", formatNumber(a.Percent))
		if a.Kind == "discount" {
			offer = fmt.Sprintf("实付打 %s 折", formatNumber(a.Percent/10))
		}
		line := fmt.Sprintf("- %s（%s）：%s", a.Name, state, offer)
		if a.MinAmount > 0 {
			line += fmt.Sprintf("，单笔满 %s 元可用", formatNumber(a.MinAmount))
		}
		line += fmt.Sprintf("，时间 %s 至 %s", a.StartsAt.In(now.Location()).Format("2006-01-02 15:04"), a.EndsAt.In(now.Location()).Format("2006-01-02 15:04"))
		if d := truncateRunes(strings.TrimSpace(botnotify.PlainText(a.Description)), 100); d != "" {
			line += "；" + d
		}
		lines = append(lines, line)
	}
	return lines
}

// cacheStatsSnapshot is the cached aggregate of all public groups.
type cacheStatsSnapshot struct {
	at     time.Time
	groups []groupCacheStats
}

type groupCacheStats struct {
	name                          string
	requests                      int64
	input, cacheCreate, cacheRead int64
}

const (
	cacheStatsWindow = 24 * time.Hour
	cacheStatsTTL    = 10 * time.Minute
	// Below these volumes a rate is noise and could describe a single user.
	cacheStatsMinRequests     = 50
	cacheStatsMinPromptTokens = 200000
)

// cacheStats reports the site-wide cache hit rate of public groups over the
// last 24 hours: cache-read tokens / all prompt tokens, the same formula as
// the dashboard. Small samples are withheld rather than reported.
func (t *botAITools) cacheStats(ctx context.Context, group string) string {
	snapshot, err := t.cacheSnapshot(ctx)
	if err != nil {
		return toolFailed
	}
	var lines []string
	for _, g := range snapshot.groups {
		if !containsFold(g.name, group) {
			continue
		}
		prompt := g.input + g.cacheCreate + g.cacheRead
		if g.requests < cacheStatsMinRequests || prompt < cacheStatsMinPromptTokens {
			lines = append(lines, g.name+"｜近24小时请求量较少，暂无可靠统计")
			continue
		}
		lines = append(lines, fmt.Sprintf("%s｜近24小时缓存命中率 %.1f%%（缓存读取 %s / 输入总量 %s token）", g.name, float64(g.cacheRead)*100/float64(prompt), compactTokens(g.cacheRead), compactTokens(prompt)))
	}
	if len(lines) == 0 {
		return "没有找到匹配的公开分组。"
	}
	return "统计口径：全站该分组近24小时所有请求，命中率 = 缓存读取 token ÷（输入 + 缓存写入 + 缓存读取）token；个人命中率取决于使用方式，可在用量记录页查看每次请求的缓存读取。\n" + strings.Join(lines, "\n")
}

func compactTokens(n int64) string {
	switch {
	case n >= 100_000_000:
		return formatNumber(float64(n)/100_000_000) + " 亿"
	case n >= 10_000:
		return formatNumber(float64(n)/10_000) + " 万"
	}
	return strconv.FormatInt(n, 10)
}

func (t *botAITools) cacheSnapshot(ctx context.Context) (*cacheStatsSnapshot, error) {
	t.statsMu.Lock()
	defer t.statsMu.Unlock()
	now := t.now()
	if t.stats != nil && now.Sub(t.stats.at) < cacheStatsTTL {
		return t.stats, nil
	}
	rows, err := t.db.QueryContext(ctx, `
SELECT id, name FROM groups
WHERE deleted_at IS NULL AND status = 'active' AND is_exclusive = false
ORDER BY rate_multiplier ASC, id ASC
LIMIT 50`)
	if err != nil {
		return nil, err
	}
	var ids []int64
	names := map[int64]string{}
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
		names[id] = name
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	snapshot := &cacheStatsSnapshot{at: now}
	if len(ids) > 0 {
		// Served by the (group_id, created_at) partial index on usage_logs.
		agg, err := t.db.QueryContext(ctx, `
SELECT group_id, COUNT(*), COALESCE(SUM(input_tokens), 0), COALESCE(SUM(cache_creation_tokens), 0), COALESCE(SUM(cache_read_tokens), 0)
FROM usage_logs
WHERE group_id = ANY($1) AND created_at >= $2
GROUP BY group_id`, pq.Array(ids), now.Add(-cacheStatsWindow))
		if err != nil {
			return nil, err
		}
		byID := map[int64]groupCacheStats{}
		for agg.Next() {
			var id int64
			var s groupCacheStats
			if err := agg.Scan(&id, &s.requests, &s.input, &s.cacheCreate, &s.cacheRead); err != nil {
				agg.Close()
				return nil, err
			}
			byID[id] = s
		}
		agg.Close()
		if err := agg.Err(); err != nil {
			return nil, err
		}
		for _, id := range ids {
			s := byID[id]
			s.name = names[id]
			snapshot.groups = append(snapshot.groups, s)
		}
	}
	t.stats = snapshot
	return snapshot, nil
}

func (t *botAITools) announcements(ctx context.Context) string {
	list, err := t.data.Announcements(ctx)
	if err != nil {
		return toolFailed
	}
	var parts []string
	for i, a := range list {
		if i >= 3 {
			break
		}
		body := truncateRunes(strings.TrimSpace(botnotify.PlainText(a.Content)), 300)
		if body == "" {
			parts = append(parts, a.Title)
		} else {
			parts = append(parts, a.Title+"："+body)
		}
	}
	if len(parts) == 0 {
		return "目前没有公告。"
	}
	return strings.Join(parts, "\n")
}
