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
// Memory is bounded by individual string values, never the whole JSONL file.
type piiJSON struct {
	ctx       context.Context
	r         *bufio.Reader
	w         io.Writer
	textDepth int
	inSchema  bool
	compact   bool
}

func redactJSON(ctx context.Context, w io.Writer, r io.Reader, compact bool) error {
	p := &piiJSON{ctx: ctx, r: bufio.NewReaderSize(r, 64<<10), w: w, compact: compact}
	if err := p.value("", "", 0, false); err != nil {
		return err
	}
	if err := p.space(); err != nil {
		return err
	}
	if _, err := p.r.Peek(1); err != io.EOF {
		return errors.New("unexpected trailing JSON data")
	}
	return ctx.Err()
}

func (p *piiJSON) write(b []byte) error { _, err := p.w.Write(b); return err }
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

func (p *piiJSON) value(key, parent string, depth int, mask bool) error {
	if err := p.ctx.Err(); err != nil {
		return err
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
	wasSchema := p.inSchema
	if key == "tools" || key == "input_schema" || key == "output_schema" || key == "json_schema" || key == "response_format" {
		p.inSchema = true
	}
	defer func() { p.inSchema = wasSchema }()
	mask = !p.inSchema && (mask || personalField(key, parent))
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
			if err := p.value(childKey, childParent, depth+1, mask); err != nil {
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
		if (opaqueProtocolField(key) || normalizePIIKey(key) == "id" && !personalContext(parent)) && !mask {
			return p.write(raw)
		}
		redacted := text
		if mask && text != "" {
			redacted = piiMask
		} else {
			redacted = redactText(text, p.textDepth)
		}
		if redacted == text {
			return p.write(raw)
		}
		var out bytes.Buffer
		writeJSONString(&out, redacted)
		return p.write(out.Bytes())
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

func redactText(text string, depth int) string {
	// Nested JSON and SSE data retain their exact framing. Recurse only into
	// actual JSON, not arbitrary code snippets or textual key names.
	if depth < 8 && json.Valid([]byte(text)) && (strings.HasPrefix(strings.TrimSpace(text), "{") || strings.HasPrefix(strings.TrimSpace(text), "[")) {
		var out bytes.Buffer
		// The nested parser uses the same rules; bound nesting via textDepth.
		if err := redactEmbeddedJSON(&out, text, depth+1); err == nil {
			return out.String()
		}
	}
	if depth < 8 && strings.Contains(text, "data:") {
		var out strings.Builder
		for _, line := range strings.SplitAfter(text, "\n") {
			if strings.HasPrefix(line, "data:") {
				prefix := len("data:")
				out.WriteString(line[:prefix])
				out.WriteString(redactText(line[prefix:], depth+1))
			} else {
				out.WriteString(redactPlainText(line))
			}
		}
		return out.String()
	}
	return redactPlainText(text)
}

func redactEmbeddedJSON(out io.Writer, text string, depth int) error {
	p := &piiJSON{ctx: context.Background(), r: bufio.NewReader(strings.NewReader(text)), w: out, textDepth: depth}
	if err := p.value("", "", 0, false); err != nil {
		return err
	}
	return p.space()
}

func redactPlainText(text string) string {
	text = emailPII.ReplaceAllString(text, piiMask)
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
			text = text[:start] + piiMask + text[end:]
		}
	}
	for _, re := range labeledPII {
		indices := re.FindAllStringSubmatchIndex(text, -1)
		for i := len(indices) - 1; i >= 0; i-- {
			if indices[i][0] > 0 && identifierByte(text[indices[i][0]-1]) {
				continue
			}
			start, end := indices[i][2], indices[i][3]
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
