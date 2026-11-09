package invoices

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

var fixedNow = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

// newTestStore runs the real module migration on SQLite next to minimal host
// tables. SQLite only auto-assigns ids to INTEGER PRIMARY KEY columns, so the
// serial columns are rewritten for the fixture.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared&_pragma=foreign_keys(1)")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT);
		CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL, username TEXT NOT NULL DEFAULT '', deleted_at TEXT);
		CREATE TABLE payment_orders (
			id INTEGER PRIMARY KEY, user_id INTEGER NOT NULL, out_trade_no TEXT NOT NULL DEFAULT '', order_type TEXT NOT NULL,
			payment_type TEXT NOT NULL DEFAULT 'alipay', pay_amount NUMERIC NOT NULL, refund_amount NUMERIC NOT NULL DEFAULT 0,
			refund_requested_at TEXT, status TEXT NOT NULL, provider_snapshot TEXT,
			paid_at TEXT, completed_at TEXT, created_at TEXT
		);`)
	require.NoError(t, err)
	raw, err := os.ReadFile("../../migrations/sql/invoices/202611070001_invoices.sql")
	require.NoError(t, err)
	_, err = db.Exec(strings.ReplaceAll(string(raw), "BIGSERIAL PRIMARY KEY", "INTEGER PRIMARY KEY"))
	require.NoError(t, err)
	var flag string
	require.NoError(t, db.QueryRow(`SELECT value FROM settings WHERE key = 'custom_extensions.invoices.enabled'`).Scan(&flag))
	require.Equal(t, "false", flag, "new modules start switched off")
	return &Store{DB: db, Now: func() time.Time { return fixedNow }}
}

type orderFixture struct {
	id, user          int64
	orderType, status string
	amount, refund    string
	currency          string
	paidDaysAgo       int
	refundRequested   bool
}

func addOrder(t *testing.T, s *Store, o orderFixture) {
	t.Helper()
	if o.orderType == "" {
		o.orderType = "balance"
	}
	if o.status == "" {
		o.status = "COMPLETED"
	}
	if o.refund == "" {
		o.refund = "0"
	}
	var snapshot any
	if o.currency != "" {
		snapshot = `{"currency":"` + o.currency + `"}`
	}
	var requested any
	if o.refundRequested {
		requested = fixedNow
	}
	paid := fixedNow.AddDate(0, 0, -o.paidDaysAgo)
	_, err := s.DB.Exec(`INSERT INTO payment_orders (id, user_id, out_trade_no, order_type, pay_amount, refund_amount, refund_requested_at, status, provider_snapshot, paid_at, completed_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $10)`,
		o.id, o.user, "T"+decimal.NewFromInt(o.id).String(), o.orderType, o.amount, o.refund, requested, o.status, snapshot, paid)
	require.NoError(t, err)
}

func allowAll(t *testing.T, s *Store, minAmount string) Settings {
	t.Helper()
	settings, err := s.SaveSettings(context.Background(), Settings{
		MinAmount: decimal.RequireFromString(minAmount), AllowSpecial: true, OrderWindowDays: 365,
		AccessMode: AccessAll, ItemName: "信息技术服务费",
	})
	require.NoError(t, err)
	return settings
}

func addTitle(t *testing.T, s *Store, user int64, in TitleInput) Title {
	t.Helper()
	normalized, err := NormalizeTitle(in)
	require.NoError(t, err)
	title, err := s.SaveTitle(context.Background(), user, 0, normalized)
	require.NoError(t, err)
	return title
}

func orderIDs(orders []Order) []int64 {
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	return ids
}

func requireReason(t *testing.T, err error, reason string) {
	t.Helper()
	require.Error(t, err)
	require.Equal(t, reason, infraerrors.Reason(err), err.Error())
}

func TestInvoiceableOrdersOnlyCompletedCNYOrdersWithoutRefunds(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	settings := allowAll(t, s, "0")
	addOrder(t, s, orderFixture{id: 1, user: 7, amount: "100", paidDaysAgo: 1})
	addOrder(t, s, orderFixture{id: 2, user: 7, orderType: "subscription", amount: "39.90", currency: "CNY", paidDaysAgo: 2})
	addOrder(t, s, orderFixture{id: 3, user: 7, status: "PENDING", amount: "50"})
	addOrder(t, s, orderFixture{id: 4, user: 7, status: "PARTIALLY_REFUNDED", amount: "50", refund: "10"})
	addOrder(t, s, orderFixture{id: 5, user: 7, amount: "50", refundRequested: true})
	addOrder(t, s, orderFixture{id: 6, user: 7, amount: "50", currency: "USD"})
	addOrder(t, s, orderFixture{id: 7, user: 8, amount: "50"})
	addOrder(t, s, orderFixture{id: 8, user: 7, amount: "50", paidDaysAgo: 400})
	addOrder(t, s, orderFixture{id: 9, user: 7, orderType: "other", amount: "50"})
	addOrder(t, s, orderFixture{id: 10, user: 7, amount: "0"})

	orders, truncated, err := s.InvoiceableOrders(ctx, 7, settings)
	require.NoError(t, err)
	require.False(t, truncated)
	require.Equal(t, []int64{1, 2}, orderIDs(orders), "newest first, only on-site CNY completed orders")
	require.Equal(t, "39.9", orders[1].Amount.String())

	settings.OrderWindowDays = 0
	orders, _, err = s.InvoiceableOrders(ctx, 7, settings)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 8}, orderIDs(orders), "a zero window means no age limit")
}

func TestApplicationCapturesOrdersAndEnforcesThreshold(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	settings := allowAll(t, s, "100")
	addOrder(t, s, orderFixture{id: 1, user: 7, amount: "60", paidDaysAgo: 1})
	addOrder(t, s, orderFixture{id: 2, user: 7, amount: "50.50", paidDaysAgo: 2})
	addOrder(t, s, orderFixture{id: 3, user: 8, amount: "500", paidDaysAgo: 2})
	title := addTitle(t, s, 7, TitleInput{TitleType: TitleCompany, Name: "麋鹿云科技有限公司", TaxNo: "91310000ma1fl1xx2a", Email: "fin@example.com"})
	require.True(t, title.IsDefault, "the first title becomes the default")
	require.Equal(t, "91310000MA1FL1XX2A", title.TaxNo)

	_, err := s.CreateApplication(ctx, 7, ApplyInput{OrderIDs: []int64{1}, TitleID: title.ID, InvoiceType: InvoiceNormal}, title, settings)
	requireReason(t, err, "INVOICE_BELOW_THRESHOLD")

	_, err = s.CreateApplication(ctx, 7, ApplyInput{OrderIDs: []int64{1, 3}, TitleID: title.ID, InvoiceType: InvoiceNormal}, title, settings)
	requireReason(t, err, "INVOICE_ORDERS_UNAVAILABLE")

	app, err := s.CreateApplication(ctx, 7, ApplyInput{OrderIDs: []int64{1, 2}, TitleID: title.ID, InvoiceType: InvoiceNormal, Remark: "合并开票"}, title, settings)
	require.NoError(t, err)
	require.Equal(t, StatusPending, app.Status)
	require.Equal(t, "110.5", app.Amount.String())
	require.Equal(t, 2, app.OrderCount)
	require.Equal(t, "信息技术服务费", app.ItemName)
	require.Len(t, app.Orders, 2)

	orders, _, err := s.InvoiceableOrders(ctx, 7, settings)
	require.NoError(t, err)
	require.Empty(t, orders, "captured orders leave the invoiceable list")

	_, err = s.CreateApplication(ctx, 7, ApplyInput{OrderIDs: []int64{1, 2}, TitleID: title.ID, InvoiceType: InvoiceNormal}, title, settings)
	requireReason(t, err, "INVOICE_ORDERS_UNAVAILABLE")

	// The database itself refuses a second live capture of the same order.
	_, err = s.DB.Exec(`INSERT INTO invoice_applications (user_id, invoice_type, title_type, title_name, amount, order_count) VALUES (7, 'normal', 'personal', 'x', 1, 1)`)
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO invoice_application_orders (application_id, order_id, amount, active) VALUES (2, 1, 60, true)`)
	require.True(t, isUniqueViolation(err), "partial unique index must reject a second live capture: %v", err)

	// Later title edits do not touch the submitted copy.
	changed := TitleInput{TitleType: TitleCompany, Name: "新名称有限公司", TaxNo: "91310000MA1FL1XX2A"}
	_, err = s.SaveTitle(ctx, 7, title.ID, changed)
	require.NoError(t, err)
	reloaded, err := s.Application(ctx, app.ID, 7, false)
	require.NoError(t, err)
	require.Equal(t, "麋鹿云科技有限公司", reloaded.TitleName)

	_, err = s.Application(ctx, app.ID, 8, false)
	requireReason(t, err, "INVOICE_NOT_FOUND")

	require.NoError(t, s.Cancel(ctx, 7, app.ID))
	orders, _, err = s.InvoiceableOrders(ctx, 7, settings)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, orderIDs(orders), "cancelling releases the orders")
}

