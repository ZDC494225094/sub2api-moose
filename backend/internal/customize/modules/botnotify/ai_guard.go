package botnotify

import (
	"net"
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
	GuardReasonUntrustedLink   = "untrusted_link_in_output"
)

// The input patterns are a cheap pre-filter for unambiguous attacks only.
// The model cannot reach credentials, user data or servers through any tool,
// so a missed phrasing costs one refused answer, while a false positive turns
// an everyday question ("怎么获取 API 密钥") into a refusal. Patterns therefore
// target the attack shape (whose data, which verb), never a bare keyword.
var (
	promptInjectionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)ignore\s+(?:all\s+|any\s+)?(?:the\s+)?(?:previous|prior|above|earlier)\s+(?:instructions?|prompts?|rules)`),
		regexp.MustCompile(`(?i)\bjailbreak|\bdan\s+mode\b|do\s+anything\s+now|developer\s+mode`),
		regexp.MustCompile(`(?i)\b(?:reveal|show|print|repeat|output|tell\s+me|give\s+me|leak|dump|display)\b.{0,30}\b(?:system\s+prompt|developer\s+message|hidden\s+(?:instructions?|prompt)|initial\s+(?:instructions?|prompt)|your\s+(?:instructions?|prompt|rules))`),
		regexp.MustCompile(`(?i)\byou\s+are\s+now\b|\bpretend\s+(?:to\s+be|you\s+are)\b|\bact\s+as\s+(?:an?\s+)?(?:admin|administrator|developer|system|root|unrestricted)\b`),
		regexp.MustCompile(`(?i)\[system\]|</?system>|<\|im_(?:start|end)\|>|###\s*(?:system|instruction)`),
		regexp.MustCompile(`忽略(?:掉)?(?:你)?(?:之前|上面|以前|前面|所有|全部)(?:的)?(?:指令|提示词|设定|规则|限制|要求)`),
		regexp.MustCompile(`(?:无视|抛开)(?:掉)?(?:你)?(?:之前|上面|以前|前面|所有|全部)?(?:的)?(?:指令|提示词|设定|规则|限制)`),
		regexp.MustCompile(`(?:忘掉|忘记)(?:你)?(?:之前|上面|以前|前面|所有|全部)(?:的)?(?:指令|提示词|设定|规则|限制)`),
		regexp.MustCompile(`(?:你现在是|从现在起你是|从现在开始你是|请你扮演|你来扮演|你扮演|扮演成|扮演一个|扮演一名).{0,8}(?:管理员|开发者|站长|超级用户|root|无限制|越狱|系统)`),
		regexp.MustCompile(`(?:无限制模式|管理员模式|越狱模式|上帝模式)`),
		regexp.MustCompile(`(?:输出|告诉我|显示|发我|给我看|打印|泄露|透露|复述|背诵|重复一遍|原样输出|逐字输出|完整输出|翻译).{0,16}(?:系统提示词|系统指令|系统设定|开发者消息|隐藏指令|初始指令|内部规则|全部规则|你的规则|你的指令|你的设定|你的提示词)`),
		regexp.MustCompile(`(?i)\brepeat\b.{0,24}\b(?:your|the\s+above|previous|system)\s+(?:instructions?|prompt|rules)`),
	}
	sensitiveRequestPatterns = []*regexp.Regexp{
		// Someone else's credentials, never the user's own "怎么获取密钥". The
		// owner must be directly attached ("管理员的密码"), so "管理员，我的密钥
		// 失效了" — users often address the bot as an admin — stays allowed.
		regexp.MustCompile(`(?:管理员|站长|别人|其他用户|其它用户|别的用户|他人|所有用户|全部用户)(?:的)?\s*(?:api[ _-]?key|密钥|令牌|token|密码|cookie|凭证|session)`),
		regexp.MustCompile(`(?:数据库|服务器|内网|redis|postgres|mysql)(?:的)?(?:密码|账号密码|连接串|连接地址|ip地址|root密码)`),
		regexp.MustCompile(`(?:查|看|查询|查看|获取|告诉我|导出|给我|发我|列出).{0,14}(?:别人|其他用户|其它用户|别的用户|他人|某个用户|某位用户|所有用户|全部用户|用户列表|全站用户).{0,14}(?:余额|订单|消费|用量|信息|记录|密钥|账号|邮箱|手机|充值|数据|ip)`),
		regexp.MustCompile(`(?:泄露|拖库|脱库|dump).{0,20}(?:用户|余额|订单|数据库|日志|密钥)|(?:导出|打印)(?:全部|所有|全站|整个)(?:的)?(?:用户|订单|数据库|日志|密钥)`),
		regexp.MustCompile(`(?:绕过|绕开|破解|跳过|逃过|规避)(?:掉)?.{0,8}(?:鉴权|计费|扣费|付费|支付|风控|实名)|提权|越权|提升.{0,4}(?:权限|管理员)`),
		regexp.MustCompile(`(?:我是|我就是|本人是)(?:本站|你们|网站)?(?:的)?(?:管理员|站长|开发者|系统管理员|客服主管|老板|运维).{0,30}(?:给我|显示|告诉|提供|发送|导出|打开|名单|列表|密码|密钥|数据|权限|后台)`),
		regexp.MustCompile(`(?i)\b(?:show|reveal|leak|dump|export|print|give\s+me|send\s+me|tell\s+me)\s+(?:me\s+)?(?:the\s+|your\s+|all\s+|other\s+users?'?s?\s+|admin(?:istrator)?'?s?\s+)(?:api[_ -]?keys?|access[_ -]?tokens?|secrets?|passwords?|cookies?|credentials?|database|env(?:ironment)?\s+variables?)`),
	}
	illegalRequestPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:write|build|create|make|develop|code|generate)\b.{0,24}\b(?:phishing|ransomware|keylogger|malware|botnet|stealer|trojan)\b`),
		regexp.MustCompile(`(?i)\bddos\b|\bcredential\s+steal|\bcard(?:ing)?\s+fraud`),
		regexp.MustCompile(`(?i)(?:写|做|搭建|制作|开发|生成|教我|帮我写|帮我做).{0,10}(?:钓鱼|木马|勒索|病毒|外挂|盗号|黑产|撞库|keylogger|ransomware|malware|phishing|trojan|stealer|botnet)`),
		// Intent to commit, not victims asking for help ("我的号被盗刷了").
		regexp.MustCompile(`(?:教我|帮我|替我|给我|怎么|如何|怎样)(?:去)?(?:盗取|窃取|盗刷|盗号|撞库|攻击网站|攻击服务器|刷单|洗钱)`),
		regexp.MustCompile(`(?:伪造|篡改)(?:充值|订单|支付|付款)(?:记录|截图|凭证)?`),
	}
	// A support bot can only chat, so requests to operate systems are out of
	// scope; only imperative shapes are matched so a narrative ("我执行了
	// rm -rf ~/.codex 之后…") stays a normal troubleshooting question.
	offScopeTaskPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\s*(?:sudo\s+)?(?:rm\s+-[a-z]*[rf]|mkfs|format\s+[a-z]:|del\s+/[fsq]\b|drop\s+(?:table|database)\b)`),
		regexp.MustCompile(`(?i)(?:帮我|请|给我|替我)(?:执行|运行|跑).{0,6}(?:rm\s+-|mkfs|format\s+[a-z]:|del\s+/|drop\s+(?:table|database))`),
		regexp.MustCompile(`(?:帮我|请你?|麻烦你?|给我|替我|你去)(?:去)?(?:删除|删掉|清空|格式化|重启)(?:一下)?(?:你们的?)?(?:服务器|数据库|数据表|系统盘|全部用户|所有用户|所有数据|全部数据)`),
		regexp.MustCompile(`(?:帮我|请你?|你去|替我)(?:在|到|去)(?:你们的?)?服务器(?:上)?.{0,10}(?:执行|运行|删除|删掉|修改|部署|安装)`),
		regexp.MustCompile(`(?:^|\s)(?:删除|删掉|清空)(?:一下)?(?:这个|那个|全部|所有)?文件(?:\s|$|，|。|,|\.)`),
	}
	secretOutputPatterns = []*regexp.Regexp{
		// Telegram bot token.
		regexp.MustCompile(`\b\d{6,12}:[A-Za-z0-9_-]{20,}\b`),
		regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----`),
		regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`),
		// Connection strings that carry user:password@.
		regexp.MustCompile(`(?i)\b(?:postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|amqp)://[^\s:/@]+:[^\s@/]+@`),
	}
	// Key-shaped values are checked separately so documentation placeholders
	// (sk-xxxxxxxx, "Bearer 你的密钥") stay allowed in configuration examples.
	secretValuePatterns = []*regexp.Regexp{
		regexp.MustCompile(`\b(?:sk|pk|rk)-[A-Za-z0-9_-]{16,}`),
		regexp.MustCompile(`(?i)\bbearer\s+([A-Za-z0-9._~+/=-]{20,})`),
		regexp.MustCompile(`(?i)\b(?:api[_-]?key|access[_-]?token|auth[_-]?token|secret[_-]?key|client[_-]?secret)\b["']?\s*[:=]\s*["']?([A-Za-z0-9._~+/=-]{24,})`),
	}
	placeholderPattern     = regexp.MustCompile(`(?i)x{4,}|\*{3,}|\.{3,}|your|example|placeholder|dummy|<|>`)
	internalOutputPatterns = []*regexp.Regexp{
		// Private addresses are fine in proxy advice (127.0.0.1:7890); only
		// database/infrastructure ports and admin paths are refused.
		regexp.MustCompile(`(?i)\b(?:localhost|127\.0\.0\.1|0\.0\.0\.0|10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}):(?:5432|6379|3306|27017|9200|2379|8500|11211)\b`),
		regexp.MustCompile(`(?i)\b(?:localhost|127\.0\.0\.1|0\.0\.0\.0|10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3})(?::\d+)?/(?:api/v1/admin|admin|debug|metrics|internal|actuator)`),
		regexp.MustCompile(`(?i)\bselect\s+(?:\*|[a-z_]+(?:\s*,\s*[a-z_]+)+)\s+from\s+[a-z_]+`),
		regexp.MustCompile(`(?i)\b(?:insert\s+into\s+[a-z_]+|delete\s+from\s+[a-z_]+|drop\s+(?:table|database)\s+[a-z_]+|alter\s+table\s+[a-z_]+|update\s+[a-z_]+\s+set\s+[a-z_]+\s*=)`),
		regexp.MustCompile(`/etc/(?:passwd|shadow)`),
	}
	urlHostPattern   = regexp.MustCompile(`(?i)\bhttps?://([a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+)`)
	bareHostPattern  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9._/@-])((?:[a-z0-9](?:[a-z0-9-]*[a-z0-9])?\.)+(?:com|net|org|cn|cc|io|top|xyz|vip|me|co|app|dev|site|shop|link|info|ai|club|online|store|tech|pro|fun|icu|live|life|work|ltd|wang|ink|run|cloud|pw|tk|ml|ga|cf|gq|ru|de|uk|jp|kr|hk|tw|sg|us|biz|tv|to|gg|ly|im|fm))\b`)
	emailHostPattern = regexp.MustCompile(`(?i)[a-z0-9._%+-]@((?:[a-z0-9-]+\.)+[a-z]{2,})`)
)

// builtinTrustedHosts are vendor and tooling sites a support answer may
// legitimately point at. Site-specific domains come from the knowledge base,
// the docs manual, contact settings and tool results of the same request.
var builtinTrustedHosts = []string{
	"anthropic.com", "claude.ai", "claude.com", "openai.com", "chatgpt.com",
	"github.com", "ccswitch.io", "nodejs.org", "npmjs.com", "npmmirror.com",
	"python.org", "pypi.org", "tsinghua.edu.cn", "git-scm.com", "brew.sh",
	"visualstudio.com", "microsoft.com", "apple.com", "google.com", "x.ai", "qq.com",
}

// normalizeForGuard folds the common evasions of keyword filters: zero-width
// characters splitting a word and full-width ASCII ("ｉｇｎｏｒｅ").
func normalizeForGuard(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	for _, r := range text {
		switch {
		case r == '\u200b' || r == '\u200c' || r == '\u200d' || r == '\u2060' || r == '\ufeff':
			continue
		case r == '\u3000':
			b.WriteByte(' ')
		case r >= '\uff01' && r <= '\uff5e':
			b.WriteRune(r - 0xfee0)
		default:
			b.WriteRune(r)
		}
	}
	return strings.ToLower(strings.TrimSpace(b.String()))
}

// InspectInput runs before any model call.  A blocked result must be turned
// into SafeReply by the caller and must not be sent to the model.
func InspectInput(text string) GuardResult {
	normalized := normalizeForGuard(text)
	if normalized == "" {
		return GuardResult{Reason: GuardReasonEmpty}
	}
	if len([]rune(normalized)) > AIInputLimit {
		return GuardResult{Reason: GuardReasonTooLong}
	}
	for _, group := range []struct {
		patterns []*regexp.Regexp
		reason   string
	}{
		{promptInjectionPatterns, GuardReasonPromptInjection},
		{sensitiveRequestPatterns, GuardReasonSensitiveAsk},
		{illegalRequestPatterns, GuardReasonIllegalRequest},
		{offScopeTaskPatterns, GuardReasonOffScopeTask},
	} {
		for _, p := range group.patterns {
			if p.MatchString(normalized) {
				return GuardResult{Reason: group.reason}
			}
		}
	}
	return GuardResult{Allowed: true}
}

// InspectOutput rejects secrets, internal implementation details and quoted
// policy text even when a provider ignores the system policy.  Do not attempt
// to redact and send a partially trusted answer: use a fixed reply instead.
func InspectOutput(text string) GuardResult {
	if strings.TrimSpace(text) == "" {
		return GuardResult{Reason: GuardReasonEmpty}
	}
	for _, p := range secretOutputPatterns {
		if p.MatchString(text) {
			return GuardResult{Reason: GuardReasonSecretOutput}
		}
	}
	for _, p := range secretValuePatterns {
		for _, m := range p.FindAllStringSubmatch(text, -1) {
			value := m[0]
			if len(m) > 1 && m[1] != "" {
				value = m[1]
			}
			if !isPlaceholderSecret(value) {
				return GuardResult{Reason: GuardReasonSecretOutput}
			}
		}
	}
	for _, p := range internalOutputPatterns {
		if p.MatchString(text) {
			return GuardResult{Reason: GuardReasonInternalOutput}
		}
	}
	for _, canary := range promptCanaries {
		if strings.Contains(text, canary) {
			return GuardResult{Reason: GuardReasonInternalOutput}
		}
	}
	return GuardResult{Allowed: true}
}

// isPlaceholderSecret recognizes documentation stand-ins such as
// sk-xxxxxxxxxxxxxxxx or one character repeated: they carry no secret.
func isPlaceholderSecret(value string) bool {
	if placeholderPattern.MatchString(value) {
		return true
	}
	_, body, found := strings.Cut(value, "-")
	if !found {
		body = value
	}
	counts := map[rune]int{}
	top := 0
	for _, r := range body {
		counts[r]++
		if counts[r] > top {
			top = counts[r]
		}
	}
	return len(body) >= 8 && top*10 >= len(body)*6
}

// LinkHosts extracts the host names of links, bare domains and email
// addresses mentioned in text; IP addresses are skipped.
func LinkHosts(text string) []string {
	var hosts []string
	add := func(h string) {
		h = strings.TrimSuffix(strings.ToLower(h), ".")
		if h == "" || net.ParseIP(h) != nil {
			return
		}
		hosts = append(hosts, h)
	}
	for _, m := range urlHostPattern.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	for _, m := range bareHostPattern.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	for _, m := range emailHostPattern.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}
	return hosts
}

// CheckLinks rejects an answer that points users at a host nobody trusted:
// a group bot must never be talked into amplifying a phishing link, whether
// the push came from a crafted message or from text inside an image.
func CheckLinks(text string, trusted []string) GuardResult {
	allowed := append(append([]string{}, builtinTrustedHosts...), trusted...)
	for _, host := range LinkHosts(text) {
		if !hostAllowed(host, allowed) {
			return GuardResult{Reason: GuardReasonUntrustedLink}
		}
	}
	return GuardResult{Allowed: true}
}

// SafeReply is fixed text, not model-generated text.  It avoids confirming
// whether a particular secret or administrator record exists.
func SafeReply() string {
	return "抱歉，我只能协助处理公开的充值、技术支持和操作配置问题，无法提供受限信息或执行违规请求。"
}
