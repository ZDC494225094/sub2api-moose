package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/robfig/cron/v3"
)

type BalanceHistoryPoint struct {
	Period   string  `json:"period"`
	Balance  float64 `json:"balance"`
	Realtime bool    `json:"realtime,omitempty"`
}

type UserBalanceSnapshotRepository interface {
	EnsureCurrentMonthSnapshot(ctx context.Context, userID int64, now time.Time) error
	SnapshotPreviousMonth(ctx context.Context, now time.Time) error
	ListHistory(ctx context.Context, userID int64, fromPeriod, toPeriod string) ([]BalanceHistoryPoint, error)
	GetCurrentBalance(ctx context.Context, userID int64) (float64, error)
}

type UserBalanceSnapshotService struct {
	repo      UserBalanceSnapshotRepository
	cronSched *cron.Cron
	now       func() time.Time
}

func NewUserBalanceSnapshotService(repo UserBalanceSnapshotRepository) *UserBalanceSnapshotService {
	return &UserBalanceSnapshotService{repo: repo, now: timezone.Now}
}

func (snapshotService *UserBalanceSnapshotService) Start() {
	snapshotService.cronSched = cron.New(cron.WithLocation(timezone.Location()), cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	_, err := snapshotService.cronSched.AddFunc("5 0 1 * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := snapshotService.repo.SnapshotPreviousMonth(ctx, snapshotService.now()); err != nil {
			logger.LegacyPrintf("service.balance_snapshot", "monthly balance snapshot failed: %v", err)
		}
	})
	if err != nil {
		logger.LegacyPrintf("service.balance_snapshot", "register monthly balance snapshot: %v", err)
		return
	}
	snapshotService.cronSched.Start()
}

func (snapshotService *UserBalanceSnapshotService) Stop() {
	if snapshotService.cronSched != nil {
		<-snapshotService.cronSched.Stop().Done()
	}
}

func (snapshotService *UserBalanceSnapshotService) EnsureCurrentMonthSnapshot(ctx context.Context, userID int64) error {
	return snapshotService.repo.EnsureCurrentMonthSnapshot(ctx, userID, snapshotService.now())
}

func (snapshotService *UserBalanceSnapshotService) GetHistory(ctx context.Context, userID int64, months int) ([]BalanceHistoryPoint, error) {
	switch months {
	case 3, 6, 12:
	default:
		months = 12
	}
	now := snapshotService.now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	currentPeriod := monthStart.Format("2006-01")
	fromPeriod := monthStart.AddDate(0, 1-months, 0).Format("2006-01")
	points, err := snapshotService.repo.ListHistory(ctx, userID, fromPeriod, currentPeriod)
	if err != nil {
		return nil, err
	}
	if len(points) == 0 || points[len(points)-1].Period != currentPeriod {
		balance, err := snapshotService.repo.GetCurrentBalance(ctx, userID)
		if err != nil {
			return nil, err
		}
		points = append(points, BalanceHistoryPoint{Period: currentPeriod, Balance: balance, Realtime: true})
	}
	return points, nil
}