func TestApplicationReviewLifecycle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	settings := allowAll(t, s, "0")
	addOrder(t, s, orderFixture{id: 1, user: 7, amount: "88", paidDaysAgo: 1})
	addOrder(t, s, orderFixture{id: 2, user: 7, amount: "12", paidDaysAgo: 1})
	title := addTitle(t, s, 7, TitleInput{TitleType: TitlePersonal, Name: "张三"})

	app, err := s.CreateApplication(ctx, 7, ApplyInput{OrderIDs: []int64{1}, TitleID: title.ID, InvoiceType: InvoiceNormal}, title, settings)
	require.NoError(t, err)
	require.NoError(t, s.MarkProcessing(ctx, 1, app.ID))
	requireReason(t, s.Cancel(ctx, 7, app.ID), "INVOICE_STATE_CONFLICT")
	requireReason(t, s.MarkProcessing(ctx, 1, app.ID), "INVOICE_STATE_CONFLICT")

	_, _, err = s.File(ctx, app.ID, 7)
	requireReason(t, err, "INVOICE_FILE_NOT_FOUND")
	pdf := []byte("%PDF-1.7 test invoice")
	require.NoError(t, s.Issue(ctx, 1, app.ID, "24312000000012345678", "已发邮箱", &File{Name: "a.pdf", Data: pdf}))
	issued, err := s.Application(ctx, app.ID, 0, true)
	require.NoError(t, err)
	require.Equal(t, StatusIssued, issued.Status)
	require.Equal(t, "24312000000012345678", issued.InvoiceNo)
	require.Equal(t, "已发邮箱", issued.AdminNote)
	require.True(t, issued.HasFile)
	require.NotNil(t, issued.IssuedAt)

	file, invoiceNo, err := s.File(ctx, app.ID, 7)
	require.NoError(t, err)
	require.Equal(t, pdf, file.Data)
	require.Equal(t, "24312000000012345678", invoiceNo)
	_, _, err = s.File(ctx, app.ID, 8)
	requireReason(t, err, "INVOICE_FILE_NOT_FOUND")
	require.NoError(t, s.ReplaceFile(ctx, 1, app.ID, &File{Name: "b.pdf", Data: []byte("%PDF-1.4 v2")}))

	requireReason(t, s.Reject(ctx, 1, app.ID, "重复"), "INVOICE_STATE_CONFLICT")
	requireReason(t, s.Void(ctx, 1, 999, "x"), "INVOICE_NOT_FOUND")

	// A refund after issue raises an alert for administrators only.
	_, err = s.DB.Exec(`UPDATE payment_orders SET refund_amount = 88, status = 'REFUNDED' WHERE id = 1`)
	require.NoError(t, err)
	alerted, err := s.Application(ctx, app.ID, 0, true)
	require.NoError(t, err)
	require.True(t, alerted.RefundAlert)
	require.True(t, alerted.Orders[0].Refunded)
	owner, err := s.Application(ctx, app.ID, 7, false)
	require.NoError(t, err)
	require.False(t, owner.RefundAlert)
	require.False(t, owner.Orders[0].Refunded)
	require.Empty(t, owner.AdminNote)

	require.NoError(t, s.Void(ctx, 1, app.ID, "订单已退款，发票红冲"))
	voided, err := s.Application(ctx, app.ID, 0, true)
	require.NoError(t, err)
	require.Equal(t, StatusVoided, voided.Status)
	require.False(t, voided.RefundAlert, "closed applications no longer alert")
	var active int
	require.NoError(t, s.DB.QueryRow(`SELECT COUNT(*) FROM invoice_application_orders WHERE active`).Scan(&active))
	require.Zero(t, active)

	second, err := s.CreateApplication(ctx, 7, ApplyInput{OrderIDs: []int64{2}, TitleID: title.ID, InvoiceType: InvoiceNormal}, title, settings)
	require.NoError(t, err)
	require.NoError(t, s.Reject(ctx, 1, second.ID, "抬头有误"))
	rejected, err := s.Application(ctx, second.ID, 7, false)
	require.NoError(t, err)
	require.Equal(t, StatusRejected, rejected.Status)
	require.Equal(t, "抬头有误", rejected.RejectReason)

	items, total, err := s.ListApplications(ctx, ApplicationFilter{Status: StatusRejected})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, second.ID, items[0].ID)
	items, total, err = s.ListApplications(ctx, ApplicationFilter{Query: "2431200000"})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, app.ID, items[0].ID)
	counts, err := s.StatusCounts(ctx)
	require.NoError(t, err)
	require.Equal(t, map[string]int64{StatusVoided: 1, StatusRejected: 1}, counts)
}

