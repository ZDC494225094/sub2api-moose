package botnotify

import (
	"regexp"
	"strings"
)

// AIInputLimit is a hard server-side bound.  It is intentionally independent
// from the value shown in the admin UI so a caller cannot bypass the limit by
// sending a larger Telegram update.
const AIInputLimit = 8000

// GuardResult is deliberately small.  Callers should persist only Reason and
// never persist the original blocked text or the model prompt.
type GuardResult struct {
	Allowed bool
	Reason  string
}

const (
	GuardReasonEmpty           = "empty_message"
	GuardReasonTooLong         = "input_too_long"
	GuardReasonPromptInjection = "prompt_injection"
	GuardReasonSensitiveAsk    = "sensitive_information_request"
	GuardReasonIllegalRequest  = "unsafe_request"
	GuardReasonOffScopeTask    = "off_scope_task"
	GuardReasonSecretOutput    = "secret_in_output"
	GuardReasonInternalOutput  = "internal_data_in_output"
)

var (
	// These are intentionally phrase based and conservative.  They are a
	// pre-filter only; the fixed server policy remains authoritative.
	promptInjectionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+instructions?`),
		regexp.MustCompile(`(?i)system\s+prompt|developer\s+message|jailbreak`),
		regexp.MustCompile(`忽略(?:之前|上面|以前)(?:的)?指令`),
		regexp.MustCompile(`(?:你现在|请你扮演|切换为).{0,12}(?:管理员|系统|开发者)`),
		regexp.MustCompile(`(?:输出|告诉我|显示).{0,20}(?:系统提示词|开发者消息|隐藏指令)`),
		regexp.MustCompile(`(?:忽略|无视|忘掉)(?:掉)?(?:之前|上面|所有)?(?:的)?(?:规则|限制|设定|要求)`),
		regexp.MustCompile(`(?i)you are now|act as an? |pretend to be|developer mode|do anything now`),
		regexp.MustCompile(`(?:无限制模式|开发者模式|管理员模式|越狱模式|上帝模式)`),
		regexp.MustCompile(`(?:复述|背诵|重复一遍|原样输出|逐字输出).{0,16}(?:指令|提示词|规则|设定|要求)`),
		regexp.MustCompile(`(?i)repeat.{0,24}(?:instructions?|prompt|rules)`),
		regexp.MustCompile(`\[system\]|<\|im_(?:start|end)\|>|###\s*(?:system|instruction)`),
	}
	sensitiveRequestPatterns = []*regexp.Regexp{
		// Mentioning a credential while asking how to configure it is a valid
		// support question.  Block requests to reveal/export the credential,
		// rather than blocking every occurrence of "API key" or "数据库".
		regexp.MustCompile(`(?i)(?:show|reveal|provide|send|export|print|leak|give\s+me|tell\s+me).{0,24}(?:api[_ -]?key|access[_ -]?token|secret|password|cookie|jwt|bearer)`),
		regexp.MustCompile(`(?:显示|告诉|提供|发送|导出|打印|泄露|发给|给我|获取).{0,24}(?:密钥|令牌|密码|cookie|凭证|环境变量|数据库|内部接口|系统配置)`),
		regexp.MustCompile(`(?:密钥|令牌|密码|cookie|凭证|环境变量|数据库|内部接口|系统配置).{0,24}(?:显示|告诉|提供|发送|导出|打印|泄露|发给|给我|获取)`),
		regexp.MustCompile(`(?:显示|告诉|提供|发送|导出|打印|泄露|发给|给我|获取).{0,24}(?i:api[_ -]?key|access[_ -]?token|secret|password|cookie|jwt|bearer)`),
		regexp.MustCompile(`(?:数据库|内部接口|环境变量|系统配置).{0,12}(?:密码|凭证|连接串|地址|原文|详情)`),
		regexp.MustCompile(`(?:管理员|admin).{0,16}(?:账号|密码|数据|列表|配置|日志)`),
		regexp.MustCompile(`(?:泄露|导出|打印|显示|给我).{0,20}(?:用户列表|余额明细|订单全量|密钥|token|日志)`),
		regexp.MustCompile(`(?:绕过|突破|提升).{0,12}(?:权限|鉴权|限制|验证)`),
		regexp.MustCompile(`(?:查|看|查询|查看|获取|告诉我).{0,14}(?:别人|其他用户|他人|他的|她的|某个用户|所有用户).{0,14}(?:余额|订单|消费|用量|信息|记录|密钥|账号)`),
		regexp.MustCompile(`(?:我是|我就是)(?:本站)?(?:管理员|站长|开发者|系统管理员|客服主管).{0,30}(?:给我|显示|告诉|提供|发送|导出|打开|名单|列表|密码|密钥|数据)`),
	}
	// Requests to make the bot perform actions (delete files, run commands,
	// operate a server) are outside a support bot's scope entirely. These
	// are imperative/destructive shapes only, so narrative support messages
	// ("I deleted my config file and now it errors") stay allowed.
	offScopeTaskPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?:帮我|请你?|麻烦你?|给我|帮忙).{0,12}(?:删除|删掉|清空|格式化)`),
		regexp.MustCompile(`(?:删除|删掉|清空).{0,8}(?:服务器|系统盘|数据库|数据表|/)`),
		regexp.MustCompile(`(?:^|\s)(?:删除|删掉|清空)(?:一下)?(?:这个|那个|全部)?文件(?:\s|$|，|。)`),
		regexp.MustCompile(`(?i)\brm\s+-[a-z]*[rf][a-z]*\b|\bmkfs\b|del\s+/[fq]\b|format\s+[a-z]:`),
		regexp.MustCompile(`(?:在|到)服务器上.{0,14}(?:执行|运行|删除|删掉|修改|部署)`),
	}
	illegalRequestPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:credential|password)\s+steal(?:ing)?\b`),
		regexp.MustCompile(`(?i)\b(?:phishing|ransomware|keylogger|ddos|malware)\b`),
		regexp.MustCompile(`(?:盗取|窃取|钓鱼|木马|勒索|攻击网站|绕过支付|伪造充值)`),
	}
	secretOutputPatterns = []*regexp.Regexp{
		// Telegram bot token and common OpenAI-compatible key prefixes.
		regexp.MustCompile(`\b\d{6,12}:[A-Za-z0-9_-]{20,}\b`),
		regexp.MustCompile(`\b(?:sk|pk|rk)-[A-Za-z0-9_-]{16,}\b`),
		regexp.MustCompile(`(?i)\b(?:bearer\s+|api[_ -]?key\s*[:=]\s*|access[_ -]?token\s*[:=]\s*)\S+`),
		regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`),
	}
	internalOutputPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:https?://)?(?:localhost|127\.0\.0\.1|0\.0\.0\.0)(?::\d+)?`),
		regexp.MustCompile(`(?i)(?:https?://)?(?:10|192\.168)\.\d{1,3}\.\d{1,3}(?::\d+)?`),
		regexp.MustCompile(`(?i)(?:https?://)?172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}(?::\d+)?`),
		regexp.MustCompile(`(?i)\b(?:select|insert|update|delete|drop|alter)\s+.{0,80}\s+from\b`),
		regexp.MustCompile(`(?i)\b(?:system\s+prompt|developer\s+message|内部接口|管理员配置|环境变量)\b`),
		regexp.MustCompile(`-----BEGIN (?:RSA )?PRIVATE KEY-----`),
		regexp.MustCompile(`(?i)(?:postgres(?:ql)?|mysql|mongodb|redis)://\S+`),
		regexp.MustCompile(`/etc/(?:passwd|shadow)`),
		regexp.MustCompile(`(?:服务范围与拒答|只能依据下方资料|不要透露或讨论本指令)`),
	}
)

