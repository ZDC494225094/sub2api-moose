package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type LotteryActivityRepository interface {
	Create(ctx context.Context, activity *LotteryActivity) error
	Update(ctx context.Context, activity *LotteryActivity) error
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*LotteryActivity, error)
	GetByIDForUpdate(ctx context.Context, id int64) (*LotteryActivity, error)
	GetActiveActivity(ctx context.Context) (*LotteryActivity, error)
	List(ctx context.Context, params pagination.PaginationParams, filter LotteryActivityListFilter) ([]LotteryActivity, *pagination.PaginationResult, error)
}

type LotteryPrizeRepository interface {
	Create(ctx context.Context, prize *LotteryPrize) error
	Update(ctx context.Context, prize *LotteryPrize) error
	GetByID(ctx context.Context, id int64) (*LotteryPrize, error)
	ListByActivity(ctx context.Context, activityID int64) ([]LotteryPrize, error)
	ListActiveByActivityForUpdate(ctx context.Context, activityID int64) ([]LotteryPrize, error)
	DecrementStock(ctx context.Context, prizeID int64) error
}

type LotteryUserStateRepository interface {
	GetOrCreate(ctx context.Context, activityID, userID int64) (*LotteryUserState, error)
	GetOrCreateForUpdate(ctx context.Context, activityID, userID int64) (*LotteryUserState, error)
	Update(ctx context.Context, state *LotteryUserState) error
}

type LotteryChanceLogRepository interface {
	Create(ctx context.Context, log *LotteryChanceLog) error
}

type LotteryDrawRecordRepository interface {
	Create(ctx context.Context, record *LotteryDrawRecord) error
	ListByUser(ctx context.Context, userID int64, activityID int64, params pagination.PaginationParams) ([]LotteryDrawRecord, *pagination.PaginationResult, error)
	ListRecentByActivity(ctx context.Context, activityID int64, limit int) ([]LotteryDrawRecord, error)
	ExistsByActivity(ctx context.Context, activityID int64) (bool, error)
}

type LotteryConsumeProgressRepository interface {
	GetQualifiedAmount(ctx context.Context, activityID, userID int64, threshold float64) (float64, error)
}

type LotteryClock interface {
	Now() time.Time
}
