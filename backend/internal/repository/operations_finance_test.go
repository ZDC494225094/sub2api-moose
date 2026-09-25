package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Opt-in read-only verification against the configured database. No migrations or writes.
func TestOperationsFinanceReadOnlyDatabase(t *testing.T) {
	if os.Getenv("OPERATIONS_FINANCE_VERIFY") != "1" {
		t.Skip("set OPERATIONS_FINANCE_VERIFY=1 for configured database verification")
	}
	content, err := os.ReadFile("../../config.yaml")
	require.NoError(t, err)
	var config struct {
		Database struct {
			Host, User, Password, DBName, SSLMode string
			Port                                  int
		}
	}
	require.NoError(t, yaml.Unmarshal(content, &config))
	c := config.Database
	dsn := &url.URL{Scheme: "postgres", Host: fmt.Sprintf("%s:%d", c.Host, c.Port), Path: c.DBName, User: url.UserPassword(c.User, c.Password)}
	q := dsn.Query()
	q.Set("sslmode", c.SSLMode)
	q.Set("connect_timeout", "8")
	q.Set("options", "-c default_transaction_read_only=on -c statement_timeout=20000")
	dsn.RawQuery = q.Encode()
	db, err := sql.Open("postgres", dsn.String())
	require.NoError(t, err)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	// Prefer a non-empty payment day so reconciliation is not a zero-equals-zero check.
	var latestCreated sql.NullTime
	require.NoError(t, db.QueryRowContext(ctx, `SELECT max(created_at) FROM payment_orders`).Scan(&latestCreated))
	if latestCreated.Valid {
		paid := latestCreated.Time.In(loc)
		start = time.Date(paid.Year(), paid.Month(), paid.Day(), 0, 0, 0, 0, loc)
	}
	report, err := (&usageLogRepository{sql: db}).GetOperationsFinance(ctx, start, start.AddDate(0, 0, 1))
	require.NoError(t, err)
	var dailyCost, accountCost, upstreamCost, modelCost float64
	var dayCount int
	for _, row := range report.Rows {
		switch row.Dimension {
		case "day":
			dailyCost += row.Cost
			dayCount++
		case "account":
			accountCost += row.Cost
		case "upstream":
			upstreamCost += row.Cost
		case "model":
			modelCost += row.Cost
		}
	}
	require.Equal(t, 1, dayCount)
	require.InDelta(t, dailyCost, accountCost, 0.000001)
	require.InDelta(t, dailyCost, upstreamCost, 0.000001)
	require.InDelta(t, dailyCost, modelCost, 0.000001)
	// Reconcile against raw orders using the same created_at and order type range.
	orderRows, err := db.QueryContext(ctx, `SELECT amount FROM payment_orders
	 WHERE created_at >= $1 AND created_at < $2 AND order_type IN ('balance','subscription') AND status IN ('PAID','RECHARGING','COMPLETED')`, start, start.AddDate(0, 0, 1))
	require.NoError(t, err)
	credits := 0.0
	for orderRows.Next() {
		var amount float64
		require.NoError(t, orderRows.Scan(&amount))
		credits += amount
	}
	require.NoError(t, orderRows.Err())
	orderRows.Close()
	for _, row := range report.Rows {
		if row.Dimension == "day" {
			require.InDelta(t, credits, row.Recharge, 0.000001)
		}
	}
	var consumption, rawCost, listCost, balance float64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(sum(actual_cost),0),COALESCE(sum(total_cost),0),COALESCE(sum(COALESCE(account_stats_cost,total_cost)*COALESCE(account_rate_multiplier,1)),0) FROM usage_logs WHERE created_at >= $1 AND created_at < $2`, start, start.AddDate(0, 0, 1)).Scan(&consumption, &listCost, &rawCost))
	require.InDelta(t, rawCost, dailyCost, 0.000001)
	for _, row := range report.Rows {
		if row.Dimension == "day" {
			require.InDelta(t, consumption, row.Consumption, 0.000001)
			require.InDelta(t, listCost, row.ListCost, 0.000001)
		}
	}
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(sum(balance),0) FROM users WHERE deleted_at IS NULL`).Scan(&balance))
	require.InDelta(t, balance, report.CurrentBalance, 0.000001)
	t.Logf("reconciled %s: credit=%.4f", start.Format("2006-01-02"), credits)
	for _, segment := range []string{"repeat", "churned", "balance", "paying"} {
		customers, err := (&usageLogRepository{sql: db}).GetOperationsCustomers(ctx, service.OperationsCustomerFilter{
			Start: start.AddDate(0, 0, -29), End: start.AddDate(0, 0, 1), AsOf: now, ChurnDays: 30, Page: 1, PageSize: 20, Segment: segment,
		})
		require.NoError(t, err)
		switch segment {
		case "repeat":
			require.Equal(t, customers.Summary.Repeat, customers.Total)
		case "churned":
			require.Equal(t, customers.Summary.Churned, customers.Total)
		case "balance":
			require.Equal(t, customers.Summary.BalanceUsers, customers.Total)
		case "paying":
			require.Equal(t, customers.Summary.Paying, customers.Total)
		}
		require.LessOrEqual(t, customers.Summary.Repeat, customers.Summary.Paying)
		require.LessOrEqual(t, customers.Summary.Churned, customers.Summary.PreviousActive)
		for _, user := range customers.Items {
			if segment == "repeat" {
				require.True(t, user.Repeat)
				require.GreaterOrEqual(t, user.TotalOrders, int64(2))
				require.Positive(t, user.PeriodOrders)
			}
			if segment == "churned" {
				require.True(t, user.Churned)
				require.True(t, user.LastUsedAt.Before(now.AddDate(0, 0, -30)))
			}
		}
	}
}
