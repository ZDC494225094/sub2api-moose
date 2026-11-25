package gatewaylogarchive

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const piiMask = "[REDACTED]"

// This parser runs ONLY in the upload worker. It preserves JSON keys, number
// spellings, whitespace and unchanged string tokens, including duplicate keys.
// Memory is bounded by individual string values, never the whole JSONL file;
// with a detection service, also by one pending batch (see lazyOut).
type piiJSON struct {
	ctx       context.Context
	r         *bufio.Reader
	out       *lazyOut
	env       redactEnv
	textDepth int
	inSchema  bool
	compact   bool
	// inTool marks a tool call, tool result or code subtree, left unchanged
	// with env.naturalOnly. lastString is the most recent string value read
	// at lastStringDepth, so an object can be classified by its type or role.
	inTool          bool
	lastString      string
	lastStringDepth int
}

// redactOptions selects what upload redaction applies.
type redactOptions struct {
	det         textDetector // nil without a detection service
	naturalOnly bool         // skip tool calls, tool results and code
	stats       *redactStats // this node's totals; the current upload's progress counts too
}

// redactEnv is shared by a document and the JSON nested in its strings.
type redactEnv struct {
	batch       *detectBatch // nil without a detection service, and in schemas
	naturalOnly bool
	counts      *docCounts
}

