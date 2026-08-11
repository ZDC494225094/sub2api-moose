package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/paymentauditlog"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
)

// --- Dashboard & Analytics ---

func (s *PaymentService) GetDashboardStats(ctx context.Context, days int, startDate, endDate string) (*DashboardStats, error) {
	now := time.Now()
	since, until, days := dashboardDateRange(now, days, startDate, endDate)

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	paidStatuses := []string{OrderStatusCompleted, OrderStatusPaid, OrderStatusRecharging}

	orders, err := s.entClient.PaymentOrder.Query().
		Where(
			paymentorder.StatusIn(paidStatuses...),
			paymentorder.PaidAtGTE(since),
			paymentorder.PaidAtLT(until),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	// collect unique user IDs to determine first-time payers
	userIDs := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, o := range orders {
		if _, ok := seen[o.UserID]; !ok {
			seen[o.UserID] = struct{}{}
			userIDs = append(userIDs, o.UserID)
		}
	}

	// for each user, find their earliest ever paid order date
	firstPayMap := make(map[int64]time.Time)
	if len(userIDs) > 0 {
		allPriorOrders, err2 := s.entClient.PaymentOrder.Query().
			Where(
				paymentorder.StatusIn(paidStatuses...),
				paymentorder.UserIDIn(userIDs...),
			).
			All(ctx)
		if err2 == nil {
			for _, o := range allPriorOrders {
				if o.PaidAt == nil {
					continue
				}
				if t, ok := firstPayMap[o.UserID]; !ok || o.PaidAt.Before(t) {
					firstPayMap[o.UserID] = *o.PaidAt
				}
			}
		}
	}

	st := &DashboardStats{}
	computeBasicStats(st, orders, todayStart)

	st.PendingOrders, err = s.entClient.PaymentOrder.Query().
		Where(paymentorder.StatusEQ(OrderStatusPending)).
		Count(ctx)
	if err != nil {
		return nil, err
	}

	st.DailySeries = buildDailySeries(orders, since, days, firstPayMap)
	st.PaymentMethods = buildMethodDistribution(orders)
	st.TopUsers = buildTopUsers(orders)

	return st, nil
}

func dashboardDateRange(now time.Time, days int, startDate, endDate string) (since, until time.Time, dateCount int) {
	if startDate != "" && endDate != "" {
		parsedSince, sinceErr := time.ParseInLocation("2006-01-02", startDate, now.Location())
		parsedEnd, endErr := time.ParseInLocation("2006-01-02", endDate, now.Location())
		if sinceErr == nil && endErr == nil {
			since = parsedSince
			until = parsedEnd.AddDate(0, 0, 1)
			dateCount = int(until.Sub(since).Hours() / 24)
			if dateCount > 0 {
				return since, until, dateCount
			}
		}
	}

	if days <= 0 {
		days = 30
	}
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	since = todayStart.AddDate(0, 0, -days+1)
	until = todayStart.AddDate(0, 0, 1)
	return since, until, days
}

func computeBasicStats(st *DashboardStats, orders []*dbent.PaymentOrder, todayStart time.Time) {
	st.TotalAmount = make(CurrencyAmounts)
	st.TodayAmount = make(CurrencyAmounts)
	st.AvgAmount = make(CurrencyAmounts)
	currencyCounts := make(map[string]int)
	var todayCount int
	for _, o := range orders {
		currency := PaymentOrderCurrency(o)
		st.TotalAmount[currency] += o.PayAmount
		currencyCounts[currency]++
		if o.PaidAt != nil && !o.PaidAt.Before(todayStart) {
			st.TodayAmount[currency] += o.PayAmount
			todayCount++
		}
	}
	st.TotalCount = len(orders)
	st.TodayCount = todayCount
	for currency, totalAmount := range st.TotalAmount {
		st.AvgAmount[currency] = roundAmount(totalAmount / float64(currencyCounts[currency]))
	}
	roundCurrencyAmounts(st.TotalAmount)
	roundCurrencyAmounts(st.TodayAmount)
}

func buildDailySeries(orders []*dbent.PaymentOrder, since time.Time, days int, firstPayMap map[int64]time.Time) []DailyStats {
	dailyMap := make(map[string]*DailyStats)

	// track which users already counted per day to avoid double-counting
	type dayUser struct {
		date   string
		userID int64
	}
	seenDayUser := make(map[dayUser]struct{})

	for _, o := range orders {
		if o.PaidAt == nil {
			continue
		}
		date := o.PaidAt.Format("2006-01-02")
		ds, ok := dailyMap[date]
		if !ok {
			ds = newDailyStats(date)
			dailyMap[date] = ds
		}
		currency := PaymentOrderCurrency(o)
		ds.Amount[currency] += o.PayAmount
		ds.Count++

		if o.OrderType == "balance" {
			ds.BalanceAmount[currency] += o.PayAmount
			ds.BalanceCount++
		} else {
			ds.SubscriptionAmount[currency] += o.PayAmount
			ds.SubscriptionCount++
		}

		du := dayUser{date: date, userID: o.UserID}
		if _, alreadyCounted := seenDayUser[du]; !alreadyCounted {
			seenDayUser[du] = struct{}{}
			firstEver, exists := firstPayMap[o.UserID]
			// new user if today is their first-ever paid order
			if exists && firstEver.Format("2006-01-02") == date {
				ds.NewUserCount++
				ds.NewUserAmount[currency] += o.PayAmount
			} else {
				ds.ReturningUserCount++
				ds.ReturningUserAmount[currency] += o.PayAmount
			}
		}
	}

	series := make([]DailyStats, 0, days)
	for i := 0; i < days; i++ {
		date := since.AddDate(0, 0, i).Format("2006-01-02")
		if ds, ok := dailyMap[date]; ok {
			roundCurrencyAmounts(ds.Amount)
			roundCurrencyAmounts(ds.BalanceAmount)
			roundCurrencyAmounts(ds.SubscriptionAmount)
			roundCurrencyAmounts(ds.NewUserAmount)
			roundCurrencyAmounts(ds.ReturningUserAmount)
			series = append(series, *ds)
		} else {
			series = append(series, *newDailyStats(date))
		}
	}
	return series
}

func newDailyStats(date string) *DailyStats {
	return &DailyStats{
		Date:                date,
		Amount:              make(CurrencyAmounts),
		BalanceAmount:       make(CurrencyAmounts),
		SubscriptionAmount:  make(CurrencyAmounts),
		NewUserAmount:       make(CurrencyAmounts),
		ReturningUserAmount: make(CurrencyAmounts),
	}
}

func buildMethodDistribution(orders []*dbent.PaymentOrder) []PaymentMethodStat {
	methodMap := make(map[string]*PaymentMethodStat)
	for _, o := range orders {
		ms, ok := methodMap[o.PaymentType]
		if !ok {
			ms = &PaymentMethodStat{Type: o.PaymentType, Amount: make(CurrencyAmounts)}
			methodMap[o.PaymentType] = ms
		}
		ms.Amount[PaymentOrderCurrency(o)] += o.PayAmount
		ms.Count++
	}
	methods := make([]PaymentMethodStat, 0, len(methodMap))
	for _, ms := range methodMap {
		roundCurrencyAmounts(ms.Amount)
		methods = append(methods, *ms)
	}
	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Type < methods[j].Type
	})
	return methods
}

