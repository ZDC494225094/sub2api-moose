package invoices

import (
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// Unified social credit codes have 18 characters; older taxpayer numbers have
// 15, 17 or 20. Only digits and upper-case letters occur.
var taxNoPattern = regexp.MustCompile(`^[0-9A-Z]{15,20}$`)
var phonePattern = regexp.MustCompile(`^[0-9+\-() ]{5,32}$`)
var bankAccountPattern = regexp.MustCompile(`^[0-9 \-]{6,64}$`)

func badRequest(reason, message string) error {
	return infraerrors.BadRequest(reason, message)
}

func clean(value string) string {
	return strings.TrimSpace(value)
}

func tooLong(value string, max int) bool {
	return utf8.RuneCountInString(value) > max
}

// NormalizeSettings validates administrator settings.
func NormalizeSettings(in Settings) (Settings, error) {
	out := in
	out.ItemName = clean(in.ItemName)
	out.Notice = strings.TrimSpace(in.Notice)
	if out.MinAmount.IsNegative() || out.MinAmount.GreaterThan(decimal.NewFromInt(100000000)) {
		return Settings{}, badRequest("INVOICE_SETTINGS_INVALID", "开票门槛需在 0 到 1 亿元之间")
	}
	out.MinAmount = out.MinAmount.Round(2)
	if out.OrderWindowDays < 0 || out.OrderWindowDays > 3650 {
		return Settings{}, badRequest("INVOICE_SETTINGS_INVALID", "可开票订单时限需在 0 到 3650 天之间（0 表示不限）")
	}
	if out.AccessMode != AccessAll && out.AccessMode != AccessAllowlist {
		return Settings{}, badRequest("INVOICE_SETTINGS_INVALID", "开放范围只能是全部用户或指定用户")
	}
	if tooLong(out.ItemName, 100) {
		return Settings{}, badRequest("INVOICE_SETTINGS_INVALID", "开票内容最多 100 个字符")
	}
	if tooLong(out.Notice, 2000) {
		return Settings{}, badRequest("INVOICE_SETTINGS_INVALID", "开票说明最多 2000 个字符")
	}
	return out, nil
}

// NormalizeTitle validates a title. Personal titles keep only the name, an
// optional taxpayer number and the receiving email; company titles require a
// taxpayer number. The bank and address fields matter for special invoices,
// which ValidateTitleFor checks at application time.
func NormalizeTitle(in TitleInput) (TitleInput, error) {
	out := TitleInput{
		TitleType:   clean(in.TitleType),
		Name:        clean(in.Name),
		TaxNo:       strings.ToUpper(strings.ReplaceAll(clean(in.TaxNo), " ", "")),
		Address:     clean(in.Address),
		Phone:       clean(in.Phone),
		BankName:    clean(in.BankName),
		BankAccount: clean(in.BankAccount),
		Email:       clean(in.Email),
		IsDefault:   in.IsDefault,
	}
	if out.TitleType != TitlePersonal && out.TitleType != TitleCompany {
		return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "抬头类型只能是个人或企业")
	}
	if out.Name == "" || tooLong(out.Name, 100) {
		return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "抬头名称需为 1 到 100 个字符")
	}
	if out.TitleType == TitleCompany && out.TaxNo == "" {
		return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "企业抬头必须填写纳税人识别号")
	}
	if out.TaxNo != "" && !taxNoPattern.MatchString(out.TaxNo) {
		return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "纳税人识别号应为 15 到 20 位数字或大写字母")
	}
	if out.TitleType == TitlePersonal {
		// Personal titles never carry company registration details.
		out.Address, out.Phone, out.BankName, out.BankAccount = "", "", "", ""
	}
	if tooLong(out.Address, 200) {
		return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "注册地址最多 200 个字符")
	}
	if out.Phone != "" && !phonePattern.MatchString(out.Phone) {
		return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "注册电话格式不正确")
	}
	if tooLong(out.BankName, 100) {
		return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "开户银行最多 100 个字符")
	}
	if out.BankAccount != "" && !bankAccountPattern.MatchString(out.BankAccount) {
		return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "银行账号格式不正确")
	}
	if out.Email != "" {
		if tooLong(out.Email, 255) {
			return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "接收邮箱过长")
		}
		addr, err := mail.ParseAddress(out.Email)
		if err != nil || addr.Address != out.Email {
			return TitleInput{}, badRequest("INVOICE_TITLE_INVALID", "接收邮箱格式不正确")
		}
	}
	return out, nil
}

