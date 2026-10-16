package redeemmarketing

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestEligibilityBoundaryAndHistoricalPolicy(t *testing.T) {
	cutoff := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	for _, delta := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
		t.Run(delta.String(), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			// No settings query: disabling the plugin cannot erase a policy.
			mock.ExpectQuery("SELECT registered_after").WithArgs("batch").WillReturnRows(sqlmock.NewRows([]string{"registered_after"}).AddRow(cutoff))
			err = CheckEligibility(context.Background(), db, "batch", cutoff.Add(delta))
			if delta < 0 {
				require.ErrorContains(t, err, "仅限")
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT registered_after").WillReturnRows(sqlmock.NewRows([]string{"registered_after"}))
	require.NoError(t, CheckEligibility(context.Background(), db, "legacy", cutoff.Add(-time.Hour)))
	mock.ExpectQuery("SELECT registered_after").WillReturnError(errors.New("database unavailable"))
	require.Error(t, CheckEligibility(context.Background(), db, "batch", cutoff))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAdmissionAndReportScan(t *testing.T) {
	for _, value := range []string{"false", "", "malformed"} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectQuery("SELECT value").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow(value))
		_, err = ListBatches(context.Background(), db, "", 1, 20)
		require.Error(t, err)
		require.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	}
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT value").WillReturnRows(sqlmock.NewRows([]string{"value"}).AddRow("true"))
	now := time.Now().UTC()
	mock.ExpectQuery(regexp.QuoteMeta(batchQuery)).WithArgs("batch", 20, 20).WillReturnRows(sqlmock.NewRows([]string{"batch_id", "created_at", "registered_after", "code_count", "redeemed_users", "redeemed_value", "paying_users", "order_count", "pay_amounts"}).AddRow("batch", now, nil, 10, 3, 30, 2, 4, `{"CNY":12.5,"USD":2}`))
	items, err := ListBatches(context.Background(), db, "batch", 2, 20)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Nil(t, items[0].RegisteredAfter)
	require.Equal(t, int64(2), items[0].PayingUsers)
	require.Equal(t, map[string]float64{"CNY": 12.5, "USD": 2}, items[0].PayAmounts)
	require.NoError(t, mock.ExpectationsWereMet())
}
