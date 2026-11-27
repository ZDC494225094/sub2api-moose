package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOperationsReportingRangeIgnoresBrowserTimezone(t *testing.T) {
	now := time.Date(2026, 9, 14, 18, 0, 0, 0, time.UTC)
	var expectedStart, expectedEnd time.Time
	for _, browserTZ := range []string{"America/New_York", "Asia/Shanghai", "Pacific/Honolulu"} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?preset=today&timezone="+browserTZ, nil)
		start, end, err := parseOperationsReportingRange(c, now)
		require.NoError(t, err)
		if expectedStart.IsZero() {
			expectedStart, expectedEnd = start, end
		}
		require.Equal(t, expectedStart, start)
		require.Equal(t, expectedEnd, end)
	}
}

func TestOperationsReportingCustomRangeAndPresets(t *testing.T) {
	now := time.Date(2026, 9, 14, 18, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		query string
		valid bool
		days  int
	}{
		{"preset=7d", true, 7}, {"preset=30d", true, 30}, {"preset=yesterday", true, 1},
		{"start_date=2026-09-01&end_date=2026-09-01", true, 1},
		{"start_date=2026-09-02&end_date=2026-09-01", false, 0},
		{"start_date=2026-01-01&end_date=2026-09-01", false, 0},
		{"start_date=2026-09-01", false, 0}, {"preset=bad", false, 0},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?"+test.query, nil)
		start, end, err := parseOperationsReportingRange(c, now)
		if !test.valid {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, start.AddDate(0, 0, test.days), end)
	}
}

func TestBalanceHistoryDateSelection(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	now := time.Date(2026, 1, 5, 3, 0, 0, 0, loc)
	for _, tc := range []struct{ query, start, end, unit string }{
		{"months=3", "2025-11-01", "2026-01-05", "month"},
		{"start_date=2024-02-29&end_date=2026-01-05&granularity=range", "2024-02-29", "2026-01-05", "range"},
		{"months=4", "", "", ""},
		{"months=3&start_date=2025-11-01&end_date=2026-01-05", "", "", ""},
		{"start_date=2025-02-29&end_date=2025-03-01", "", "", ""},
		{"start_date=2026-01-01", "", "", ""},
		{"start_date=2026-01-07&end_date=2026-01-01", "", "", ""},
		{"start_date=2026-01-01&end_date=2026-01-06", "", "", ""},
		{"granularity=hour", "", "", ""},
	} {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("GET", "/?"+tc.query, nil)
		start, end, unit, err := parseBalanceHistoryRange(c, now)
		if tc.unit == "" {
			require.Error(t, err, tc.query)
			continue
		}
		require.NoError(t, err)
		require.Equal(t, tc.unit, unit)
		require.Equal(t, tc.start, start.Format(time.DateOnly))
		require.Equal(t, tc.end, end.Format(time.DateOnly))
	}
}
