package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type couponTemplateRepository struct {
	sql sqlExecutor
}

type userCouponRepository struct {
	sql sqlExecutor
}

type paymentOrderDiscountRepository struct {
	sql sqlExecutor
}

func NewCouponTemplateRepository(db *sql.DB) service.CouponTemplateRepository {
	return &couponTemplateRepository{sql: db}
}

func NewUserCouponRepository(db *sql.DB) service.UserCouponRepository {
	return &userCouponRepository{sql: db}
}

func NewPaymentOrderDiscountRepository(db *sql.DB) service.PaymentOrderDiscountRepository {
	return &paymentOrderDiscountRepository{sql: db}
}

func (r *couponTemplateRepository) Create(ctx context.Context, input *service.CouponTemplate) error {
	query := `
INSERT INTO coupon_templates (
  name, description, scope, discount_amount, threshold_amount, valid_days,
  valid_from, valid_until, status, notes
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, created_at, updated_at`
	return scanSingleRow(ctx, r.sql, query, []any{
		input.Name, input.Description, input.Scope, input.DiscountAmount, input.ThresholdAmount,
		input.ValidDays, input.ValidFrom, input.ValidUntil, input.Status, input.Notes,
	}, &input.ID, &input.CreatedAt, &input.UpdatedAt)
}

func (r *couponTemplateRepository) Update(ctx context.Context, item *service.CouponTemplate) error {
	query := `
UPDATE coupon_templates
SET name = $2,
    description = $3,
    scope = $4,
    discount_amount = $5,
    threshold_amount = $6,
    valid_days = $7,
    valid_from = $8,
    valid_until = $9,
    status = $10,
    notes = $11,
    updated_at = NOW()
WHERE id = $1
RETURNING updated_at`
	return scanSingleRow(ctx, r.sql, query, []any{
		item.ID, item.Name, item.Description, item.Scope, item.DiscountAmount, item.ThresholdAmount,
		item.ValidDays, item.ValidFrom, item.ValidUntil, item.Status, item.Notes,
	}, &item.UpdatedAt)
}

func (r *couponTemplateRepository) GetByID(ctx context.Context, id int64) (*service.CouponTemplate, error) {
	query := `
SELECT id, name, description, scope, discount_amount, threshold_amount, valid_days,
       valid_from, valid_until, status, notes, created_at, updated_at
FROM coupon_templates
WHERE id = $1`
	item := &service.CouponTemplate{}
	var validDays sql.NullInt64
	var validFrom sql.NullTime
	var validUntil sql.NullTime
	err := scanSingleRow(ctx, r.sql, query, []any{id},
		&item.ID, &item.Name, &item.Description, &item.Scope, &item.DiscountAmount, &item.ThresholdAmount,
		&validDays, &validFrom, &validUntil, &item.Status, &item.Notes, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrCouponTemplateNotFound
		}
		return nil, err
	}
	if validDays.Valid {
		v := int(validDays.Int64)
		item.ValidDays = &v
	}
	if validFrom.Valid {
		v := validFrom.Time.UTC()
		item.ValidFrom = &v
	}
	if validUntil.Valid {
		v := validUntil.Time.UTC()
		item.ValidUntil = &v
	}
	return item, nil
}

func (r *couponTemplateRepository) List(ctx context.Context, params pagination.PaginationParams, filter service.CouponTemplateListFilter) ([]service.CouponTemplate, *pagination.PaginationResult, error) {
	exec := r.sql
	where := []string{"1=1"}
	args := make([]any, 0, 4)
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Scope != "" {
		args = append(args, filter.Scope)
		where = append(where, fmt.Sprintf("scope = $%d", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+strings.TrimSpace(filter.Search)+"%")
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR description ILIKE $%d)", len(args), len(args)))
	}
	whereClause := strings.Join(where, " AND ")

	countQuery := "SELECT COUNT(*) FROM coupon_templates WHERE " + whereClause
	var total int64
	if err := scanSingleRow(ctx, exec, countQuery, args, &total); err != nil {
		return nil, nil, err
	}

	orderBy := "ORDER BY created_at DESC, id DESC"
	listQuery := `
SELECT id, name, description, scope, discount_amount, threshold_amount, valid_days,
       valid_from, valid_until, status, notes, created_at, updated_at
FROM coupon_templates
WHERE ` + whereClause + ` ` + orderBy + fmt.Sprintf(" LIMIT %d OFFSET %d", params.Limit(), params.Offset())
	rows, err := exec.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.CouponTemplate, 0)
	for rows.Next() {
		var item service.CouponTemplate
		var validDays sql.NullInt64
		var validFrom sql.NullTime
		var validUntil sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Scope, &item.DiscountAmount, &item.ThresholdAmount, &validDays, &validFrom, &validUntil, &item.Status, &item.Notes, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, nil, err
		}
		if validDays.Valid {
			v := int(validDays.Int64)
			item.ValidDays = &v
		}
		if validFrom.Valid {
			v := validFrom.Time.UTC()
			item.ValidFrom = &v
		}
		if validUntil.Valid {
			v := validUntil.Time.UTC()
			item.ValidUntil = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *userCouponRepository) Create(ctx context.Context, input *service.UserCoupon) error {
	query := `
INSERT INTO user_coupons (
  template_id, user_id, coupon_code, source_type, source_ref_id, scope, discount_amount,
  threshold_amount, valid_from, valid_until, status
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id, created_at, updated_at`
	return scanSingleRow(ctx, r.sql, query, []any{
		input.TemplateID, input.UserID, input.CouponCode, input.SourceType, input.SourceRefID,
		input.Scope, input.DiscountAmount, input.ThresholdAmount, input.ValidFrom, input.ValidUntil, input.Status,
	}, &input.ID, &input.CreatedAt, &input.UpdatedAt)
}

