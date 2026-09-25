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
	require.Equal(t, 60.0, recharge)
	require.Equal(t, 30.0, subscription)
	require.Equal(t, int64(26), total)
	require.Equal(t, int64(6), paid)
	require.Equal(t, 200.0, excluded)
	// A day containing only unsuccessful orders must not report recharge credits.
	_, err = db.Exec(`UPDATE payment_orders SET status = 'PENDING'`)
	require.NoError(t, err)
	err = db.QueryRow(query, "2026-09-01 00:00:00", "2026-09-02 00:00:00", "+0 days", service.OrderStatusPaid, service.OrderStatusRecharging, service.OrderStatusCompleted).
		Scan(&day, &recharge, &subscription, &total, &paid, &excluded)
	require.NoError(t, err)
	require.Zero(t, recharge)
	require.Zero(t, subscription)
	require.Zero(t, paid)
	require.Equal(t, 260.0, excluded)
}
