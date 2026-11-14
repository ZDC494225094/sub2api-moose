package invoices

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/shopspring/decimal"
)

// Store keeps all module state in its own tables and reads payment orders and
// users without writing to them. SQL stays portable (positional parameters,
// CURRENT_TIMESTAMP, no row locks): concurrency rests on conditional updates and
// the partial unique index on live application orders.
type Store struct {
	DB *sql.DB
	// Now is injectable for tests; nil means time.Now.
	Now func() time.Time
}

var errStoreUnavailable = errors.New("invoices: store unavailable")

func (s *Store) ready() error {
	if s == nil || s.DB == nil {
		return errStoreUnavailable
	}
	return nil
}

func (s *Store) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func notFound() error {
	return infraerrors.NotFound("INVOICE_NOT_FOUND", "开票申请不存在")
}

func stateConflict(message string) error {
	return infraerrors.Conflict("INVOICE_STATE_CONFLICT", message)
}

// ---------- settings ----------

func (s *Store) Settings(ctx context.Context) (Settings, error) {
	if err := s.ready(); err != nil {
		return Settings{}, err
	}
	var out Settings
	var updated dbTime
	err := s.DB.QueryRowContext(ctx, `SELECT min_amount, allow_special, order_window_days, access_mode, item_name, notice, updated_at
		FROM invoice_settings WHERE id = 1`).
		Scan(&out.MinAmount, &out.AllowSpecial, &out.OrderWindowDays, &out.AccessMode, &out.ItemName, &out.Notice, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return Settings{AllowSpecial: true, OrderWindowDays: 365, AccessMode: AccessAllowlist}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("invoices: load settings: %w", err)
	}
	out.UpdatedAt = updated.ptr()
	return out, nil
}