func (r *userCouponRepository) GetByID(ctx context.Context, id int64) (*service.UserCoupon, error) {
	return r.getBy(ctx, "id = $1", id, false)
}

func (r *userCouponRepository) GetByIDForUpdate(ctx context.Context, id int64) (*service.UserCoupon, error) {
	return r.getBy(ctx, "id = $1", id, true)
}

func (r *userCouponRepository) GetByCode(ctx context.Context, code string) (*service.UserCoupon, error) {
	return r.getBy(ctx, "coupon_code = $1", code, false)
}

func (r *userCouponRepository) getBy(ctx context.Context, predicate string, arg any, forUpdate bool) (*service.UserCoupon, error) {
	query := `
SELECT id, template_id, user_id, coupon_code, source_type, source_ref_id, scope, discount_amount,
       threshold_amount, valid_from, valid_until, status, reserved_order_id, reserved_at,
       used_order_id, used_at, created_at, updated_at
FROM user_coupons
WHERE ` + predicate
	if forUpdate {
		query += ` FOR UPDATE`
	}
	item := &service.UserCoupon{}
	var sourceRefID sql.NullInt64
	var validFrom sql.NullTime
	var validUntil sql.NullTime
	var reservedOrderID sql.NullInt64
	var reservedAt sql.NullTime
	var usedOrderID sql.NullInt64
	var usedAt sql.NullTime
	err := scanSingleRow(ctx, r.sql, query, []any{arg},
		&item.ID, &item.TemplateID, &item.UserID, &item.CouponCode, &item.SourceType, &sourceRefID, &item.Scope,
		&item.DiscountAmount, &item.ThresholdAmount, &validFrom, &validUntil, &item.Status,
		&reservedOrderID, &reservedAt, &usedOrderID, &usedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrUserCouponNotFound
		}
		return nil, err
	}
	if sourceRefID.Valid {
		v := sourceRefID.Int64
		item.SourceRefID = &v
	}
	if validFrom.Valid {
		v := validFrom.Time.UTC()
		item.ValidFrom = &v
	}
	if validUntil.Valid {
		v := validUntil.Time.UTC()
		item.ValidUntil = &v
	}
	if reservedOrderID.Valid {
		v := reservedOrderID.Int64
		item.ReservedOrderID = &v
	}
	if reservedAt.Valid {
		v := reservedAt.Time.UTC()
		item.ReservedAt = &v
	}
	if usedOrderID.Valid {
		v := usedOrderID.Int64
		item.UsedOrderID = &v
	}
	if usedAt.Valid {
		v := usedAt.Time.UTC()
		item.UsedAt = &v
	}
	return item, nil
}