// redactJSON masks personal information in one JSON document. opts.det, when
// not nil, adds model findings to the built-in rules.
func redactJSON(ctx context.Context, w io.Writer, r io.Reader, compact bool, opts redactOptions) error {
	out := &lazyOut{w: w}
	counts := &docCounts{}
	p := &piiJSON{ctx: ctx, r: bufio.NewReaderSize(r, 64<<10), out: out, compact: compact, env: redactEnv{naturalOnly: opts.naturalOnly, counts: counts}}
	if opts.det != nil {
		p.env.batch = &detectBatch{ctx: ctx, det: opts.det, root: out, results: map[int][]detectSpan{}, counts: counts}
	}
	if err := p.value("", "", 0, false); err != nil {
		return err
	}
	if err := p.space(); err != nil {
		return err
	}
	if _, err := p.r.Peek(1); err != io.EOF {
		return errors.New("unexpected trailing JSON data")
	}
	if p.env.batch != nil {
		if err := p.env.batch.flush(); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	opts.stats.merge(counts)
	if up := progressFor(ctx); up != nil {
		up.redaction.merge(counts)
	}
	return nil
}

// maxHeldOutput bounds output held back behind pending detection: past it the
// batch is sent early so held output can be written.
const maxHeldOutput = 4 << 20

func (p *piiJSON) write(b []byte) error {
	if err := p.out.write(b); err != nil {
		return err
	}
	if p.env.batch != nil && p.out == p.env.batch.root && p.out.held > maxHeldOutput {
		return p.env.batch.flush()
	}
	return nil
}

// writeString writes the string token for a redacted value: the original
// token when nothing changed, else a re-encoded string. A value still waiting
// for the detection service is written when its batch returns.
func (p *piiJSON) writeString(raw []byte, text string, redacted lazy) error {
	encode := func(s string) []byte {
		if s == text {
			return raw
		}
		var out bytes.Buffer
		writeJSONString(&out, s)
		return out.Bytes()
	}
	if redacted.fn == nil {
		return p.write(encode(redacted.s))
	}
	p.out.deferred(func() []byte { return encode(redacted.get()) })
	return nil
}

func (p *piiJSON) space() error {
	for {
		b, err := p.r.Peek(1)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !strings.ContainsRune(" \t\r\n", rune(b[0])) {
			return nil
		}
		_, _ = p.r.ReadByte()
		if !p.compact {
			if err := p.write(b); err != nil {
				return err
			}
		}
	}
}
func (p *piiJSON) punctuation(want byte) error {
	if err := p.space(); err != nil {
		return err
	}
	b, err := p.r.ReadByte()
	if err != nil {
		return err
	}
	if b != want {
		return errors.New("invalid JSON punctuation")
	}
	return p.write([]byte{b})
}
func (p *piiJSON) stringToken() ([]byte, string, error) {
	first, err := p.r.ReadByte()
	if err != nil || first != '"' {
		return nil, "", errors.New("expected JSON string")
	}
	raw := []byte{'"'}
	escaped := false
	for {
		b, err := p.r.ReadByte()
		if err != nil {
			return nil, "", err
		}
		raw = append(raw, b)
		if !escaped && b == '"' {
			break
		}
		if escaped {
			escaped = false
		} else if b == '\\' {
			escaped = true
		}
		if len(raw)%(64<<10) == 0 && p.ctx.Err() != nil {
			return nil, "", p.ctx.Err()
		}
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, "", errors.New("invalid JSON string")
	}
	return raw, s, nil
}

func normalizePIIKey(key string) string {
	return strings.Map(func(r rune) rune {
		if r == '_' || r == '-' || r == ' ' {
			return -1
		}
		return unicode.ToLower(r)
	}, key)
}

func personalContext(key string) bool {
	switch normalizePIIKey(key) {
	case "person", "personalinfo", "personalinformation", "profile", "user", "customer", "contact", "recipient", "patient", "employee":
		return true
	}
	return false
}

func personalField(key, parent string) bool {
	switch normalizePIIKey(key) {
	case "fullname", "firstname", "lastname", "realname", "personname", "customername", "contactname", "recipientname", "patientname", "姓名", "真实姓名",
		"email", "emailaddress", "personalemail", "邮箱", "电子邮箱",
		"phone", "phonenumber", "mobile", "mobilephone", "mobilenumber", "telephone", "tel", "手机号", "手机号码", "联系电话",
		"idcard", "idcardnumber", "identitycard", "identitynumber", "nationalid", "ssn", "socialsecuritynumber", "passportnumber", "身份证", "身份证号", "护照号码",
		"bankcard", "bankcardnumber", "creditcardnumber", "debitcardnumber", "银行卡号",
		"homeaddress", "residentialaddress", "mailingaddress", "shippingaddress", "billingaddress", "streetaddress", "家庭住址", "收货地址", "详细地址",
		"dateofbirth", "birthdate", "dob", "出生日期":
		return true
	case "name", "address":
		return personalContext(parent)
	}
	return false
}

// Protocol signatures and references are opaque, not prose. Applying phone or
// email regexes to their bytes can invalidate a signature or break tool pairing.
func opaqueProtocolField(key string) bool {
	switch normalizePIIKey(key) {
	case "signature", "thoughtsignature", "encryptedcontent", "requestid", "callid", "tooluseid", "toolcallid", "previousresponseid", "correlationid", "segmentid", "nodeid":
		return true
	}
	return false
}

// toolField reports keys whose whole value is a tool call, tool result or
// code: OpenAI Chat tool_calls and function_call, Gemini parts, Bedrock
// Converse blocks and Anthropic input_json_delta fragments.
func toolField(key string) bool {
	switch normalizePIIKey(key) {
	case "toolcalls", "functioncall", "functionresponse", "executablecode", "codeexecutionresult", "tooluse", "toolresult", "partialjson":
		return true
	}
	return false
}

// toolObject reports whether an object's type or role marks it as tool
// content: Anthropic tool_use/tool_result blocks (also server and MCP tools),
// OpenAI Responses *_call and *_call_output items and their stream events,
// and Chat messages with role tool or function.
func toolObject(key, value string) bool {
	switch key {
	case "role":
		return value == "tool" || value == "function"
	case "type":
		switch {
		case value == "tool_use", value == "tool_result", value == "input_json_delta", value == "mcp_list_tools", value == "mcp_approval_request":
			return true
		case strings.HasSuffix(value, "_tool_use"), strings.HasSuffix(value, "_tool_result"):
			return true
		case strings.HasSuffix(value, "_call"), strings.HasSuffix(value, "_call_output"):
			return true
		case strings.HasPrefix(value, "response."):
			// response.function_call_arguments.delta, response.code_interpreter_call_code.delta, ...
			event := strings.Split(value, ".")
			return len(event) > 2 && strings.Contains(event[1], "_call")
		}
	}
	return false
}

func (p *piiJSON) value(key, parent string, depth int, mask bool) error {
	if err := p.ctx.Err(); err != nil {
		return err
	}
	if p.env.batch != nil && p.env.batch.err != nil {
		return p.env.batch.err
	}
	if depth > 10000 {
		return errors.New("JSON nesting exceeds limit")
	}
	if err := p.space(); err != nil {
		return err
	}
	peek, err := p.r.Peek(1)
	if err != nil {
		return err
	}
	wasSchema, wasTool := p.inSchema, p.inTool
	if key == "tools" || key == "input_schema" || key == "output_schema" || key == "json_schema" || key == "response_format" {
		p.inSchema = true
	}
	if p.env.naturalOnly && toolField(key) {
		p.inTool = true
	}
	defer func() { p.inSchema, p.inTool = wasSchema, wasTool }()
	mask = !p.inSchema && !p.inTool && (mask || personalField(key, parent))
	switch peek[0] {
	case '{', '[':
		open := peek[0]
		close := byte('}')
		if open == '[' {
			close = ']'
		}
		if err := p.punctuation(open); err != nil {
			return err
		}
		if err := p.space(); err != nil {
			return err
		}
		b, err := p.r.Peek(1)
		if err != nil {
			return err
		}
		if b[0] == close {
			return p.punctuation(close)
		}
		for {
			childKey, childParent := key, parent
			if open == '{' {
				raw, name, err := p.stringToken()
				if err != nil {
					return err
				}
				if err := p.write(raw); err != nil {
					return err
				}
				if err := p.punctuation(':'); err != nil {
					return err
				}
				childKey, childParent = name, key
			}
			p.lastStringDepth = -1
			if err := p.value(childKey, childParent, depth+1, mask); err != nil {
				return err
			}
			// Clients send type and role before the content, so the rest of
			// a tool object is left unchanged. Content before them is still
			// redacted.
			if p.env.naturalOnly && open == '{' && p.lastStringDepth == depth+1 && toolObject(childKey, p.lastString) {
				p.inTool = true
			}
			if err := p.space(); err != nil {
				return err
			}
			b, err := p.r.Peek(1)
			if err != nil {
				return err
			}
			if b[0] == close {
				return p.punctuation(close)
			}
			if err := p.punctuation(','); err != nil {
				return err
			}
			if err := p.space(); err != nil {
				return err
			}
		}
	case '"':
		raw, text, err := p.stringToken()
		if err != nil {
			return err
		}
		p.lastString, p.lastStringDepth = text, depth
		if p.inTool {
			if text != "" {
				p.env.counts.toolSkipped++
			}
			return p.write(raw)
		}
		if (opaqueProtocolField(key) || normalizePIIKey(key) == "id" && !personalContext(parent)) && !mask {
			return p.write(raw)
		}
		if mask && text != "" {
			if text != piiMask {
				p.env.counts.add(ruleField)
			}
			return p.writeString(raw, text, static(piiMask))
		}
		env := p.env
		if p.inSchema {
			env.batch = nil // tool schemas are not conversation content
		}
		return p.writeString(raw, text, redactText(text, p.textDepth, env))
	default:
		var raw []byte
		for {
			b, err := p.r.Peek(1)
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if strings.ContainsRune(",]} \r\n\t", rune(b[0])) {
				break
			}
			v, _ := p.r.ReadByte()
			raw = append(raw, v)
		}
		if len(raw) == 0 || !json.Valid(raw) {
			return errors.New("invalid JSON scalar")
		}
		if mask && string(raw) != "null" && string(raw) != "true" && string(raw) != "false" {
			// Retain the numeric type for explicit numeric personal fields.
			if string(raw) != "0" {
				p.env.counts.add(ruleField)
			}
			return p.write([]byte("0"))
		}
		return p.write(raw)
	}
}

var (
	emailPII  = regexp.MustCompile(`[a-zA-Z0-9][a-zA-Z0-9._%+\-]*@[a-zA-Z0-9](?:[a-zA-Z0-9-]*[a-zA-Z0-9])?(?:\.[a-zA-Z0-9-]+)*\.(?:[a-zA-Z]{2,}|xn--[a-zA-Z0-9-]{2,})\b`)
	mobilePII = regexp.MustCompile(`(?:\+?86[ -]?)?1[3-9][0-9]{9}`)
	idCardPII = regexp.MustCompile(`[1-9][0-9]{5}(?:19|20)[0-9]{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12][0-9]|3[01])[0-9]{3}[0-9Xx]`)
	// Labels make names, addresses, international phones and financial IDs
	// identifiable without guessing at ordinary names, numbers, code or URLs.
	labeledPII = []*regexp.Regexp{
		regexp.MustCompile(`(?:真实姓名|联系人姓名|姓名)[ \t]*[:：=][ \t]*([\p{Han}·]{2,12})`),
		regexp.MustCompile(`(?i:full[ _]name|real[ _]name)[ \t]*[:：=][ \t]*([A-Z][a-z]+(?:[ -][A-Z][a-z]+){1,3})`),
		regexp.MustCompile(`(?:家庭住址|收货地址|详细地址)[ \t]*[:：=][ \t]*([^\s,，;；。\n\r"<>]+)`),
		regexp.MustCompile(`(?i:home[ _]address|street[ _]address)[ \t]*[:：=][ \t]*([0-9]+[ \t]+[A-Za-z0-9 .#-]+)`),
		regexp.MustCompile(`(?i:身份证号(?:码)?|护照号(?:码)?|银行卡号|passport[ _]number|ssn|credit[ _]card(?:[ _]number)?)[ \t]*[:：=][ \t]*([A-Za-z0-9][A-Za-z0-9-]{4,24})`),
		regexp.MustCompile(`(?i:手机(?:号(?:码)?)?|联系电话|phone(?:[ _]number)?|mobile(?:[ _]number)?)[ \t]*[:：=][ \t]*(\+?[0-9][0-9 ()-]{5,22}[0-9])`),
	}
)

// labeledKinds names the replacement kind of each labeledPII pattern.
var labeledKinds = []string{rulePerson, rulePerson, ruleAddress, ruleAddress, ruleAccount, rulePhone}

func redactText(text string, depth int, env redactEnv) lazy {
	// Nested JSON and SSE data retain their exact framing. Recurse only into
	// actual JSON, not arbitrary code snippets or textual key names.
	if depth < 8 && json.Valid([]byte(text)) && (strings.HasPrefix(strings.TrimSpace(text), "{") || strings.HasPrefix(strings.TrimSpace(text), "[")) {
		// The nested parser uses the same rules; bound nesting via textDepth.
		if out, err := redactEmbeddedJSON(text, depth+1, env); err == nil {
			return out
		}
	}
	if depth < 8 && strings.Contains(text, "data:") {
		var parts []lazy
		for _, line := range strings.SplitAfter(text, "\n") {
			if strings.HasPrefix(line, "data:") {
				prefix := len("data:")
				parts = append(parts, static(line[:prefix]), redactText(line[prefix:], depth+1, env))
			} else {
				parts = append(parts, redactLeaf(line, env))
			}
		}
		return concatLazy(parts)
	}
	return redactLeaf(text, env)
}

// redactLeaf applies the built-in rules and, with a detection service, queues
// the result for the model. The model sees text whose rule matches are
// already masked.
func redactLeaf(text string, env redactEnv) lazy {
	local := redactPlainText(text, env.counts)
	if env.batch == nil || !detectable(local) {
		return static(local)
	}
	return env.batch.leaf(local)
}

func redactEmbeddedJSON(text string, depth int, env redactEnv) (lazy, error) {
	ctx := context.Background()
	if env.batch != nil {
		ctx = env.batch.ctx
	}
	// Rule counts are kept only if the text parses as a whole; otherwise the
	// caller redacts it as plain text, which counts again.
	parent, nested := env.counts, &docCounts{}
	env.counts = nested
	out := &lazyOut{}
	p := &piiJSON{ctx: ctx, r: bufio.NewReader(strings.NewReader(text)), out: out, env: env, textDepth: depth}
	if err := p.value("", "", 0, false); err != nil {
		return lazy{}, err
	}
	if err := p.space(); err != nil {
		return lazy{}, err
	}
	parent.mergeDoc(nested)
	return out.result(), nil
}

// redactPlainText applies the built-in rules; counts, when not nil, records
// each replacement.
func redactPlainText(text string, counts *docCounts) string {
	text = emailPII.ReplaceAllStringFunc(text, func(string) string {
		counts.add(ruleEmail)
		return piiMask
	})
	for _, re := range []*regexp.Regexp{idCardPII, mobilePII} {
		indices := re.FindAllStringIndex(text, -1)
		for i := len(indices) - 1; i >= 0; i-- {
			start, end := indices[i][0], indices[i][1]
			if start > 0 && identifierByte(text[start-1]) || end < len(text) && identifierByte(text[end]) {
				continue
			}
			if re == idCardPII && !validIDCard(text[start:end]) {
				continue
			}
			if re == idCardPII {
				counts.add(ruleIDCard)
			} else {
				counts.add(rulePhone)
			}
			text = text[:start] + piiMask + text[end:]
		}
	}
	for k, re := range labeledPII {
		indices := re.FindAllStringSubmatchIndex(text, -1)
		for i := len(indices) - 1; i >= 0; i-- {
			if indices[i][0] > 0 && identifierByte(text[indices[i][0]-1]) {
				continue
			}
			start, end := indices[i][2], indices[i][3]
			if text[start:end] == piiMask {
				continue // already replaced by an earlier rule
			}
			counts.add(labeledKinds[k])
			text = text[:start] + piiMask + text[end:]
		}
	}
	return text
}

func validIDCard(s string) bool {
	if len(s) != 18 {
		return false
	}
	if _, err := time.Parse("20060102", s[6:14]); err != nil {
		return false
	}
	weights := [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	sum := 0
	for i, weight := range weights {
		sum += int(s[i]-'0') * weight
	}
	return strings.EqualFold(s[17:], string("10X98765432"[sum%11]))
}

func identifierByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-'
}
