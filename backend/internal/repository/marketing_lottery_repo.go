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

type lotteryActivityRepository struct{ sql sqlExecutor }
type lotteryPrizeRepository struct{ sql sqlExecutor }
type lotteryUserStateRepository struct{ sql sqlExecutor }
type lotteryChanceLogRepository struct{ sql sqlExecutor }
type lotteryDrawRecordRepository struct{ sql sqlExecutor }
type lotteryConsumeProgressRepository struct{ sql sqlExecutor }

func NewLotteryActivityRepository(db *sql.DB) service.LotteryActivityRepository {
	return &lotteryActivityRepository{sql: db}
}

func NewLotteryPrizeRepository(db *sql.DB) service.LotteryPrizeRepository {
	return &lotteryPrizeRepository{sql: db}
}

func NewLotteryUserStateRepository(db *sql.DB) service.LotteryUserStateRepository {
	return &lotteryUserStateRepository{sql: db}
}

func NewLotteryChanceLogRepository(db *sql.DB) service.LotteryChanceLogRepository {
	return &lotteryChanceLogRepository{sql: db}
}

func NewLotteryDrawRecordRepository(db *sql.DB) service.LotteryDrawRecordRepository {
	return &lotteryDrawRecordRepository{sql: db}
}

func NewLotteryConsumeProgressRepository(db *sql.DB) service.LotteryConsumeProgressRepository {
	return &lotteryConsumeProgressRepository{sql: db}
}