func (r *userCouponRepository) ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams, filter service.UserCouponListFilter) ([]service.UserCoupon, *pagination.PaginationResult, error) {
	exec := r.sql
	where := []string{"user_id = $1"}
	args := []any{userID}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Scope != "" {
		args = append(args, filter.Scope)
		where = append(where, fmt.Sprintf("scope = $%d", len(args)))
	}
	whereClause := strings.Join(where, " AND ")
	var total int64
	if err := scanSingleRow(ctx, exec, "SELECT COUNT(*) FROM user_coupons WHERE "+whereClause, args, &total); err != nil {
		return nil, nil, err
	}
	listQuery := `
SELECT id, template_id, user_id, coupon_code, source_type, source_ref_id, scope, discount_amount,
       threshold_amount, valid_from, valid_until, status, reserved_order_id, reserved_at,
       used_order_id, used_at, created_at, updated_at
FROM user_coupons
WHERE ` + whereClause + fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT %d OFFSET %d", params.Limit(), params.Offset())
	rows, err := exec.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.UserCoupon, 0)
	for rows.Next() {
		var item service.UserCoupon
		var sourceRefID sql.NullInt64
		var validFrom sql.NullTime
		var validUntil sql.NullTime
		var reservedOrderID sql.NullInt64
		var reservedAt sql.NullTime
		var usedOrderID sql.NullInt64
		var usedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.TemplateID, &item.UserID, &item.CouponCode, &item.SourceType, &sourceRefID, &item.Scope, &item.DiscountAmount, &item.ThresholdAmount, &validFrom, &validUntil, &item.Status, &reservedOrderID, &reservedAt, &usedOrderID, &usedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, nil, err
		}
		if sourceRefID.Valid {
			v := sourceRefID.Int64
			item.SourceRefID = &v
		}
		if validFrom.Valid {
			v := validFrom.Time.UTC()
			item.ValidFrom = &v
		}
		if validUntil.Valid {
			v := validUntil.Time.UTC()
			item.ValidUntil = &v
		}
		if reservedOrderID.Valid {
			v := reservedOrderID.Int64
			item.ReservedOrderID = &v
		}
		if reservedAt.Valid {
			v := reservedAt.Time.UTC()
			item.ReservedAt = &v
		}
		if usedOrderID.Valid {
			v := usedOrderID.Int64
			item.UsedOrderID = &v
		}
		if usedAt.Valid {
			v := usedAt.Time.UTC()
			item.UsedAt = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *userCouponRepository) ReserveForOrder(ctx context.Context, couponID int64, orderID int64, reservedAt time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
UPDATE user_coupons
SET status = $3,
    reserved_order_id = $2,
    reserved_at = $4,
    updated_at = NOW()
WHERE id = $1 AND status = 'unused'`, couponID, orderID, service.UserCouponStatusReserved, reservedAt.UTC())
	return err
}

func (r *userCouponRepository) ReleaseReservationByOrderID(ctx context.Context, orderID int64, releasedAt time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
UPDATE user_coupons
SET status = 'unused',
    reserved_order_id = NULL,
    reserved_at = NULL,
    updated_at = NOW()
WHERE reserved_order_id = $1 AND status = 'reserved'`, orderID)
	_ = releasedAt
	return err
}

func (r *userCouponRepository) MarkUsedByOrderID(ctx context.Context, orderID int64, usedAt time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
UPDATE user_coupons
SET status = 'used',
    used_order_id = $1,
    used_at = $2,
    reserved_order_id = NULL,
    reserved_at = NULL,
    updated_at = NOW()
WHERE reserved_order_id = $1 AND status = 'reserved'`, orderID, usedAt.UTC())
	return err
}

func (r *paymentOrderDiscountRepository) Create(ctx context.Context, item *service.PaymentOrderDiscount) error {
	query := `
INSERT INTO payment_order_discounts (
  order_id, user_coupon_id, coupon_template_id, coupon_code, scope, discount_amount,
  threshold_amount, original_amount, discounted_amount, status, reserved_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
RETURNING id, created_at, updated_at`
	return scanSingleRow(ctx, r.sql, query, []any{
		item.OrderID, item.UserCouponID, item.CouponTemplateID, item.CouponCode, item.Scope,
		item.DiscountAmount, item.ThresholdAmount, item.OriginalAmount, item.DiscountedAmount,
		item.Status, item.ReservedAt.UTC(),
	}, &item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *paymentOrderDiscountRepository) GetByOrderID(ctx context.Context, orderID int64) (*service.PaymentOrderDiscount, error) {
	item := &service.PaymentOrderDiscount{}
	var userCouponID sql.NullInt64
	var templateID sql.NullInt64
	var releasedAt sql.NullTime
	var usedAt sql.NullTime
	err := scanSingleRow(ctx, r.sql, `
SELECT id, order_id, user_coupon_id, coupon_template_id, coupon_code, scope, discount_amount,
       threshold_amount, original_amount, discounted_amount, status, reserved_at, released_at,
       used_at, created_at, updated_at
FROM payment_order_discounts
WHERE order_id = $1`, []any{orderID},
		&item.ID, &item.OrderID, &userCouponID, &templateID, &item.CouponCode, &item.Scope,
		&item.DiscountAmount, &item.ThresholdAmount, &item.OriginalAmount, &item.DiscountedAmount,
		&item.Status, &item.ReservedAt, &releasedAt, &usedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if userCouponID.Valid {
		v := userCouponID.Int64
		item.UserCouponID = &v
	}
	if templateID.Valid {
		v := templateID.Int64
		item.CouponTemplateID = &v
	}
	if releasedAt.Valid {
		v := releasedAt.Time.UTC()
		item.ReleasedAt = &v
	}
	if usedAt.Valid {
		v := usedAt.Time.UTC()
		item.UsedAt = &v
	}
	return item, nil
}

func (r *paymentOrderDiscountRepository) MarkReleasedByOrderID(ctx context.Context, orderID int64, releasedAt time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
UPDATE payment_order_discounts
SET status = 'released',
    released_at = $2,
    updated_at = NOW()
WHERE order_id = $1 AND status = 'reserved'`, orderID, releasedAt.UTC())
	return err
}

func (r *paymentOrderDiscountRepository) MarkUsedByOrderID(ctx context.Context, orderID int64, usedAt time.Time) error {
	_, err := r.sql.ExecContext(ctx, `
UPDATE payment_order_discounts
SET status = 'used',
    used_at = $2,
    updated_at = NOW()
WHERE order_id = $1 AND status = 'reserved'`, orderID, usedAt.UTC())
	return err
}