// ValidateTitleFor checks that a title can be used for the requested invoice type.
func ValidateTitleFor(title Title, invoiceType string, settings Settings) error {
	switch invoiceType {
	case InvoiceNormal:
		return nil
	case InvoiceSpecial:
		if !settings.AllowSpecial {
			return badRequest("INVOICE_SPECIAL_DISABLED", "当前不支持申请增值税专用发票")
		}
		if title.TitleType != TitleCompany {
			return badRequest("INVOICE_TITLE_INCOMPLETE", "增值税专用发票只能开给企业抬头")
		}
		if title.Address == "" || title.Phone == "" || title.BankName == "" || title.BankAccount == "" {
			return badRequest("INVOICE_TITLE_INCOMPLETE", "增值税专用发票需要完整的注册地址、注册电话、开户银行和银行账号")
		}
		return nil
	default:
		return badRequest("INVOICE_TYPE_INVALID", "发票类型只能是普通发票或专用发票")
	}
}

// NormalizeApply validates the shape of an application request; ownership,
// eligibility and the amount threshold are checked against the database.
func NormalizeApply(in ApplyInput) (ApplyInput, error) {
	out := ApplyInput{TitleID: in.TitleID, InvoiceType: clean(in.InvoiceType), Remark: clean(in.Remark)}
	if out.TitleID <= 0 {
		return ApplyInput{}, badRequest("INVOICE_TITLE_REQUIRED", "请选择发票抬头")
	}
	if tooLong(out.Remark, 200) {
		return ApplyInput{}, badRequest("INVOICE_REMARK_TOO_LONG", "备注最多 200 个字符")
	}
	seen := make(map[int64]struct{}, len(in.OrderIDs))
	for _, id := range in.OrderIDs {
		if id <= 0 {
			return ApplyInput{}, badRequest("INVOICE_ORDERS_INVALID", "订单编号无效")
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out.OrderIDs = append(out.OrderIDs, id)
	}
	if len(out.OrderIDs) == 0 {
		return ApplyInput{}, badRequest("INVOICE_ORDERS_REQUIRED", "请至少选择一笔订单")
	}
	if len(out.OrderIDs) > MaxOrdersPerApplication {
		return ApplyInput{}, badRequest("INVOICE_ORDERS_TOO_MANY", "一次最多选择 200 笔订单")
	}
	return out, nil
}

// NormalizeInvoiceNo validates the number an administrator records on issue.
func NormalizeInvoiceNo(value string) (string, error) {
	value = clean(value)
	if value == "" || tooLong(value, 64) {
		return "", badRequest("INVOICE_NO_INVALID", "请填写发票号码（最多 64 个字符）")
	}
	return value, nil
}

// NormalizeReason validates a rejection or void reason.
func NormalizeReason(value string) (string, error) {
	value = clean(value)
	if value == "" || tooLong(value, 500) {
		return "", badRequest("INVOICE_REASON_INVALID", "请填写原因（最多 500 个字符）")
	}
	return value, nil
}

// ValidatePDF checks an uploaded invoice document.
func ValidatePDF(name string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", badRequest("INVOICE_FILE_INVALID", "发票文件为空")
	}
	if len(data) > MaxFileBytes {
		return "", badRequest("INVOICE_FILE_TOO_LARGE", "发票文件不能超过 10MB")
	}
	// PDF readers accept the header anywhere in the first 1024 bytes.
	if !strings.Contains(string(data[:min(len(data), 1024)]), "%PDF-") {
		return "", badRequest("INVOICE_FILE_INVALID", "只支持上传 PDF 格式的电子发票")
	}
	name = clean(name)
	if tooLong(name, 200) {
		name = string([]rune(name)[:200])
	}
	return name, nil
}
