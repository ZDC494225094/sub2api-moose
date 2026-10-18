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
