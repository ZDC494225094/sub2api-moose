package service

import "time"

const (
	subscriptionResetAfterExpiryBoundaryDelay = time.Minute
	subscriptionDayDuration                   = 24 * time.Hour
)

type UserSubscription struct {
	ID      int64
	UserID  int64
	GroupID int64

	StartsAt  time.Time
	ExpiresAt time.Time
	Status    string

	DailyWindowStart   *time.Time
	WeeklyWindowStart  *time.Time
	MonthlyWindowStart *time.Time

	DailyUsageUSD   float64
	WeeklyUsageUSD  float64
	MonthlyUsageUSD float64

	AssignedBy *int64
	AssignedAt time.Time
	Notes      string

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time

	User           *User
	Group          *Group
	AssignedByUser *User
}

func (s *UserSubscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive && time.Now().Before(s.ExpiresAt)
}

func (s *UserSubscription) IsExpired() bool {
	return s.IsExpiredAt(time.Now())
}

func (s *UserSubscription) IsExpiredAt(now time.Time) bool {
	if s == nil {
		return true
	}
	return !now.Before(s.ExpiresAt)
}

func (s *UserSubscription) DaysRemaining() int {
	return s.daysRemainingAt(time.Now())
}

func (s *UserSubscription) daysRemainingAt(now time.Time) int {
	remaining := s.ExpiresAt.Sub(now)
	if remaining <= 0 {
		return 0
	}

	days := int(remaining / subscriptionDayDuration)
	if remaining%subscriptionDayDuration != 0 {
		days++
	}
	return days
}

func (s *UserSubscription) IsWindowActivated() bool {
	return s.DailyWindowStart != nil || s.WeeklyWindowStart != nil || s.MonthlyWindowStart != nil
}

func (s *UserSubscription) HasOneTimeDailyQuota() bool {
	if s == nil || s.StartsAt.IsZero() || s.ExpiresAt.IsZero() {
		return false
	}
	return !s.ExpiresAt.After(s.StartsAt.AddDate(0, 0, 1))
}

func (s *UserSubscription) NeedsDailyReset() bool {
	return s.NeedsDailyResetAt(time.Now())
}

func (s *UserSubscription) NeedsDailyResetAt(now time.Time) bool {
	if s.DailyWindowStart == nil {
		return false
	}
	if s.IsExpiredAt(now) {
		return false
	}
	if s.HasOneTimeDailyQuota() {
		return false
	}
	resetAt := s.DailyResetTime()
	return resetAt != nil && !now.Before(*resetAt)
}

func (s *UserSubscription) NeedsWeeklyReset() bool {
	return s.NeedsWeeklyResetAt(time.Now())
}

func (s *UserSubscription) NeedsWeeklyResetAt(now time.Time) bool {
	if s.WeeklyWindowStart == nil {
		return false
	}
	if s.IsExpiredAt(now) {
		return false
	}
	resetAt := s.WeeklyResetTime()
	return resetAt != nil && !now.Before(*resetAt)
}

func (s *UserSubscription) NeedsMonthlyReset() bool {
	return s.NeedsMonthlyResetAt(time.Now())
}

func (s *UserSubscription) NeedsMonthlyResetAt(now time.Time) bool {
	if s.MonthlyWindowStart == nil {
		return false
	}
	if s.IsExpiredAt(now) {
		return false
	}
	resetAt := s.MonthlyResetTime()
	return resetAt != nil && !now.Before(*resetAt)
}

func (s *UserSubscription) DailyResetTime() *time.Time {
	windowStart := s.effectiveWindowStart(s.DailyWindowStart)
	if windowStart == nil {
		return nil
	}
	if s.HasOneTimeDailyQuota() {
		t := s.ExpiresAt
		return &t
	}
	t := s.resetTimeAfterExpiryBoundary(windowStart.Add(24 * time.Hour))
	return &t
}