func (s *Store) SaveSettings(ctx context.Context, in Settings) (Settings, error) {
	if err := s.ready(); err != nil {
		return Settings{}, err
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO invoice_settings (id, min_amount, allow_special, order_window_days, access_mode, item_name, notice, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP)
		ON CONFLICT (id) DO UPDATE SET min_amount = EXCLUDED.min_amount, allow_special = EXCLUDED.allow_special,
			order_window_days = EXCLUDED.order_window_days, access_mode = EXCLUDED.access_mode,
			item_name = EXCLUDED.item_name, notice = EXCLUDED.notice, updated_at = CURRENT_TIMESTAMP`,
		in.MinAmount.StringFixed(2), in.AllowSpecial, in.OrderWindowDays, in.AccessMode, in.ItemName, in.Notice)
	if err != nil {
		return Settings{}, fmt.Errorf("invoices: save settings: %w", err)
	}
	return s.Settings(ctx)
}

// ---------- access ----------

// UserSettings overlays a user's threshold even when access is open to everyone.
// A NULL override follows the global rule; an explicit zero means no minimum.
func (s *Store) UserSettings(ctx context.Context, userID int64, settings Settings) (Settings, error) {
	if err := s.ready(); err != nil {
		return Settings{}, err
	}
	var amount decimal.NullDecimal
	err := s.DB.QueryRowContext(ctx, `SELECT min_amount FROM invoice_user_access WHERE user_id = $1`, userID).Scan(&amount)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Settings{}, fmt.Errorf("invoices: load user threshold: %w", err)
	}
	if amount.Valid {
		settings.MinAmount = amount.Decimal
	}
	return settings, nil
}

// HasAccess reports whether a user may use the invoice pages under the settings.
func (s *Store) HasAccess(ctx context.Context, userID int64, settings Settings) (bool, error) {
	if settings.AccessMode == AccessAll {
		return true, nil
	}
	if err := s.ready(); err != nil {
		return false, err
	}
	var one int
	err := s.DB.QueryRowContext(ctx, `SELECT 1 FROM invoice_user_access WHERE user_id = $1`, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("invoices: check access: %w", err)
	}
	return true, nil
}

func (s *Store) ListAccess(ctx context.Context) ([]AccessEntry, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT a.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''), a.note, a.created_at, a.min_amount
		FROM invoice_user_access a LEFT JOIN users u ON u.id = a.user_id
		ORDER BY a.created_at DESC, a.user_id DESC`)
	if err != nil {
		return nil, fmt.Errorf("invoices: list access: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := []AccessEntry{}
	for rows.Next() {
		var item AccessEntry
		var created dbTime
		var amount decimal.NullDecimal
		if err := rows.Scan(&item.UserID, &item.Email, &item.Username, &item.Note, &created, &amount); err != nil {
			return nil, fmt.Errorf("invoices: scan access: %w", err)
		}
		item.CreatedAt = created.T
		if amount.Valid {
			item.MinAmount = &amount.Decimal
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ResolveUsers turns e-mail addresses or numeric IDs into existing, not deleted
// user IDs. Unknown entries are returned so the administrator can correct them.
func (s *Store) ResolveUsers(ctx context.Context, refs []string) (found []int64, missing []string, err error) {
	if err := s.ready(); err != nil {
		return nil, nil, err
	}
	seen := map[int64]bool{}
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		var id int64
		if numeric, convErr := strconv.ParseInt(ref, 10, 64); convErr == nil && numeric > 0 {
			err = s.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE id = $1 AND deleted_at IS NULL`, numeric).Scan(&id)
		} else {
			err = s.DB.QueryRowContext(ctx, `SELECT id FROM users WHERE LOWER(email) = LOWER($1) AND deleted_at IS NULL`, ref).Scan(&id)
		}
		if errors.Is(err, sql.ErrNoRows) {
			missing = append(missing, ref)
			err = nil
			continue
		}
		if err != nil {
			return nil, nil, fmt.Errorf("invoices: resolve user: %w", err)
		}
		if !seen[id] {
			seen[id] = true
			found = append(found, id)
		}
	}
	return found, missing, nil
}

func (s *Store) AddAccess(ctx context.Context, userIDs []int64, note string) error {
	if err := s.ready(); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("invoices: begin add access: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range userIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO invoice_user_access (user_id, note) VALUES ($1, $2)
			ON CONFLICT (user_id) DO UPDATE SET note = EXCLUDED.note`, id, note); err != nil {
			return fmt.Errorf("invoices: add access: %w", err)
		}
	}
	return tx.Commit()
}

// UpdateAccess edits only an existing entry; it never grants access implicitly.
func (s *Store) UpdateAccess(ctx context.Context, userID int64, note string, minAmount *decimal.Decimal) error {
	if err := s.ready(); err != nil {
		return err
	}
	var amount any
	if minAmount != nil {
		amount = minAmount.StringFixed(2)
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE invoice_user_access SET note = $1, min_amount = $2 WHERE user_id = $3`, note, amount, userID)
	if err != nil {
		return fmt.Errorf("invoices: update access: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return infraerrors.NotFound("INVOICE_ACCESS_NOT_FOUND", "该用户不在开放名单中，请刷新后重试")
	}
	return nil
}

// RemoveAccess stops new applications from the user; existing applications and
// issued invoices stay visible to administrators.
func (s *Store) RemoveAccess(ctx context.Context, userID int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM invoice_user_access WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("invoices: remove access: %w", err)
	}
	return nil
}

// ---------- titles ----------

const titleColumns = `id, user_id, title_type, name, tax_no, address, phone, bank_name, bank_account, email, is_default, created_at, updated_at`

func scanTitle(row interface{ Scan(...any) error }) (Title, error) {
	var t Title
	var created, updated dbTime
	err := row.Scan(&t.ID, &t.UserID, &t.TitleType, &t.Name, &t.TaxNo, &t.Address, &t.Phone, &t.BankName, &t.BankAccount, &t.Email, &t.IsDefault, &created, &updated)
	t.CreatedAt, t.UpdatedAt = created.T, updated.T
	return t, err
}

func (s *Store) ListTitles(ctx context.Context, userID int64) ([]Title, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+titleColumns+` FROM invoice_titles WHERE user_id = $1 ORDER BY is_default DESC, id DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("invoices: list titles: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := []Title{}
	for rows.Next() {
		t, err := scanTitle(rows)
		if err != nil {
			return nil, fmt.Errorf("invoices: scan title: %w", err)
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (s *Store) Title(ctx context.Context, userID, id int64) (Title, error) {
	if err := s.ready(); err != nil {
		return Title{}, err
	}
	t, err := scanTitle(s.DB.QueryRowContext(ctx, `SELECT `+titleColumns+` FROM invoice_titles WHERE id = $1 AND user_id = $2`, id, userID))
	if errors.Is(err, sql.ErrNoRows) {
		return Title{}, infraerrors.NotFound("INVOICE_TITLE_NOT_FOUND", "发票抬头不存在")
	}
	if err != nil {
		return Title{}, fmt.Errorf("invoices: load title: %w", err)
	}
	return t, nil
}

// SaveTitle creates (id == 0) or updates a title. The first title, and any title
// saved as default, becomes the only default of the user.
func (s *Store) SaveTitle(ctx context.Context, userID, id int64, in TitleInput) (Title, error) {
	if err := s.ready(); err != nil {
		return Title{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Title{}, fmt.Errorf("invoices: begin save title: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoice_titles WHERE user_id = $1`, userID).Scan(&count); err != nil {
		return Title{}, fmt.Errorf("invoices: count titles: %w", err)
	}
	makeDefault := in.IsDefault || count == 0
	if id == 0 {
		if count >= MaxTitlesPerUser {
			return Title{}, badRequest("INVOICE_TITLE_LIMIT", "最多保存 10 个发票抬头")
		}
		err = tx.QueryRowContext(ctx, `INSERT INTO invoice_titles (user_id, title_type, name, tax_no, address, phone, bank_name, bank_account, email, is_default)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`,
			userID, in.TitleType, in.Name, in.TaxNo, in.Address, in.Phone, in.BankName, in.BankAccount, in.Email, makeDefault).Scan(&id)
		if err != nil {
			return Title{}, fmt.Errorf("invoices: create title: %w", err)
		}
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE invoice_titles SET title_type = $1, name = $2, tax_no = $3, address = $4, phone = $5,
			bank_name = $6, bank_account = $7, email = $8, is_default = CASE WHEN $9 THEN true ELSE is_default END, updated_at = CURRENT_TIMESTAMP
			WHERE id = $10 AND user_id = $11`,
			in.TitleType, in.Name, in.TaxNo, in.Address, in.Phone, in.BankName, in.BankAccount, in.Email, makeDefault, id, userID)
		if err != nil {
			return Title{}, fmt.Errorf("invoices: update title: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return Title{}, infraerrors.NotFound("INVOICE_TITLE_NOT_FOUND", "发票抬头不存在")
		}
	}
	if makeDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE invoice_titles SET is_default = false WHERE user_id = $1 AND id <> $2`, userID, id); err != nil {
			return Title{}, fmt.Errorf("invoices: reset default title: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Title{}, fmt.Errorf("invoices: commit title: %w", err)
	}
	return s.Title(ctx, userID, id)
}

// DeleteTitle removes a title; if it was the default, the newest remaining
// title becomes the default. Submitted applications keep their own copy.
func (s *Store) DeleteTitle(ctx context.Context, userID, id int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("invoices: begin delete title: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var wasDefault bool
	err = tx.QueryRowContext(ctx, `SELECT is_default FROM invoice_titles WHERE id = $1 AND user_id = $2`, id, userID).Scan(&wasDefault)
	if errors.Is(err, sql.ErrNoRows) {
		return infraerrors.NotFound("INVOICE_TITLE_NOT_FOUND", "发票抬头不存在")
	}
	if err != nil {
		return fmt.Errorf("invoices: load title: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM invoice_titles WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
		return fmt.Errorf("invoices: delete title: %w", err)
	}
	if wasDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE invoice_titles SET is_default = true
			WHERE id = (SELECT MAX(id) FROM invoice_titles WHERE user_id = $1)`, userID); err != nil {
			return fmt.Errorf("invoices: promote default title: %w", err)
		}
	}
	return tx.Commit()
}

// ---------- invoiceable orders ----------

// eligibleOrderSQL selects the user's orders that may be invoiced: on-site
// payment orders (balance recharges and plan purchases) that completed, were
// paid in CNY, never asked for or received a refund, fall inside the order
// window and are not part of a live application.
func eligibleOrderSQL(windowStart *time.Time, idPlaceholders string) string {
	var b strings.Builder
	b.WriteString(`SELECT o.id, o.out_trade_no, o.order_type, o.payment_type, o.pay_amount, COALESCE(o.paid_at, o.completed_at, o.created_at)
		FROM payment_orders o
		WHERE o.user_id = $1
			AND o.status = 'COMPLETED'
			AND o.order_type IN ('balance', 'subscription')
			AND o.pay_amount > 0
			AND o.refund_amount = 0
			AND o.refund_requested_at IS NULL
			AND UPPER(COALESCE(o.provider_snapshot ->> 'currency', 'CNY')) = 'CNY'
			AND NOT EXISTS (SELECT 1 FROM invoice_application_orders ao WHERE ao.order_id = o.id AND ao.active)`)
	if windowStart != nil {
		b.WriteString(` AND COALESCE(o.paid_at, o.completed_at, o.created_at) >= $2`)
	}
	if idPlaceholders != "" {
		b.WriteString(` AND o.id IN (` + idPlaceholders + `)`)
	}
	b.WriteString(` ORDER BY COALESCE(o.paid_at, o.completed_at, o.created_at) DESC, o.id DESC`)
	return b.String()
}

func (s *Store) windowStart(settings Settings) *time.Time {
	if settings.OrderWindowDays <= 0 {
		return nil
	}
	start := s.now().AddDate(0, 0, -settings.OrderWindowDays)
	return &start
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (s *Store) queryEligible(ctx context.Context, q queryer, userID int64, settings Settings, ids []int64, limit int) ([]Order, error) {
	args := []any{userID}
	start := s.windowStart(settings)
	if start != nil {
		args = append(args, *start)
	}
	placeholders := make([]string, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
		placeholders = append(placeholders, "$"+strconv.Itoa(len(args)))
	}
	query := eligibleOrderSQL(start, strings.Join(placeholders, ", "))
	if limit > 0 {
		query += ` LIMIT ` + strconv.Itoa(limit)
	}
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("invoices: list invoiceable orders: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := []Order{}
	for rows.Next() {
		var o Order
		var paid dbTime
		if err := rows.Scan(&o.ID, &o.OutTradeNo, &o.OrderType, &o.PaymentType, &o.Amount, &paid); err != nil {
			return nil, fmt.Errorf("invoices: scan order: %w", err)
		}
		o.Amount = o.Amount.Round(2)
		o.PaidAt = paid.ptr()
		items = append(items, o)
	}
	return items, rows.Err()
}

// InvoiceableOrders lists the newest invoiceable orders of a user. truncated is
// set when more orders exist than are listed.
func (s *Store) InvoiceableOrders(ctx context.Context, userID int64, settings Settings) (orders []Order, truncated bool, err error) {
	if err := s.ready(); err != nil {
		return nil, false, err
	}
	orders, err = s.queryEligible(ctx, s.DB, userID, settings, nil, MaxInvoiceableListed+1)
	if err != nil {
		return nil, false, err
	}
	if len(orders) > MaxInvoiceableListed {
		return orders[:MaxInvoiceableListed], true, nil
	}
	return orders, false, nil
}

// ---------- applications ----------

// CreateApplication captures the chosen orders under a copy of the title. Every
// order is re-checked inside the transaction; the partial unique index rejects a
// concurrent application that captured one of them first.
func (s *Store) CreateApplication(ctx context.Context, userID int64, in ApplyInput, title Title, settings Settings) (Application, error) {
	if err := s.ready(); err != nil {
		return Application{}, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Application{}, fmt.Errorf("invoices: begin apply: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Re-read the override inside the submission transaction so callers cannot
	// bypass a higher per-user threshold with stale/global settings.
	err = tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT min_amount FROM invoice_user_access WHERE user_id = $1), min_amount)
		FROM invoice_settings WHERE id = 1`, userID).Scan(&settings.MinAmount)
	if err != nil {
		return Application{}, fmt.Errorf("invoices: load application threshold: %w", err)
	}

	orders, err := s.queryEligible(ctx, tx, userID, settings, in.OrderIDs, 0)
	if err != nil {
		return Application{}, err
	}
	if len(orders) != len(in.OrderIDs) {
		return Application{}, infraerrors.Conflict("INVOICE_ORDERS_UNAVAILABLE", "部分订单当前不可开票（可能已退款、已在其他申请中或超出可开票时限），请刷新后重新选择")
	}
	total := decimal.Zero
	for _, o := range orders {
		total = total.Add(o.Amount)
	}
	total = total.Round(2)
	if total.LessThan(settings.MinAmount) {
		return Application{}, badRequest("INVOICE_BELOW_THRESHOLD", fmt.Sprintf("开票金额需不少于 ¥%s，当前所选 ¥%s，可合并更多订单后再申请", settings.MinAmount.StringFixed(2), total.StringFixed(2)))
	}

	var id int64
	err = tx.QueryRowContext(ctx, `INSERT INTO invoice_applications (user_id, invoice_type, title_type, title_name, tax_no, address, phone,
			bank_name, bank_account, email, item_name, amount, order_count, remark, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, 'pending') RETURNING id`,
		userID, in.InvoiceType, title.TitleType, title.Name, title.TaxNo, title.Address, title.Phone,
		title.BankName, title.BankAccount, title.Email, settings.ItemName, total.StringFixed(2), len(orders), in.Remark).Scan(&id)
	if err != nil {
		return Application{}, fmt.Errorf("invoices: create application: %w", err)
	}
	for _, o := range orders {
		_, err := tx.ExecContext(ctx, `INSERT INTO invoice_application_orders (application_id, order_id, out_trade_no, order_type, amount, paid_at, active)
			VALUES ($1, $2, $3, $4, $5, $6, true)`, id, o.ID, o.OutTradeNo, o.OrderType, o.Amount.StringFixed(2), o.PaidAt)
		if err != nil {
			if isUniqueViolation(err) {
				return Application{}, infraerrors.Conflict("INVOICE_ORDERS_UNAVAILABLE", "部分订单刚刚已被其他申请占用，请刷新后重新选择")
			}
			return Application{}, fmt.Errorf("invoices: capture order: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		if isUniqueViolation(err) {
			return Application{}, infraerrors.Conflict("INVOICE_ORDERS_UNAVAILABLE", "部分订单刚刚已被其他申请占用，请刷新后重新选择")
		}
		return Application{}, fmt.Errorf("invoices: commit application: %w", err)
	}
	return s.Application(ctx, id, userID, false)
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key") || strings.Contains(msg, "unique constraint")
}

const applicationColumns = `a.id, a.user_id, COALESCE(u.email, ''), a.invoice_type, a.title_type, a.title_name, a.tax_no, a.address, a.phone,
	a.bank_name, a.bank_account, a.email, a.item_name, a.amount, a.order_count, a.remark, a.status, a.invoice_no, a.reject_reason,
	a.admin_note, a.issued_at, a.closed_at, a.created_at, a.updated_at,
	CASE WHEN f.application_id IS NULL THEN 0 ELSE 1 END, COALESCE(f.file_name, ''),
	CASE WHEN a.status IN ('pending', 'processing', 'issued') AND EXISTS (
		SELECT 1 FROM invoice_application_orders ao JOIN payment_orders o ON o.id = ao.order_id
		WHERE ao.application_id = a.id AND ao.active
			AND (o.refund_amount > 0 OR o.refund_requested_at IS NOT NULL OR o.status <> 'COMPLETED')
	) THEN 1 ELSE 0 END`

const applicationFrom = ` FROM invoice_applications a
	LEFT JOIN users u ON u.id = a.user_id
	LEFT JOIN invoice_files f ON f.application_id = a.id`

func scanApplication(row interface{ Scan(...any) error }) (Application, error) {
	var a Application
	var issued, closed, created, updated dbTime
	var hasFile, refundAlert int
	err := row.Scan(&a.ID, &a.UserID, &a.UserEmail, &a.InvoiceType, &a.TitleType, &a.TitleName, &a.TaxNo, &a.Address, &a.Phone,
		&a.BankName, &a.BankAccount, &a.Email, &a.ItemName, &a.Amount, &a.OrderCount, &a.Remark, &a.Status, &a.InvoiceNo, &a.RejectReason,
		&a.AdminNote, &issued, &closed, &created, &updated, &hasFile, &a.FileName, &refundAlert)
	a.Amount = a.Amount.Round(2)
	a.IssuedAt, a.ClosedAt = issued.ptr(), closed.ptr()
	a.CreatedAt, a.UpdatedAt = created.T, updated.T
	a.HasFile = hasFile == 1
	a.RefundAlert = refundAlert == 1
	return a, err
}

// ListApplications returns one page of applications, newest first. A positive
// UserID restricts the list to that user's applications.
func (s *Store) ListApplications(ctx context.Context, f ApplicationFilter) ([]Application, int64, error) {
	if err := s.ready(); err != nil {
		return nil, 0, err
	}
	where := []string{"1 = 1"}
	args := []any{}
	add := func(cond string, values ...any) {
		for _, v := range values {
			args = append(args, v)
			cond = strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1)
		}
		where = append(where, cond)
	}
	if f.UserID > 0 {
		add("a.user_id = ?", f.UserID)
	}
	if f.Status != "" {
		add("a.status = ?", f.Status)
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		pattern := "%" + escapeLike(strings.ToLower(q)) + "%"
		if id, err := strconv.ParseInt(q, 10, 64); err == nil && id > 0 {
			add(`(a.id = ? OR a.user_id = ? OR LOWER(a.title_name) LIKE ? ESCAPE '\' OR LOWER(a.invoice_no) LIKE ? ESCAPE '\' OR LOWER(COALESCE(u.email, '')) LIKE ? ESCAPE '\')`, id, id, pattern, pattern, pattern)
		} else {
			add(`(LOWER(a.title_name) LIKE ? ESCAPE '\' OR LOWER(a.invoice_no) LIKE ? ESCAPE '\' OR LOWER(COALESCE(u.email, '')) LIKE ? ESCAPE '\' OR LOWER(a.tax_no) LIKE ? ESCAPE '\')`, pattern, pattern, pattern, pattern)
		}
	}
	condition := " WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM invoice_applications a LEFT JOIN users u ON u.id = a.user_id`+condition, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("invoices: count applications: %w", err)
	}
	page, size := f.Page, f.PageSize
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	query := `SELECT ` + applicationColumns + applicationFrom + condition +
		` ORDER BY a.id DESC LIMIT ` + strconv.Itoa(size) + ` OFFSET ` + strconv.Itoa((page-1)*size)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("invoices: list applications: %w", err)
	}
	defer func() { _ = rows.Close() }()
	items := []Application{}
	for rows.Next() {
		a, err := scanApplication(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("invoices: scan application: %w", err)
		}
		items = append(items, a)
	}
	return items, total, rows.Err()
}

// StatusCounts counts applications per status for the review tabs.
func (s *Store) StatusCounts(ctx context.Context) (map[string]int64, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT status, COUNT(*) FROM invoice_applications GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("invoices: count statuses: %w", err)
	}
	defer func() { _ = rows.Close() }()
	counts := map[string]int64{}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("invoices: scan status count: %w", err)
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

// Application loads one application with its orders. A positive userID limits
// the lookup to that owner. withRefunds marks orders refunded after capture.
func (s *Store) Application(ctx context.Context, id, userID int64, withRefunds bool) (Application, error) {
	if err := s.ready(); err != nil {
		return Application{}, err
	}
	query := `SELECT ` + applicationColumns + applicationFrom + ` WHERE a.id = $1`
	args := []any{id}
	if userID > 0 {
		query += ` AND a.user_id = $2`
		args = append(args, userID)
	}
	a, err := scanApplication(s.DB.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Application{}, notFound()
	}
	if err != nil {
		return Application{}, fmt.Errorf("invoices: load application: %w", err)
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT ao.order_id, ao.out_trade_no, ao.order_type, ao.amount, ao.paid_at,
			CASE WHEN o.id IS NOT NULL AND (o.refund_amount > 0 OR o.refund_requested_at IS NOT NULL OR o.status <> 'COMPLETED') THEN 1 ELSE 0 END
		FROM invoice_application_orders ao LEFT JOIN payment_orders o ON o.id = ao.order_id
		WHERE ao.application_id = $1 ORDER BY ao.paid_at DESC, ao.order_id DESC`, id)
	if err != nil {
		return Application{}, fmt.Errorf("invoices: load application orders: %w", err)
	}
	defer func() { _ = rows.Close() }()
	a.Orders = []ApplicationOrder{}
	for rows.Next() {
		var o ApplicationOrder
		var paid dbTime
		var refunded int
		if err := rows.Scan(&o.OrderID, &o.OutTradeNo, &o.OrderType, &o.Amount, &paid, &refunded); err != nil {
			return Application{}, fmt.Errorf("invoices: scan application order: %w", err)
		}
		o.Amount = o.Amount.Round(2)
		o.PaidAt = paid.ptr()
		o.Refunded = withRefunds && refunded == 1
		a.Orders = append(a.Orders, o)
	}
	if !withRefunds {
		a.RefundAlert = false
		a.AdminNote = ""
		a.UserEmail = ""
	}
	return a, rows.Err()
}

// transition moves an application from one of the allowed states to the next
// in one conditional update, so two reviewers cannot both act on it. Closing
// states release the captured orders in the same transaction.
func (s *Store) transition(ctx context.Context, id, userID int64, from []string, set string, args []any, release bool, after func(*sql.Tx) error) error {
	if err := s.ready(); err != nil {
		return err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("invoices: begin transition: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	all := append([]any{}, args...)
	all = append(all, id)
	where := ` WHERE id = $` + strconv.Itoa(len(all))
	if userID > 0 {
		all = append(all, userID)
		where += ` AND user_id = $` + strconv.Itoa(len(all))
	}
	states := make([]string, 0, len(from))
	for _, state := range from {
		all = append(all, state)
		states = append(states, "$"+strconv.Itoa(len(all)))
	}
	where += ` AND status IN (` + strings.Join(states, ", ") + `)`
	res, err := tx.ExecContext(ctx, `UPDATE invoice_applications SET `+set+`, updated_at = CURRENT_TIMESTAMP`+where, all...)
	if err != nil {
		return fmt.Errorf("invoices: update application: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var status string
		lookup := `SELECT status FROM invoice_applications WHERE id = $1`
		lookupArgs := []any{id}
		if userID > 0 {
			lookup += ` AND user_id = $2`
			lookupArgs = append(lookupArgs, userID)
		}
		if err := tx.QueryRowContext(ctx, lookup, lookupArgs...).Scan(&status); errors.Is(err, sql.ErrNoRows) {
			return notFound()
		}
		return stateConflict("申请状态已变化，请刷新后重试")
	}
	if release {
		if _, err := tx.ExecContext(ctx, `UPDATE invoice_application_orders SET active = false WHERE application_id = $1`, id); err != nil {
			return fmt.Errorf("invoices: release orders: %w", err)
		}
	}
	if after != nil {
		if err := after(tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Cancel withdraws a user's own pending application.
func (s *Store) Cancel(ctx context.Context, userID, id int64) error {
	return s.transition(ctx, id, userID, []string{StatusPending},
		`status = 'cancelled', closed_at = CURRENT_TIMESTAMP`, nil, true, nil)
}

// MarkProcessing records that an administrator started issuing the invoice;
// the user can no longer cancel it.
func (s *Store) MarkProcessing(ctx context.Context, adminID, id int64) error {
	return s.transition(ctx, id, 0, []string{StatusPending},
		`status = 'processing', handled_by = $1`, []any{adminID}, false, nil)
}

// Issue records the invoice number and, optionally, the PDF.
func (s *Store) Issue(ctx context.Context, adminID, id int64, invoiceNo, note string, file *File) error {
	return s.transition(ctx, id, 0, []string{StatusPending, StatusProcessing},
		`status = 'issued', invoice_no = $1, admin_note = $2, handled_by = $3, issued_at = CURRENT_TIMESTAMP`,
		[]any{invoiceNo, note, adminID}, false, func(tx *sql.Tx) error {
			if file == nil {
				return nil
			}
			return putFile(ctx, tx, id, file)
		})
}

// ReplaceFile attaches or replaces the PDF of an issued invoice.
func (s *Store) ReplaceFile(ctx context.Context, adminID, id int64, file *File) error {
	return s.transition(ctx, id, 0, []string{StatusIssued}, `handled_by = $1`, []any{adminID}, false, func(tx *sql.Tx) error {
		return putFile(ctx, tx, id, file)
	})
}

func putFile(ctx context.Context, tx *sql.Tx, id int64, file *File) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO invoice_files (application_id, file_name, size_bytes, data, uploaded_at)
		VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)
		ON CONFLICT (application_id) DO UPDATE SET file_name = EXCLUDED.file_name, size_bytes = EXCLUDED.size_bytes,
			data = EXCLUDED.data, uploaded_at = CURRENT_TIMESTAMP`, id, file.Name, len(file.Data), file.Data)
	if err != nil {
		return fmt.Errorf("invoices: store file: %w", err)
	}
	return nil
}

// Reject closes a pending or processing application and frees its orders.
func (s *Store) Reject(ctx context.Context, adminID, id int64, reason string) error {
	return s.transition(ctx, id, 0, []string{StatusPending, StatusProcessing},
		`status = 'rejected', reject_reason = $1, handled_by = $2, closed_at = CURRENT_TIMESTAMP`, []any{reason, adminID}, true, nil)
}

// Void marks an issued invoice as reversed (红冲/作废) and frees its orders.
func (s *Store) Void(ctx context.Context, adminID, id int64, reason string) error {
	return s.transition(ctx, id, 0, []string{StatusIssued},
		`status = 'voided', reject_reason = $1, handled_by = $2, closed_at = CURRENT_TIMESTAMP`, []any{reason, adminID}, true, nil)
}

// File returns the stored PDF. Owners only get it for issued applications.
func (s *Store) File(ctx context.Context, id, userID int64) (File, string, error) {
	if err := s.ready(); err != nil {
		return File{}, "", err
	}
	query := `SELECT f.file_name, f.data, a.invoice_no, a.status FROM invoice_files f
		JOIN invoice_applications a ON a.id = f.application_id WHERE f.application_id = $1`
	args := []any{id}
	if userID > 0 {
		query += ` AND a.user_id = $2`
		args = append(args, userID)
	}
	var f File
	var invoiceNo, status string
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(&f.Name, &f.Data, &invoiceNo, &status)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && userID > 0 && status != StatusIssued) {
		return File{}, "", infraerrors.NotFound("INVOICE_FILE_NOT_FOUND", "发票文件不存在")
	}
	if err != nil {
		return File{}, "", fmt.Errorf("invoices: load file: %w", err)
	}
	return f, invoiceNo, nil
}

func escapeLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}

// dbTime scans timestamps from PostgreSQL (time.Time) and SQLite (text).
type dbTime struct {
	T     time.Time
	Valid bool
}

var timeLayouts = []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02T15:04:05"}

func (t *dbTime) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*t = dbTime{}
		return nil
	case time.Time:
		*t = dbTime{T: v, Valid: true}
		return nil
	case []byte:
		return t.parse(string(v))
	case string:
		return t.parse(v)
	default:
		return fmt.Errorf("invoices: unsupported time value %T", value)
	}
}

func (t *dbTime) parse(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		*t = dbTime{}
		return nil
	}
	// time.Time.String() adds a monotonic clock suffix; drop it.
	if i := strings.Index(raw, " m="); i > 0 {
		raw = raw[:i]
	}
	for _, layout := range append(timeLayouts, "2006-01-02 15:04:05.999999999 -0700 MST") {
		if parsed, err := time.Parse(layout, raw); err == nil {
			*t = dbTime{T: parsed, Valid: true}
			return nil
		}
	}
	return fmt.Errorf("invoices: cannot parse time %q", raw)
}

func (t dbTime) ptr() *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.T
	return &v
}
