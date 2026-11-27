package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/robfig/cron/v3"
)

type BalanceTotals struct {
	TotalBalance               float64
	TotalSubscriptionRemaining float64
	AsOf                       time.Time
}

type BalanceObservation struct {
	ObservedOn                 string     `json:"observed_on,omitempty"`
	AsOf                       *time.Time `json:"as_of,omitempty"`
	Source                     string     `json:"source"`
	TotalBalance               *float64   `json:"total_balance"`
	TotalSubscriptionRemaining *float64   `json:"total_subscription_remaining"`
}

type BalanceHistoryPoint struct {
	BalanceObservation
	Period    string `json:"period"`
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Realtime  bool   `json:"realtime,omitempty"`
}

type UserBalanceSnapshotRepository interface {
	EnsureCurrentMonthSnapshot(ctx context.Context, userID int64, now time.Time) error
	SnapshotPreviousMonth(ctx context.Context, now time.Time) error
	CaptureDailyBalance(ctx context.Context, loc *time.Location) error
	ListBalanceObservations(ctx context.Context, start, end time.Time) ([]BalanceObservation, error)
	CurrentBalanceTotals(ctx context.Context) (*BalanceTotals, error)
}

type UserBalanceSnapshotService struct {
	repo        UserBalanceSnapshotRepository
	cronSched   *cron.Cron
	startupDone chan struct{}
	now         func() time.Time
}

func NewUserBalanceSnapshotService(repo UserBalanceSnapshotRepository) *UserBalanceSnapshotService {
	return &UserBalanceSnapshotService{repo: repo, now: timezone.Now}
}

func (snapshotService *UserBalanceSnapshotService) Start() {
	snapshotService.cronSched = cron.New(cron.WithSeconds(), cron.WithLocation(timezone.Location()), cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	capture := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := snapshotService.repo.CaptureDailyBalance(ctx, timezone.Location()); err != nil {
			logger.LegacyPrintf("service.balance_snapshot", "daily balance snapshot failed: %v", err)
		}
	}
	// Save the last observation of each business day, including one just
	// before midnight while subscription windows still belong to that day.
	if _, err := snapshotService.cronSched.AddFunc("59 14,29,44,59 * * * *", capture); err != nil {
		logger.LegacyPrintf("service.balance_snapshot", "register daily balance snapshot: %v", err)
		return
	}
	_, err := snapshotService.cronSched.AddFunc("0 5 0 1 * *", func() {
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
	snapshotService.startupDone = make(chan struct{})
	go func() {
		defer close(snapshotService.startupDone)
		capture()
	}()
}

func (snapshotService *UserBalanceSnapshotService) Stop() {
	if snapshotService.cronSched != nil {
		<-snapshotService.cronSched.Stop().Done()
	}
	if snapshotService.startupDone != nil {
		<-snapshotService.startupDone
	}
}

func (snapshotService *UserBalanceSnapshotService) EnsureCurrentMonthSnapshot(ctx context.Context, userID int64) error {
	return snapshotService.repo.EnsureCurrentMonthSnapshot(ctx, userID, snapshotService.now())
}

// GetHistory returns the last observation inside each inclusive reporting
// period. Missing observations retain null amounts, including a missing
// historical subscription total when only a wallet balance can be recovered.
func (snapshotService *UserBalanceSnapshotService) GetHistory(ctx context.Context, start, end time.Time, granularity string) ([]BalanceHistoryPoint, error) {
	observations, err := snapshotService.repo.ListBalanceObservations(ctx, start, end)
	if err != nil {
		return nil, err
	}
	endDay := end.Format(time.DateOnly)
	if endDay == snapshotService.now().In(end.Location()).Format(time.DateOnly) {
		current, err := snapshotService.repo.CurrentBalanceTotals(ctx)
		if err != nil {
			return nil, err
		}
		if current.AsOf.In(end.Location()).Format(time.DateOnly) == endDay {
			observations = append(observations, BalanceObservation{
				ObservedOn: endDay, AsOf: &current.AsOf, Source: "live",
				TotalBalance: &current.TotalBalance, TotalSubscriptionRemaining: &current.TotalSubscriptionRemaining,
			})
		}
	}
	points := make([]BalanceHistoryPoint, 0)
	for day := start; !day.After(end); {
		last := day
		label := day.Format(time.DateOnly)
		switch granularity {
		case "day":
		case "week":
			last = day.AddDate(0, 0, (7-int(day.Weekday()))%7) // Sunday, inclusive
		case "month":
			last = time.Date(day.Year(), day.Month()+1, 0, 0, 0, 0, 0, day.Location())
			label = day.Format("2006-01")
		case "range":
			last = end
		default:
			return nil, fmt.Errorf("invalid balance history granularity %q", granularity)
		}
		if last.After(end) {
			last = end
		}
		point := BalanceHistoryPoint{Period: label, StartDate: day.Format(time.DateOnly), EndDate: last.Format(time.DateOnly),
			BalanceObservation: BalanceObservation{Source: "missing"}}
		if (granularity == "week" || granularity == "range") && point.StartDate != point.EndDate {
			point.Period = point.StartDate + " – " + point.EndDate
		}
		for len(observations) > 0 && observations[0].ObservedOn <= point.EndDate {
			point.BalanceObservation = observations[0]
			observations = observations[1:]
		}
		point.Realtime = point.Source == "live"
		points = append(points, point)
		day = last.AddDate(0, 0, 1)
	}
	return points, nil
}
