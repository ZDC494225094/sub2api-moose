package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type dailyResetTrackingUserSubRepo struct {
	userSubRepoNoop

	resetDailyCalled   bool
	resetWeeklyCalled  bool
	resetMonthlyCalled bool
}

func (r *dailyResetTrackingUserSubRepo) ResetDailyUsage(context.Context, int64, *time.Time, time.Time) error {
	r.resetDailyCalled = true
	return nil
}

func (r *dailyResetTrackingUserSubRepo) ResetWeeklyUsage(context.Context, int64, *time.Time, time.Time) error {
	r.resetWeeklyCalled = true
	return nil
}

func (r *dailyResetTrackingUserSubRepo) ResetMonthlyUsage(context.Context, int64, *time.Time, time.Time) error {
	r.resetMonthlyCalled = true
	return nil
}

func TestAssignOrExtendSubscription_ExpiredDailyCardCreatesIndependentOneTimeQuota(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Now().AddDate(0, 0, -3)
	oldWindowStart := startOfDay(oldStart)
	subRepo.seed(&UserSubscription{
		ID:                 100,
		UserID:             200,
		GroupID:            1,
		StartsAt:           oldStart,
		ExpiresAt:          oldStart.AddDate(0, 0, 1),
		Status:             SubscriptionStatusExpired,
		DailyWindowStart:   &oldWindowStart,
		WeeklyWindowStart:  &oldWindowStart,
		MonthlyWindowStart: &oldWindowStart,
		DailyUsageUSD:      10,
		WeeklyUsageUSD:     20,
		MonthlyUsageUSD:    30,
		Notes:              "old",
	})
	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	renewed, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       200,
		GroupID:      1,
		ValidityDays: 1,
		Notes:        "new",
	})

	require.NoError(t, err)
	require.False(t, reused)
	require.NotEqual(t, int64(100), renewed.ID)
	require.True(t, renewed.HasOneTimeDailyQuota(), "过期后重新购买 1 日卡仍应被识别为一次性日额度")
	require.Equal(t, SubscriptionStatusActive, renewed.Status)
	require.True(t, renewed.StartsAt.After(oldStart), "重新购买过期订阅时应重置当前周期 StartsAt")
	require.False(t, renewed.ExpiresAt.After(renewed.StartsAt.AddDate(0, 0, 1)))
	require.NotNil(t, renewed.DailyWindowStart)
	require.Equal(t, timezone.StartOfDay(renewed.StartsAt), *renewed.DailyWindowStart, "续期后日窗口应锚定当天 0 点")
	require.Equal(t, 0.0, renewed.DailyUsageUSD)
	require.Equal(t, 0.0, renewed.WeeklyUsageUSD)
	require.Equal(t, 0.0, renewed.MonthlyUsageUSD)
	require.Equal(t, "new", renewed.Notes)

	oldSub, err := subRepo.GetByID(context.Background(), 100)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusExpired, oldSub.Status)
	require.Equal(t, 10.0, oldSub.DailyUsageUSD)
	require.Equal(t, 2, len(subRepo.byID))
}

func TestAssignOrExtendSubscription_ExpiredSubscriptionCreatesIndependentInstance(t *testing.T) {
	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 1, SubscriptionType: SubscriptionTypeSubscription},
	}
	subRepo := newSubscriptionUserSubRepoStub()
	oldStart := time.Now().AddDate(0, 0, -3)
	subRepo.seed(&UserSubscription{
		ID:        101,
		UserID:    201,
		GroupID:   1,
		StartsAt:  oldStart,
		ExpiresAt: oldStart.AddDate(0, 0, 1),
		Status:    SubscriptionStatusExpired,
		Notes:     "same",
	})
	svc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)

	created, reused, err := svc.AssignOrExtendSubscription(context.Background(), &AssignSubscriptionInput{
		UserID:       201,
		GroupID:      1,
		ValidityDays: 1,
		Notes:        "same",
	})

	require.NoError(t, err)
	require.False(t, reused)
	require.NotEqual(t, int64(101), created.ID)
	require.Equal(t, "same", created.Notes)

	original, err := subRepo.GetByID(context.Background(), 101)
	require.NoError(t, err)
	require.Equal(t, SubscriptionStatusExpired, original.Status)
	require.Equal(t, "same", original.Notes)
}