func buildTopUsers(orders []*dbent.PaymentOrder) TopUsersByCurrency {
	userMap := make(map[string]map[int64]*TopUserStat)
	for _, o := range orders {
		currency := PaymentOrderCurrency(o)
		users, ok := userMap[currency]
		if !ok {
			users = make(map[int64]*TopUserStat)
			userMap[currency] = users
		}
		us, ok := users[o.UserID]
		if !ok {
			us = &TopUserStat{UserID: o.UserID, Email: o.UserEmail}
			users[o.UserID] = us
		}
		us.Amount += o.PayAmount
	}
	result := make(TopUsersByCurrency, len(userMap))
	for currency, users := range userMap {
		userList := make([]*TopUserStat, 0, len(users))
		for _, us := range users {
			us.Amount = roundAmount(us.Amount)
			userList = append(userList, us)
		}
		sort.Slice(userList, func(i, j int) bool {
			return userList[i].Amount > userList[j].Amount
		})
		limit := topUsersLimit
		if len(userList) < limit {
			limit = len(userList)
		}
		result[currency] = make([]TopUserStat, 0, limit)
		for i := 0; i < limit; i++ {
			result[currency] = append(result[currency], *userList[i])
		}
	}
	return result
}

func roundCurrencyAmounts(amounts CurrencyAmounts) {
	for currency, amount := range amounts {
		amounts[currency] = roundAmount(amount)
	}
}

func roundAmount(amount float64) float64 {
	return math.Round(amount*100) / 100
}

// --- Audit Logs ---

func (s *PaymentService) writeAuditLog(ctx context.Context, oid int64, action, op string, detail map[string]any) {
	dj, _ := json.Marshal(detail)
	_, err := s.entClient.PaymentAuditLog.Create().SetOrderID(strconv.FormatInt(oid, 10)).SetAction(action).SetDetail(string(dj)).SetOperator(op).Save(ctx)
	if err != nil {
		slog.Error("audit log failed", "orderID", oid, "action", action, "error", err)
	}
}

func (s *PaymentService) GetOrderAuditLogs(ctx context.Context, oid int64) ([]*dbent.PaymentAuditLog, error) {
	return s.entClient.PaymentAuditLog.Query().Where(paymentauditlog.OrderIDEQ(strconv.FormatInt(oid, 10))).Order(paymentauditlog.ByCreatedAt()).All(ctx)
}
