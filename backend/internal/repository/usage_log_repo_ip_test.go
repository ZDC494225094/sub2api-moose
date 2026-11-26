package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAppendUsageLogIPWhereCondition(t *testing.T) {
	conditions, args := appendUsageLogIPWhereCondition(nil, nil, " 203.0.113.7 ", "")
	require.Equal(t, []string{"ip_address = $1"}, conditions)
	require.Equal(t, []any{"203.0.113.7"}, args)

	conditions, args = appendUsageLogIPWhereCondition(nil, nil, "not-an-ip", "")
	require.Empty(t, conditions)
	require.Empty(t, args)

	conditions, args = appendUsageLogIPWhereCondition(nil, []any{int64(7)}, "2001:db8::1", "ul")
	require.Equal(t, []string{"ul.ip_address = $2"}, conditions)
	require.Equal(t, []any{int64(7), "2001:db8::1"}, args)
}

func TestUsageLogRepositoryListSharedIPs(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	lastSeen := time.Date(2026, 11, 24, 8, 30, 0, 0, time.UTC)

	mock.ExpectQuery("COUNT\\(DISTINCT user_id\\)").
		WithArgs(start, end, 2, 50).
		WillReturnRows(sqlmock.NewRows([]string{"ip_address", "user_count", "request_count", "last_seen"}).
			AddRow("203.0.113.7", 3, 21, lastSeen))

	stats, err := repo.ListSharedIPs(context.Background(), start, end, 2, 50)
	require.NoError(t, err)
	require.Len(t, stats, 1)
	require.Equal(t, "203.0.113.7", stats[0].IPAddress)
	require.Equal(t, 3, stats[0].UserCount)
	require.Equal(t, int64(21), stats[0].RequestCount)
	require.True(t, stats[0].LastSeen.Equal(lastSeen))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryListIPSharingUsers(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}

	start := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery("LEFT JOIN users").
		WithArgs(start, end, "203.0.113.7", "203.0.113.8").
		WillReturnRows(sqlmock.NewRows([]string{"ip_address", "user_id", "email", "username", "requests"}).
			AddRow("203.0.113.7", 11, "a@example.com", "a", 12).
			AddRow("203.0.113.7", 12, "b@example.com", "b", 6).
			AddRow("203.0.113.8", 13, "c@example.com", "c", 2))

	result, err := repo.ListIPSharingUsers(context.Background(), []string{"203.0.113.7", "203.0.113.8"}, start, end)
	require.NoError(t, err)
	require.Len(t, result["203.0.113.7"], 2)
	require.Len(t, result["203.0.113.8"], 1)
	require.Equal(t, int64(11), result["203.0.113.7"][0].UserID)
	require.Equal(t, "a@example.com", result["203.0.113.7"][0].Email)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageLogRepositoryListIPSharingUsersEmpty(t *testing.T) {
	repo := &usageLogRepository{}
	result, err := repo.ListIPSharingUsers(context.Background(), nil, time.Now(), time.Now())
	require.NoError(t, err)
	require.Empty(t, result)
}