func TestUserSubscriptionNeedsDailyReset_DailyCardKeepsOneTimeQuota(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        start.Add(24 * time.Hour),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    10,
	}

	require.True(t, sub.HasOneTimeDailyQuota())
	require.False(t, sub.NeedsDailyResetAt(dailyWindowStart.Add(25*time.Hour)), "日卡应作为一次性配额，跨 0 点后不再刷新日额度")
}

func TestUserSubscriptionNeedsDailyReset_MultiDaySubscriptionStillRefreshes(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        start.AddDate(0, 0, 2),
		DailyWindowStart: &dailyWindowStart,
	}

	require.False(t, sub.HasOneTimeDailyQuota())
	require.True(t, sub.NeedsDailyResetAt(dailyWindowStart.Add(24*time.Hour)), "multi-day subscriptions reset at the next calendar-day boundary")
	require.True(t, sub.NeedsDailyResetAt(start.Add(24*time.Hour)))
}

func TestUserSubscriptionDailyResetTime_DailyCardReturnsExpiry(t *testing.T) {
	start := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	dailyWindowStart := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	expiresAt := start.Add(24 * time.Hour)
	sub := &UserSubscription{
		StartsAt:         start,
		ExpiresAt:        expiresAt,
		DailyWindowStart: &dailyWindowStart,
	}

	resetAt := sub.DailyResetTime()
	require.NotNil(t, resetAt)
	require.Equal(t, expiresAt, *resetAt, "日卡展示的日额度结束时间应为订阅过期时间")
}

func TestUserSubscriptionNeedsMonthlyReset_ThirtyDayPlanUsesExactStartTime(t *testing.T) {
	start := time.Date(2026, 6, 1, 15, 30, 45, 0, time.UTC)
	sub := &UserSubscription{
		Status:             SubscriptionStatusActive,
		StartsAt:           start,
		ExpiresAt:          start.Add(30 * 24 * time.Hour),
		MonthlyWindowStart: &start,
	}

	require.False(t, sub.NeedsMonthlyResetAt(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)), "30天包月套餐不应在到期当天零点提前重置月额度")
	require.False(t, sub.NeedsMonthlyResetAt(start.Add(30*24*time.Hour)), "到期时订阅已过期，不应再重置月额度")
	require.Equal(t, start.Add(30*24*time.Hour).Add(time.Minute), *sub.MonthlyResetTime())
}

func TestUserSubscriptionResetTime_DailyUsesCalendarBoundaryAndPeriodicUsesStartsAt(t *testing.T) {
	start := time.Date(2026, 6, 1, 15, 30, 45, 0, time.UTC)
	midnight := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	sub := &UserSubscription{
		Status:             SubscriptionStatusActive,
		StartsAt:           start,
		ExpiresAt:          start.Add(60 * 24 * time.Hour),
		DailyWindowStart:   &midnight,
		WeeklyWindowStart:  &midnight,
		MonthlyWindowStart: &midnight,
	}

	require.Equal(t, midnight.AddDate(0, 0, 1), *sub.DailyResetTime())
	require.Equal(t, start.Add(7*24*time.Hour), *sub.WeeklyResetTime())
	require.Equal(t, start.Add(30*24*time.Hour), *sub.MonthlyResetTime())
	require.True(t, sub.NeedsDailyResetAt(time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)))
}

func TestUserSubscriptionNeedsReset_ExpiredSubscriptionKeepsUsageWindows(t *testing.T) {
	start := time.Now().Add(-40 * 24 * time.Hour)
	sub := &UserSubscription{
		Status:             SubscriptionStatusActive,
		StartsAt:           start,
		ExpiresAt:          time.Now().Add(-time.Hour),
		DailyWindowStart:   &start,
		WeeklyWindowStart:  &start,
		MonthlyWindowStart: &start,
	}

	require.False(t, sub.NeedsDailyReset())
	require.False(t, sub.NeedsWeeklyReset())
	require.False(t, sub.NeedsMonthlyReset())
}