func (s *UserSubscription) AdvancedDailyWindowStartAt(now time.Time) *time.Time {
	return s.advanceWindowStartAt(s.DailyWindowStart, 24*time.Hour, now)
}

func (s *UserSubscription) WeeklyResetTime() *time.Time {
	windowStart := s.effectiveWindowStart(s.WeeklyWindowStart)
	if windowStart == nil {
		return nil
	}
	t := s.resetTimeAfterExpiryBoundary(windowStart.Add(7 * 24 * time.Hour))
	return &t
}

func (s *UserSubscription) AdvancedWeeklyWindowStartAt(now time.Time) *time.Time {
	return s.advanceWindowStartAt(s.WeeklyWindowStart, 7*24*time.Hour, now)
}

func (s *UserSubscription) MonthlyResetTime() *time.Time {
	windowStart := s.effectiveWindowStart(s.MonthlyWindowStart)
	if windowStart == nil {
		return nil
	}
	t := s.resetTimeAfterExpiryBoundary(windowStart.Add(30 * 24 * time.Hour))
	return &t
}

func (s *UserSubscription) AdvancedMonthlyWindowStartAt(now time.Time) *time.Time {
	return s.advanceWindowStartAt(s.MonthlyWindowStart, 30*24*time.Hour, now)
}

func (s *UserSubscription) effectiveWindowStart(windowStart *time.Time) *time.Time {
	if s == nil || windowStart == nil {
		return windowStart
	}
	if !s.StartsAt.IsZero() && windowStart.Before(s.StartsAt) {
		t := s.StartsAt
		return &t
	}
	return windowStart
}

func (s *UserSubscription) advanceWindowStartAt(windowStart *time.Time, period time.Duration, now time.Time) *time.Time {
	effectiveStart := s.effectiveWindowStart(windowStart)
	if effectiveStart == nil || period <= 0 {
		return effectiveStart
	}
	if now.Before(*effectiveStart) {
		return effectiveStart
	}
	periods := now.Sub(*effectiveStart) / period
	if periods <= 0 {
		return effectiveStart
	}
	t := effectiveStart.Add(time.Duration(periods) * period)
	return &t
}

func (s *UserSubscription) resetTimeAfterExpiryBoundary(resetAt time.Time) time.Time {
	if s == nil || s.ExpiresAt.IsZero() {
		return resetAt
	}
	if resetAt.Equal(s.ExpiresAt) {
		return s.ExpiresAt.Add(subscriptionResetAfterExpiryBoundaryDelay)
	}
	return resetAt
}

func (s *UserSubscription) CheckDailyLimit(group *Group, additionalCost float64) bool {
	if !group.HasDailyLimit() {
		return true
	}
	if additionalCost <= 0 {
		return s.DailyUsageUSD < *group.DailyLimitUSD
	}
	return s.DailyUsageUSD+additionalCost <= *group.DailyLimitUSD
}

func (s *UserSubscription) CheckWeeklyLimit(group *Group, additionalCost float64) bool {
	if !group.HasWeeklyLimit() {
		return true
	}
	if additionalCost <= 0 {
		return s.WeeklyUsageUSD < *group.WeeklyLimitUSD
	}
	return s.WeeklyUsageUSD+additionalCost <= *group.WeeklyLimitUSD
}

func (s *UserSubscription) CheckMonthlyLimit(group *Group, additionalCost float64) bool {
	if !group.HasMonthlyLimit() {
		return true
	}
	if additionalCost <= 0 {
		return s.MonthlyUsageUSD < *group.MonthlyLimitUSD
	}
	return s.MonthlyUsageUSD+additionalCost <= *group.MonthlyLimitUSD
}

func (s *UserSubscription) CheckAllLimits(group *Group, additionalCost float64) (daily, weekly, monthly bool) {
	daily = s.CheckDailyLimit(group, additionalCost)
	weekly = s.CheckWeeklyLimit(group, additionalCost)
	monthly = s.CheckMonthlyLimit(group, additionalCost)
	return
}