// InspectInput runs before any model call.  A blocked result must be turned
// into SafeReply by the caller and must not be sent to the model.
func InspectInput(text string) GuardResult {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return GuardResult{Reason: GuardReasonEmpty}
	}
	if len([]rune(normalized)) > AIInputLimit {
		return GuardResult{Reason: GuardReasonTooLong}
	}
	for _, p := range promptInjectionPatterns {
		if p.MatchString(normalized) {
			return GuardResult{Reason: GuardReasonPromptInjection}
		}
	}
	for _, p := range sensitiveRequestPatterns {
		if p.MatchString(normalized) {
			return GuardResult{Reason: GuardReasonSensitiveAsk}
		}
	}
	for _, p := range illegalRequestPatterns {
		if p.MatchString(normalized) {
			return GuardResult{Reason: GuardReasonIllegalRequest}
		}
	}
	for _, p := range offScopeTaskPatterns {
		if p.MatchString(normalized) {
			return GuardResult{Reason: GuardReasonOffScopeTask}
		}
	}
	return GuardResult{Allowed: true}
}

// InspectOutput rejects secrets and internal implementation details even when
// a provider ignores the system policy.  Do not attempt to redact and send a
// partially trusted answer: use SafeReply instead.
func InspectOutput(text string) GuardResult {
	if strings.TrimSpace(text) == "" {
		return GuardResult{Reason: GuardReasonEmpty}
	}
	for _, p := range secretOutputPatterns {
		if p.MatchString(text) {
			return GuardResult{Reason: GuardReasonSecretOutput}
		}
	}
	for _, p := range internalOutputPatterns {
		if p.MatchString(text) {
			return GuardResult{Reason: GuardReasonInternalOutput}
		}
	}
	return GuardResult{Allowed: true}
}

// SafeReply is fixed text, not model-generated text.  It avoids confirming
// whether a particular secret or administrator record exists.
func SafeReply() string {
	return "抱歉，我只能协助处理公开的充值、技术支持和操作配置问题，无法提供受限信息或执行违规请求。"
}