func (r *lotteryActivityRepository) Create(ctx context.Context, item *service.LotteryActivity) error {
	return scanSingleRow(ctx, r.sql, `
INSERT INTO lottery_activities (
  name, description, status, default_draw_times, consume_threshold_amount,
  wallet_cost_per_draw, starts_at, ends_at, sort_order
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
RETURNING id, created_at, updated_at`, []any{
		item.Name, item.Description, item.Status, item.DefaultDrawTimes, item.ConsumeThresholdAmount,
		item.WalletCostPerDraw, item.StartsAt, item.EndsAt, item.SortOrder,
	}, &item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *lotteryActivityRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.sql.ExecContext(ctx, `DELETE FROM lottery_activities WHERE id = $1`, id)
	return err
}

func (r *lotteryActivityRepository) Update(ctx context.Context, item *service.LotteryActivity) error {
	return scanSingleRow(ctx, r.sql, `
UPDATE lottery_activities
SET name = $2,
    description = $3,
    status = $4,
    default_draw_times = $5,
    consume_threshold_amount = $6,
    wallet_cost_per_draw = $7,
    starts_at = $8,
    ends_at = $9,
    sort_order = $10,
    updated_at = NOW()
WHERE id = $1
RETURNING updated_at`, []any{
		item.ID, item.Name, item.Description, item.Status, item.DefaultDrawTimes, item.ConsumeThresholdAmount,
		item.WalletCostPerDraw, item.StartsAt, item.EndsAt, item.SortOrder,
	}, &item.UpdatedAt)
}

func (r *lotteryActivityRepository) GetByID(ctx context.Context, id int64) (*service.LotteryActivity, error) {
	return r.getBy(ctx, "id = $1", id, false)
}

func (r *lotteryActivityRepository) GetByIDForUpdate(ctx context.Context, id int64) (*service.LotteryActivity, error) {
	return r.getBy(ctx, "id = $1", id, true)
}

func (r *lotteryActivityRepository) GetActiveActivity(ctx context.Context) (*service.LotteryActivity, error) {
	return r.getBy(ctx, "status = 'active'", nil, false)
}

func (r *lotteryActivityRepository) getBy(ctx context.Context, predicate string, arg any, forUpdate bool) (*service.LotteryActivity, error) {
	query := `
SELECT id, name, description, status, default_draw_times, consume_threshold_amount,
       wallet_cost_per_draw, starts_at, ends_at, sort_order, created_at, updated_at
FROM lottery_activities
WHERE ` + predicate + ` ORDER BY sort_order ASC, id ASC LIMIT 1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	exec := r.sql
	item := &service.LotteryActivity{}
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	var args []any
	if arg != nil {
		args = []any{arg}
	}
	err := scanSingleRow(ctx, exec, query, args, &item.ID, &item.Name, &item.Description, &item.Status, &item.DefaultDrawTimes, &item.ConsumeThresholdAmount, &item.WalletCostPerDraw, &startsAt, &endsAt, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrLotteryActivityNotFound
		}
		return nil, err
	}
	if startsAt.Valid {
		v := startsAt.Time.UTC()
		item.StartsAt = &v
	}
	if endsAt.Valid {
		v := endsAt.Time.UTC()
		item.EndsAt = &v
	}
	return item, nil
}

func (r *lotteryActivityRepository) List(ctx context.Context, params pagination.PaginationParams, filter service.LotteryActivityListFilter) ([]service.LotteryActivity, *pagination.PaginationResult, error) {
	exec := r.sql
	where := []string{"1=1"}
	args := make([]any, 0, 2)
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("status = $%d", len(args)))
	}
	if filter.Search != "" {
		args = append(args, "%"+strings.TrimSpace(filter.Search)+"%")
		where = append(where, fmt.Sprintf("(name ILIKE $%d OR description ILIKE $%d)", len(args), len(args)))
	}
	whereClause := strings.Join(where, " AND ")
	var total int64
	if err := scanSingleRow(ctx, exec, "SELECT COUNT(*) FROM lottery_activities WHERE "+whereClause, args, &total); err != nil {
		return nil, nil, err
	}
	rows, err := exec.QueryContext(ctx, `
SELECT id, name, description, status, default_draw_times, consume_threshold_amount,
       wallet_cost_per_draw, starts_at, ends_at, sort_order, created_at, updated_at
FROM lottery_activities
WHERE `+whereClause+fmt.Sprintf(" ORDER BY sort_order ASC, id ASC LIMIT %d OFFSET %d", params.Limit(), params.Offset()), args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.LotteryActivity, 0)
	for rows.Next() {
		var item service.LotteryActivity
		var startsAt sql.NullTime
		var endsAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Status, &item.DefaultDrawTimes, &item.ConsumeThresholdAmount, &item.WalletCostPerDraw, &startsAt, &endsAt, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, nil, err
		}
		if startsAt.Valid {
			v := startsAt.Time.UTC()
			item.StartsAt = &v
		}
		if endsAt.Valid {
			v := endsAt.Time.UTC()
			item.EndsAt = &v
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *lotteryPrizeRepository) Create(ctx context.Context, item *service.LotteryPrize) error {
	return scanSingleRow(ctx, r.sql, `
INSERT INTO lottery_prizes (
  activity_id, name, prize_type, stock, remaining_stock, balance_amount,
  coupon_template_id, display_order, status
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
RETURNING id, created_at, updated_at`, []any{
		item.ActivityID, item.Name, item.PrizeType, item.Stock, item.RemainingStock, item.BalanceAmount,
		item.CouponTemplateID, item.DisplayOrder, item.Status,
	}, &item.ID, &item.CreatedAt, &item.UpdatedAt)
}

func (r *lotteryPrizeRepository) Update(ctx context.Context, item *service.LotteryPrize) error {
	return scanSingleRow(ctx, r.sql, `
UPDATE lottery_prizes
SET name = $2,
    prize_type = $3,
    stock = $4,
    remaining_stock = $5,
    balance_amount = $6,
    coupon_template_id = $7,
    display_order = $8,
    status = $9,
    updated_at = NOW()
WHERE id = $1
RETURNING updated_at`, []any{
		item.ID, item.Name, item.PrizeType, item.Stock, item.RemainingStock, item.BalanceAmount,
		item.CouponTemplateID, item.DisplayOrder, item.Status,
	}, &item.UpdatedAt)
}

func (r *lotteryPrizeRepository) GetByID(ctx context.Context, id int64) (*service.LotteryPrize, error) {
	exec := r.sql
	query := `
SELECT id, activity_id, name, prize_type, stock, remaining_stock, balance_amount,
       coupon_template_id, display_order, status, created_at, updated_at
FROM lottery_prizes WHERE id = $1`
	item := &service.LotteryPrize{}
	var balanceAmount sql.NullFloat64
	var couponTemplateID sql.NullInt64
	err := scanSingleRow(ctx, exec, query, []any{id}, &item.ID, &item.ActivityID, &item.Name, &item.PrizeType, &item.Stock, &item.RemainingStock, &balanceAmount, &couponTemplateID, &item.DisplayOrder, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, service.ErrLotteryPrizeNotFound
		}
		return nil, err
	}
	if balanceAmount.Valid {
		v := balanceAmount.Float64
		item.BalanceAmount = &v
	}
	if couponTemplateID.Valid {
		v := couponTemplateID.Int64
		item.CouponTemplateID = &v
	}
	return item, nil
}

func (r *lotteryPrizeRepository) ListByActivity(ctx context.Context, activityID int64) ([]service.LotteryPrize, error) {
	return r.listByActivity(ctx, activityID, false)
}

func (r *lotteryPrizeRepository) ListActiveByActivityForUpdate(ctx context.Context, activityID int64) ([]service.LotteryPrize, error) {
	return r.listByActivity(ctx, activityID, true)
}

func (r *lotteryPrizeRepository) listByActivity(ctx context.Context, activityID int64, forUpdate bool) ([]service.LotteryPrize, error) {
	exec := r.sql
	query := `
SELECT id, activity_id, name, prize_type, stock, remaining_stock, balance_amount,
       coupon_template_id, display_order, status, created_at, updated_at
FROM lottery_prizes
WHERE activity_id = $1
  AND ($2::boolean = false OR (status = 'active' AND remaining_stock > 0))
ORDER BY display_order ASC, id ASC`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	rows, err := exec.QueryContext(ctx, query, activityID, forUpdate)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.LotteryPrize, 0)
	for rows.Next() {
		var item service.LotteryPrize
		var balanceAmount sql.NullFloat64
		var couponTemplateID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ActivityID, &item.Name, &item.PrizeType, &item.Stock, &item.RemainingStock, &balanceAmount, &couponTemplateID, &item.DisplayOrder, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		if balanceAmount.Valid {
			v := balanceAmount.Float64
			item.BalanceAmount = &v
		}
		if couponTemplateID.Valid {
			v := couponTemplateID.Int64
			item.CouponTemplateID = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *lotteryPrizeRepository) DecrementStock(ctx context.Context, prizeID int64) error {
	_, err := r.sql.ExecContext(ctx, `
UPDATE lottery_prizes
SET remaining_stock = remaining_stock - 1,
    updated_at = NOW()
WHERE id = $1
  AND remaining_stock > 0`, prizeID)
	return err
}

func (r *lotteryUserStateRepository) GetOrCreate(ctx context.Context, activityID, userID int64) (*service.LotteryUserState, error) {
	exec := r.sql
	_, err := exec.ExecContext(ctx, `
INSERT INTO lottery_user_states (activity_id, user_id)
VALUES ($1,$2)
ON CONFLICT (activity_id, user_id) DO NOTHING`, activityID, userID)
	if err != nil {
		return nil, err
	}
	item := &service.LotteryUserState{}
	err = scanSingleRow(ctx, exec, `
SELECT activity_id, user_id, default_granted, available_draw_times, total_granted_times,
       total_drawn_times, total_wallet_paid_amount, created_at, updated_at
FROM lottery_user_states
WHERE activity_id = $1 AND user_id = $2`, []any{activityID, userID},
		&item.ActivityID, &item.UserID, &item.DefaultGranted, &item.AvailableDrawTimes,
		&item.TotalGrantedTimes, &item.TotalDrawnTimes, &item.TotalWalletPaidAmount,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *lotteryUserStateRepository) GetOrCreateForUpdate(ctx context.Context, activityID, userID int64) (*service.LotteryUserState, error) {
	exec := r.sql
	_, err := exec.ExecContext(ctx, `
INSERT INTO lottery_user_states (activity_id, user_id)
VALUES ($1,$2)
ON CONFLICT (activity_id, user_id) DO NOTHING`, activityID, userID)
	if err != nil {
		return nil, err
	}
	item := &service.LotteryUserState{}
	err = scanSingleRow(ctx, exec, `
SELECT activity_id, user_id, default_granted, available_draw_times, total_granted_times,
       total_drawn_times, total_wallet_paid_amount, created_at, updated_at
FROM lottery_user_states
WHERE activity_id = $1 AND user_id = $2
FOR UPDATE`, []any{activityID, userID},
		&item.ActivityID, &item.UserID, &item.DefaultGranted, &item.AvailableDrawTimes,
		&item.TotalGrantedTimes, &item.TotalDrawnTimes, &item.TotalWalletPaidAmount,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (r *lotteryUserStateRepository) Update(ctx context.Context, item *service.LotteryUserState) error {
	return scanSingleRow(ctx, r.sql, `
UPDATE lottery_user_states
SET default_granted = $3,
    available_draw_times = $4,
    total_granted_times = $5,
    total_drawn_times = $6,
    total_wallet_paid_amount = $7,
    updated_at = NOW()
WHERE activity_id = $1 AND user_id = $2
RETURNING updated_at`, []any{
		item.ActivityID, item.UserID, item.DefaultGranted, item.AvailableDrawTimes,
		item.TotalGrantedTimes, item.TotalDrawnTimes, item.TotalWalletPaidAmount,
	}, &item.UpdatedAt)
}

func (r *lotteryChanceLogRepository) Create(ctx context.Context, item *service.LotteryChanceLog) error {
	query := `
INSERT INTO lottery_chance_logs (
  activity_id, user_id, change_amount, balance_after, source_type, source_ref_id, notes, created_at
)
VALUES ($1,$2,$3,$4,$5,$6,$7,COALESCE($8, NOW()))
RETURNING id, created_at`
	var createdAt *time.Time
	if !item.CreatedAt.IsZero() {
		t := item.CreatedAt.UTC()
		createdAt = &t
	}
	return scanSingleRow(ctx, r.sql, query, []any{
		item.ActivityID, item.UserID, item.ChangeAmount, item.BalanceAfter, item.SourceType,
		item.SourceRefID, item.Notes, createdAt,
	}, &item.ID, &item.CreatedAt)
}

func (r *lotteryDrawRecordRepository) Create(ctx context.Context, item *service.LotteryDrawRecord) error {
	return scanSingleRow(ctx, r.sql, `
INSERT INTO lottery_draw_records (
  activity_id, user_id, prize_id, prize_name, prize_type, result_code,
  chance_source, wallet_amount, user_coupon_id, reward_reference
)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
RETURNING id, created_at`, []any{
		item.ActivityID, item.UserID, item.PrizeID, item.PrizeName, item.PrizeType,
		item.ResultCode, item.ChanceSource, item.WalletAmount, item.UserCouponID, item.RewardReference,
	}, &item.ID, &item.CreatedAt)
}

func (r *lotteryDrawRecordRepository) ListByUser(ctx context.Context, userID int64, activityID int64, params pagination.PaginationParams) ([]service.LotteryDrawRecord, *pagination.PaginationResult, error) {
	exec := r.sql
	where := []string{}
	args := []any{}
	if userID > 0 {
		args = append(args, userID)
		where = append(where, fmt.Sprintf("user_id = $%d", len(args)))
	}
	if activityID > 0 {
		args = append(args, activityID)
		where = append(where, fmt.Sprintf("activity_id = $%d", len(args)))
	}
	whereClause := "1=1"
	if len(where) > 0 {
		whereClause = strings.Join(where, " AND ")
	}
	var total int64
	if err := scanSingleRow(ctx, exec, "SELECT COUNT(*) FROM lottery_draw_records WHERE "+whereClause, args, &total); err != nil {
		return nil, nil, err
	}
	rows, err := exec.QueryContext(ctx, `
SELECT r.id, r.activity_id, r.user_id, r.prize_id, r.prize_name, r.prize_type, r.result_code, r.chance_source,
       r.wallet_amount, r.user_coupon_id, r.reward_reference, r.created_at, u.username, u.email
FROM lottery_draw_records r
LEFT JOIN users u ON u.id = r.user_id
WHERE `+whereClause+fmt.Sprintf(" ORDER BY r.created_at DESC, r.id DESC LIMIT %d OFFSET %d", params.Limit(), params.Offset()), args...)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.LotteryDrawRecord, 0)
	for rows.Next() {
		var item service.LotteryDrawRecord
		var prizeID sql.NullInt64
		var userCouponID sql.NullInt64
		var userName sql.NullString
		var userEmail sql.NullString
		if err := rows.Scan(&item.ID, &item.ActivityID, &item.UserID, &prizeID, &item.PrizeName, &item.PrizeType, &item.ResultCode, &item.ChanceSource, &item.WalletAmount, &userCouponID, &item.RewardReference, &item.CreatedAt, &userName, &userEmail); err != nil {
			return nil, nil, err
		}
		if prizeID.Valid {
			v := prizeID.Int64
			item.PrizeID = &v
		}
		if userCouponID.Valid {
			v := userCouponID.Int64
			item.UserCouponID = &v
		}
		if userName.Valid {
			item.UserName = userName.String
		}
		if userEmail.Valid {
			item.UserEmail = userEmail.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return items, paginationResultFromTotal(total, params), nil
}

func (r *lotteryDrawRecordRepository) ListRecentByActivity(ctx context.Context, activityID int64, limit int) ([]service.LotteryDrawRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.sql.QueryContext(ctx, `
SELECT r.id, r.activity_id, r.user_id, r.prize_id, r.prize_name, r.prize_type, r.result_code, r.chance_source,
       r.wallet_amount, r.user_coupon_id, r.reward_reference, r.created_at, u.username, u.email
FROM lottery_draw_records r
LEFT JOIN users u ON u.id = r.user_id
WHERE r.activity_id = $1
ORDER BY r.created_at DESC, r.id DESC
LIMIT $2`, activityID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.LotteryDrawRecord, 0, limit)
	for rows.Next() {
		var item service.LotteryDrawRecord
		var prizeID sql.NullInt64
		var userCouponID sql.NullInt64
		var userName sql.NullString
		var userEmail sql.NullString
		if err := rows.Scan(&item.ID, &item.ActivityID, &item.UserID, &prizeID, &item.PrizeName, &item.PrizeType, &item.ResultCode, &item.ChanceSource, &item.WalletAmount, &userCouponID, &item.RewardReference, &item.CreatedAt, &userName, &userEmail); err != nil {
			return nil, err
		}
		if prizeID.Valid {
			v := prizeID.Int64
			item.PrizeID = &v
		}
		if userCouponID.Valid {
			v := userCouponID.Int64
			item.UserCouponID = &v
		}
		if userName.Valid {
			item.UserName = userName.String
		}
		if userEmail.Valid {
			item.UserEmail = userEmail.String
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *lotteryDrawRecordRepository) ExistsByActivity(ctx context.Context, activityID int64) (bool, error) {
	var exists bool
	err := scanSingleRow(ctx, r.sql, `
SELECT EXISTS(
  SELECT 1
  FROM lottery_draw_records
  WHERE activity_id = $1
)`, []any{activityID}, &exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (r *lotteryConsumeProgressRepository) GetQualifiedAmount(ctx context.Context, activityID, userID int64, threshold float64) (float64, error) {
	exec := r.sql
	var amount float64
	err := scanSingleRow(ctx, exec, `
SELECT COALESCE(SUM(pay_amount), 0)
FROM payment_orders
WHERE user_id = $1
  AND status IN ('PAID', 'RECHARGING', 'COMPLETED')
  AND pay_amount >= 0`, []any{userID}, &amount)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	_ = activityID
	_ = threshold
	return amount, nil
}
