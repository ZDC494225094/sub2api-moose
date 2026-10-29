package botnotify

import "strings"

// MaxKnowledgeBaseRunes bounds the admin-maintained knowledge base; it is
// sent with every model call, so it should stay a concise fact sheet.
const MaxKnowledgeBaseRunes = 20000

// DefaultKnowledgeBase is used while the admin has not saved their own text.
// It holds site facts the model must not guess: flows, rules and page links.
// Live data (prices, plans, offers, status) comes from tools instead.
const DefaultKnowledgeBase = `基本规则：
- 本站提供 Claude、GPT（Codex）等模型的 API 中转服务，按量计费：实际扣费 = 模型官方价 × 分组倍率，例如 0.35 倍率就是按官方价的 35% 计费。
- 注册：官网注册账号（邮箱加密码），完成邮箱验证后登录；也可以用 GitHub、Google 或 Linux.do 账号一键登录。
- 充值：控制台左侧「充值/订阅」在线支付，目前只有支付宝，人民币 1:1 到账美元额度（充 ¥100 到账 $100 额度）；也可以在左侧「卡密充值」打开的发卡商城买卡密，再到「兑换」页输入卡密，兑换余额或订阅。
- 余额与订阅：余额不过期，用多少扣多少；订阅（包月套餐）按周期重置额度，到期作废，没用完不退。普通分组扣余额，订阅分组扣套餐额度，互不影响。
- 创建密钥：控制台「API 密钥」页创建，创建时选分组；密钥完整内容只在创建时显示一次，务必立即复制保存。密钥泄露后立即在该页删除并新建。
- 分组：每个密钥绑定一个分组，倍率越低越便宜；不同分组在稳定性、并发和可用模型上有区别。用着不合适可以在「API 密钥」页直接换分组，不用重建密钥。
- 接入工具：推荐用 CC Switch（官网 ccswitch.io）一键导入，点 API 密钥页密钥行的「导入到 CCS」即可；Claude Code、Claude Desktop、Codex 都能在 CC Switch 里切换使用。
- Claude Desktop：导入后还要在 CC Switch 左侧选「Claude Desktop」，顶部切到「路由」，选中本站供应商（显示「路由中」），再打开 Claude Desktop。只在 Claude Code 页启用不等于配置了 Claude Desktop。
- 手动配置注意 /v1 后缀：Claude Code 的 ANTHROPIC_BASE_URL 不要带 /v1；Codex 的 base_url 要带 /v1。填错会报 401 或 404，具体按使用手册对应工具的那一节填。
- 常见报错：401 多为密钥填错或地址填错；余额不足请充值或检查订阅额度；模型不存在多为当前分组没有这个模型，到模型广场确认后换分组；“Failed to read request body”的 400 多为客户端取消或网络中断，重试即可。

页面链接（原样使用，不要改动）：
- 注册：https://moosecloud.cc/register
- 登录：https://moosecloud.cc/login
- 充值/订阅：https://moosecloud.cc/purchase
- 我的订阅：https://moosecloud.cc/subscriptions
- 卡密购买：https://wzyp.cn/shop/moosecloud （外部发卡商城，买完回站内兑换）
- 卡密兑换：https://moosecloud.cc/redeem
- API 密钥（创建和管理）：https://moosecloud.cc/keys
- 模型广场（模型与价格）：https://moosecloud.cc/model-plaza
- 可用渠道（分组状态）：https://moosecloud.cc/available-channels
- 用量记录：https://moosecloud.cc/usage
- 订单记录：https://moosecloud.cc/orders
- 图文使用手册（从注册到接入工具的完整教程）：https://moosecloud.cc/docs

人工客服：网站页面右下角悬浮的「联系客服」，或加 QQ 1076623975。`

// EffectiveKnowledgeBase returns the admin text, or the default when empty.
func EffectiveKnowledgeBase(c AIConfig) string {
	if kb := strings.TrimSpace(c.KnowledgeBase); kb != "" {
		return kb
	}
	return DefaultKnowledgeBase
}

// normalizeKnowledgeBase stores the default as empty, so an untouched
// knowledge base keeps following improvements to the built-in text.
func normalizeKnowledgeBase(text string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == strings.TrimSpace(DefaultKnowledgeBase) {
		return ""
	}
	return text
}
