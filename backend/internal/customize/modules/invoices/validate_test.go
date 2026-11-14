package invoices

import (
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTitleRules(t *testing.T) {
	company, err := NormalizeTitle(TitleInput{TitleType: TitleCompany, Name: " 麋鹿云 ", TaxNo: "9131 0000 MA1F L1XX 2A", Email: "a@b.cn"})
	require.NoError(t, err)
	require.Equal(t, "麋鹿云", company.Name)
	require.Equal(t, "91310000MA1FL1XX2A", company.TaxNo)

	personal, err := NormalizeTitle(TitleInput{TitleType: TitlePersonal, Name: "张三", Address: "上海", BankAccount: "6222"})
	require.NoError(t, err)
	require.Empty(t, personal.Address, "personal titles drop company details")
	require.Empty(t, personal.BankAccount)

	for name, in := range map[string]TitleInput{
		"unknown type":     {TitleType: "gov", Name: "x"},
		"empty name":       {TitleType: TitlePersonal, Name: "  "},
		"company no tax":   {TitleType: TitleCompany, Name: "x"},
		"bad tax":          {TitleType: TitleCompany, Name: "x", TaxNo: "123"},
		"bad email":        {TitleType: TitlePersonal, Name: "x", Email: "not-mail"},
		"display email":    {TitleType: TitlePersonal, Name: "x", Email: "A <a@b.cn>"},
		"bad phone":        {TitleType: TitleCompany, Name: "x", TaxNo: "91310000MA1FL1XX2A", Phone: "call me"},
		"long name":        {TitleType: TitlePersonal, Name: strings.Repeat("名", 101)},
		"bad bank account": {TitleType: TitleCompany, Name: "x", TaxNo: "91310000MA1FL1XX2A", BankAccount: "abc"},
	} {
		_, err := NormalizeTitle(in)
		require.Error(t, err, name)
		require.Equal(t, "INVOICE_TITLE_INVALID", infraerrors.Reason(err), name)
	}
}

func TestValidateTitleForSpecialInvoices(t *testing.T) {
	settings := Settings{AllowSpecial: true}
	full := Title{TitleType: TitleCompany, Name: "x", TaxNo: "91310000MA1FL1XX2A", Address: "上海", Phone: "021-1234567", BankName: "招商银行", BankAccount: "6222000011112222"}
	require.NoError(t, ValidateTitleFor(full, InvoiceSpecial, settings))
	require.NoError(t, ValidateTitleFor(Title{TitleType: TitlePersonal, Name: "张三"}, InvoiceNormal, settings))

	partial := full
	partial.BankAccount = ""
	require.Equal(t, "INVOICE_TITLE_INCOMPLETE", infraerrors.Reason(ValidateTitleFor(partial, InvoiceSpecial, settings)))
	require.Equal(t, "INVOICE_TITLE_INCOMPLETE", infraerrors.Reason(ValidateTitleFor(Title{TitleType: TitlePersonal}, InvoiceSpecial, settings)))
	require.Equal(t, "INVOICE_SPECIAL_DISABLED", infraerrors.Reason(ValidateTitleFor(full, InvoiceSpecial, Settings{})))
	require.Equal(t, "INVOICE_TYPE_INVALID", infraerrors.Reason(ValidateTitleFor(full, "paper", settings)))
}

func TestNormalizeApplyAndSettings(t *testing.T) {
	in, err := NormalizeApply(ApplyInput{OrderIDs: []int64{3, 3, 1}, TitleID: 5, InvoiceType: " normal "})
	require.NoError(t, err)
	require.Equal(t, []int64{3, 1}, in.OrderIDs)
	require.Equal(t, InvoiceNormal, in.InvoiceType)

	_, err = NormalizeApply(ApplyInput{TitleID: 5})
	require.Equal(t, "INVOICE_ORDERS_REQUIRED", infraerrors.Reason(err))
	_, err = NormalizeApply(ApplyInput{OrderIDs: []int64{1}})
	require.Equal(t, "INVOICE_TITLE_REQUIRED", infraerrors.Reason(err))
	_, err = NormalizeApply(ApplyInput{OrderIDs: []int64{-1}, TitleID: 1})
	require.Equal(t, "INVOICE_ORDERS_INVALID", infraerrors.Reason(err))
	many := make([]int64, MaxOrdersPerApplication+1)
	for i := range many {
		many[i] = int64(i + 1)
	}
	_, err = NormalizeApply(ApplyInput{OrderIDs: many, TitleID: 1})
	require.Equal(t, "INVOICE_ORDERS_TOO_MANY", infraerrors.Reason(err))

	settings, err := NormalizeSettings(Settings{MinAmount: decimal.RequireFromString("99.999"), AccessMode: AccessAll, OrderWindowDays: 0})
	require.NoError(t, err)
	require.Equal(t, "100", settings.MinAmount.String())
	for _, bad := range []Settings{
		{MinAmount: decimal.NewFromInt(-1), AccessMode: AccessAll},
		{AccessMode: "everyone"},
		{AccessMode: AccessAll, OrderWindowDays: -1},
		{AccessMode: AccessAll, Notice: strings.Repeat("字", 2001)},
	} {
		_, err := NormalizeSettings(bad)
		require.Equal(t, "INVOICE_SETTINGS_INVALID", infraerrors.Reason(err))
	}
}

func TestValidatePDF(t *testing.T) {
	name, err := ValidatePDF(" 发票.pdf ", []byte("%PDF-1.7\n..."))
	require.NoError(t, err)
	require.Equal(t, "发票.pdf", name)
	_, err = ValidatePDF("a.pdf", []byte("\xef\xbb\xbf%PDF-1.4"))
	require.NoError(t, err, "a short preamble before the header is allowed")
	_, err = ValidatePDF("a.pdf", []byte("<html>"))
	require.Equal(t, "INVOICE_FILE_INVALID", infraerrors.Reason(err))
	_, err = ValidatePDF("a.pdf", nil)
	require.Equal(t, "INVOICE_FILE_INVALID", infraerrors.Reason(err))
	_, err = ValidatePDF("a.pdf", make([]byte, MaxFileBytes+1))
	require.Equal(t, "INVOICE_FILE_TOO_LARGE", infraerrors.Reason(err))
}

func TestUserThresholdValidationAndMakeoutAdmission(t *testing.T) {
	value, err := ParseUserThreshold(nil)
	require.NoError(t, err)
	require.Nil(t, value)
	for _, raw := range []string{"", " ", "0", "60.50", "100000000"} {
		_, err := ParseUserThreshold(&raw)
		require.NoError(t, err, raw)
	}
	for _, raw := range []string{"-1", "abc", "1.001", "100000000.01"} {
		_, err := ParseUserThreshold(&raw)
		requireReason(t, err, "INVOICE_ACCESS_INVALID")
	}
	require.NoError(t, ValidateMakeout(Application{Status: StatusPending}))
	require.NoError(t, ValidateMakeout(Application{Status: StatusProcessing}))
	for _, status := range []string{StatusIssued, StatusRejected, StatusCancelled, StatusVoided} {
		requireReason(t, ValidateMakeout(Application{Status: status}), "INVOICE_STATE_CONFLICT")
	}
	requireReason(t, ValidateMakeout(Application{Status: StatusPending, RefundAlert: true}), "INVOICE_STATE_CONFLICT")
}
