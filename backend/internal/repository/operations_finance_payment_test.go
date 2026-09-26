package repository

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// Execute the production payment aggregation against isolated fixtures. Only the
// PostgreSQL date formatter and placeholder syntax are adapted for SQLite;
// status/type filters, amount expressions, counts and interval bounds are unchanged.
func TestOperationsFinanceSuccessfulPaymentAggregation(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE payment_orders (created_at TEXT, amount REAL, order_type TEXT, status TEXT)`)
	require.NoError(t, err)
	statuses := []string{"PENDING", "PAID", "RECHARGING", "COMPLETED", "EXPIRED", "CANCELLED", "FAILED", "REFUND_REQUESTED", "REFUNDING", "REFUND_PENDING", "PARTIALLY_REFUNDED", "REFUNDED", "REFUND_FAILED"}
	for _, status := range statuses {
		for _, kind := range []string{"balance", "subscription", "other"} {
			_, err := db.Exec(`INSERT INTO payment_orders VALUES (?, ?, ?, ?)`, "2026-09-01 00:00:00", 10, kind, status)
			require.NoError(t, err)
		}
	}
	for _, at := range []string{"2026-08-31 23:59:59", "2026-09-02 00:00:00"} {
		_, err := db.Exec(`INSERT INTO payment_orders VALUES (?, 9999, 'balance', 'COMPLETED')`, at)
		require.NoError(t, err)
	}
	query := strings.ReplaceAll(operationsFinancePaymentsQuery, "to_char(created_at AT TIME ZONE $3, 'YYYY-MM-DD')", "date(created_at, $3)")
	for _, n := range []string{"1", "2", "3", "4", "5", "6"} {
		query = strings.ReplaceAll(query, "$"+n, "?"+n)
	}
	var day string
	var recharge, subscription, excluded float64
	var total, paid int64
	err = db.QueryRow(query, "2026-09-01 00:00:00", "2026-09-02 00:00:00", "+0 days", service.OrderStatusPaid, service.OrderStatusRecharging, service.OrderStatusCompleted).
		Scan(&day, &recharge, &subscription, &total, &paid, &excluded)
	require.NoError(t, err)
	require.Equal(t, "2026-09-01", day)
	require.Equal(t, 10.0, recharge)
	require.Equal(t, 30.0, subscription)
	require.Equal(t, int64(26), total)
	require.Equal(t, int64(6), paid)
	require.Equal(t, 100.0, excluded)
	// A day containing only unsuccessful orders must not report recharge credits.
	_, err = db.Exec(`UPDATE payment_orders SET status = 'PENDING'`)
	require.NoError(t, err)
	err = db.QueryRow(query, "2026-09-01 00:00:00", "2026-09-02 00:00:00", "+0 days", service.OrderStatusPaid, service.OrderStatusRecharging, service.OrderStatusCompleted).
		Scan(&day, &recharge, &subscription, &total, &paid, &excluded)
	require.NoError(t, err)
	require.Zero(t, recharge)
	require.Zero(t, subscription)
	require.Zero(t, paid)
	require.Equal(t, 130.0, excluded)
}

// Only dialect-specific date/currency normalization syntax is translated. The
// payment/credit expressions and every production success/type/range filter run unchanged.
func TestOperationsFinanceCashIsNotCreditedAmount(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE payment_orders (created_at TEXT, amount REAL, pay_amount REAL, order_type TEXT, status TEXT, provider_snapshot TEXT);
 INSERT INTO payment_orders VALUES
 ('2026-09-01',100,70,'balance','COMPLETED','{"currency":" cny "}'),
 ('2026-09-01',60,42,'balance','PAID',NULL),
 ('2026-09-01',30,21,'balance','RECHARGING','{}'),
 ('2026-09-01',199,150,'subscription','COMPLETED','{"currency":"CNY"}'),
 ('2026-09-01',5,4,'balance','COMPLETED','{"currency":"USD"}'),
 ('2026-09-01',999,888,'balance','PENDING','{}'),
 ('2026-09-01',999,888,'balance','FAILED','{}'),
 ('2026-09-01',999,888,'balance','REFUNDED','{}'),
 ('2026-09-01',999,888,'other','COMPLETED','{}'),
 ('2026-09-02',999,888,'balance','COMPLETED','{}');`)
	require.NoError(t, err)
	query := strings.ReplaceAll(operationsFinanceCashQuery, "to_char(created_at AT TIME ZONE $3, 'YYYY-MM-DD')", "date(created_at, $3)")
	query = strings.ReplaceAll(query, "upper(btrim(provider_snapshot->>'currency')) ~ '^[A-Z]{3}$'", "length(trim(provider_snapshot->>'currency')) = 3 AND upper(trim(provider_snapshot->>'currency')) NOT GLOB '*[^A-Z]*'")
	query = strings.ReplaceAll(query, "btrim(", "trim(")
	for _, n := range []string{"1", "2", "3", "4", "5", "6"} {
		query = strings.ReplaceAll(query, "$"+n, "?"+n)
	}
	rows, err := db.Query(query, "2026-09-01", "2026-09-02", "+0 days", service.OrderStatusPaid, service.OrderStatusRecharging, service.OrderStatusCompleted)
	require.NoError(t, err)
	defer rows.Close()
	result := map[string]service.OperationsPaymentDay{}
	for rows.Next() {
		var row service.OperationsPaymentDay
		require.NoError(t, rows.Scan(&row.Day, &row.Currency, &row.RechargePaid, &row.SubscriptionPaid, &row.Credited, &row.PendingCredit))
		result[row.Currency] = row
	}
	require.NoError(t, rows.Err())
	require.Len(t, result, 2)
	require.Equal(t, 133.0, result["CNY"].RechargePaid)
	require.Equal(t, 150.0, result["CNY"].SubscriptionPaid)
	require.Equal(t, 100.0, result["CNY"].Credited)
	require.Equal(t, 90.0, result["CNY"].PendingCredit)
	require.Equal(t, 4.0, result["USD"].RechargePaid)
	require.Equal(t, 5.0, result["USD"].Credited)
}