func TestCheckAndResetWindows_DailyCardDoesNotResetDailyUsage(t *testing.T) {
	now := time.Now()
	startsAt := now.Add(-23 * time.Hour)
	dailyWindowStart := now.Add(-25 * time.Hour)
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub := &UserSubscription{
		ID:               1,
		UserID:           10,
		GroupID:          20,
		StartsAt:         startsAt,
		ExpiresAt:        startsAt.Add(24 * time.Hour),
		DailyUsageUSD:    10,
		DailyWindowStart: &dailyWindowStart,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.False(t, repo.resetDailyCalled, "日卡作为一次性配额，过了 24 小时日窗口也不应重置 daily usage")
	require.Equal(t, 10.0, sub.DailyUsageUSD)
}

func TestCheckAndResetWindows_MultiDaySubscriptionStillResetsDailyUsage(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	startsAt := now.Add(-48 * time.Hour)
	dailyWindowStart := now.Add(-25 * time.Hour)
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	svc.now = func() time.Time { return now }
	sub := &UserSubscription{
		ID:               1,
		UserID:           10,
		GroupID:          20,
		StartsAt:         startsAt,
		ExpiresAt:        startsAt.AddDate(0, 0, 4),
		DailyUsageUSD:    10,
		DailyWindowStart: &dailyWindowStart,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.True(t, repo.resetDailyCalled, "多日订阅仍应重置过期 daily window")
	require.Equal(t, 0.0, sub.DailyUsageUSD)
}

func TestCheckAndResetWindows_ExpiredSubscriptionDoesNotResetUsage(t *testing.T) {
	windowStart := time.Now().Add(-40 * 24 * time.Hour)
	repo := &dailyResetTrackingUserSubRepo{}
	svc := NewSubscriptionService(groupRepoNoop{}, repo, nil, nil, nil)
	sub := &UserSubscription{
		ID:                 1,
		UserID:             10,
		GroupID:            20,
		StartsAt:           windowStart,
		ExpiresAt:          time.Now().Add(-time.Hour),
		DailyUsageUSD:      10,
		WeeklyUsageUSD:     20,
		MonthlyUsageUSD:    30,
		DailyWindowStart:   &windowStart,
		WeeklyWindowStart:  &windowStart,
		MonthlyWindowStart: &windowStart,
	}

	err := svc.CheckAndResetWindows(context.Background(), sub)

	require.NoError(t, err)
	require.False(t, repo.resetDailyCalled)
	require.False(t, repo.resetWeeklyCalled)
	require.False(t, repo.resetMonthlyCalled)
	require.Equal(t, 10.0, sub.DailyUsageUSD)
	require.Equal(t, 20.0, sub.WeeklyUsageUSD)
	require.Equal(t, 30.0, sub.MonthlyUsageUSD)
}

func TestNormalizeExpiredWindows_ExpiredSubscriptionPreservesUsageForDisplay(t *testing.T) {
	windowStart := time.Now().Add(-40 * 24 * time.Hour)
	subs := []UserSubscription{{
		Status:             SubscriptionStatusExpired,
		StartsAt:           windowStart,
		ExpiresAt:          time.Now().Add(-time.Hour),
		DailyWindowStart:   &windowStart,
		WeeklyWindowStart:  &windowStart,
		MonthlyWindowStart: &windowStart,
		DailyUsageUSD:      10,
		WeeklyUsageUSD:     20,
		MonthlyUsageUSD:    30,
	}}

	normalizeExpiredWindows(subs)

	require.Equal(t, 10.0, subs[0].DailyUsageUSD)
	require.Equal(t, 20.0, subs[0].WeeklyUsageUSD)
	require.Equal(t, 30.0, subs[0].MonthlyUsageUSD)
	require.NotNil(t, subs[0].DailyWindowStart)
	require.NotNil(t, subs[0].WeeklyWindowStart)
	require.NotNil(t, subs[0].MonthlyWindowStart)
}

func TestValidateAndCheckLimits_DailyCardDoesNotAllowSecondQuotaAfterMidnight(t *testing.T) {
	start := time.Now().Add(-23 * time.Hour)
	dailyWindowStart := time.Now().Add(-25 * time.Hour)
	dailyLimit := 10.0
	sub := &UserSubscription{
		Status:           SubscriptionStatusActive,
		StartsAt:         start,
		ExpiresAt:        start.Add(24 * time.Hour),
		DailyWindowStart: &dailyWindowStart,
		DailyUsageUSD:    dailyLimit + 0.01,
	}
	group := &Group{
		SubscriptionType: SubscriptionTypeSubscription,
		DailyLimitUSD:    &dailyLimit,
	}
	svc := NewSubscriptionService(groupRepoNoop{}, userSubRepoNoop{}, nil, nil, nil)

	needsMaintenance, err := svc.ValidateAndCheckLimits(sub, group)

	require.False(t, needsMaintenance, "日卡跨过日窗口后不应触发 daily reset 维护")
	require.True(t, errors.Is(err, ErrDailyLimitExceeded))
	require.Equal(t, dailyLimit+0.01, sub.DailyUsageUSD, "热路径不应清零日卡已用额度")
}