func TestAccessAllowlistAndUserResolution(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	_, err := s.DB.Exec(`INSERT INTO users (id, email, username, deleted_at) VALUES (7, 'Alice@Example.com', 'alice', NULL), (8, 'gone@example.com', 'gone', '2026-01-01')`)
	require.NoError(t, err)
	settings, err := s.Settings(ctx)
	require.NoError(t, err)
	require.Equal(t, AccessAllowlist, settings.AccessMode, "access starts limited to named users")

	ok, err := s.HasAccess(ctx, 7, settings)
	require.NoError(t, err)
	require.False(t, ok)

	found, missing, err := s.ResolveUsers(ctx, []string{"alice@example.com", " 7 ", "gone@example.com", "8", "nobody@example.com"})
	require.NoError(t, err)
	require.Equal(t, []int64{7}, found)
	require.Equal(t, []string{"gone@example.com", "8", "nobody@example.com"}, missing)
	require.NoError(t, s.AddAccess(ctx, found, "财务客户"))

	ok, err = s.HasAccess(ctx, 7, settings)
	require.NoError(t, err)
	require.True(t, ok)
	entries, err := s.ListAccess(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "Alice@Example.com", entries[0].Email)
	require.Equal(t, "财务客户", entries[0].Note)

	require.NoError(t, s.RemoveAccess(ctx, 7))
	ok, err = s.HasAccess(ctx, 7, settings)
	require.NoError(t, err)
	require.False(t, ok)
	settings.AccessMode = AccessAll
	ok, err = s.HasAccess(ctx, 7, settings)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestTitlesKeepOneDefaultAndLimit(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	first := addTitle(t, s, 7, TitleInput{TitleType: TitlePersonal, Name: "张三"})
	second := addTitle(t, s, 7, TitleInput{TitleType: TitlePersonal, Name: "李四", IsDefault: true})
	titles, err := s.ListTitles(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, second.ID, titles[0].ID)
	require.True(t, titles[0].IsDefault)
	require.False(t, titles[1].IsDefault)

	require.NoError(t, s.DeleteTitle(ctx, 7, second.ID))
	titles, err = s.ListTitles(ctx, 7)
	require.NoError(t, err)
	require.Len(t, titles, 1)
	require.Equal(t, first.ID, titles[0].ID)
	require.True(t, titles[0].IsDefault, "deleting the default promotes the newest remaining title")
	requireReason(t, s.DeleteTitle(ctx, 8, first.ID), "INVOICE_TITLE_NOT_FOUND")

	for i := 1; i < MaxTitlesPerUser; i++ {
		addTitle(t, s, 7, TitleInput{TitleType: TitlePersonal, Name: "抬头"})
	}
	_, err = s.SaveTitle(ctx, 7, 0, TitleInput{TitleType: TitlePersonal, Name: "超出"})
	requireReason(t, err, "INVOICE_TITLE_LIMIT")
}
