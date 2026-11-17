package repository

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogCanvasFilterKeepsActualAndAccountCostsSeparate(t *testing.T) {
	for _, dimension := range []string{"model", "inbound_endpoint", "upstream_endpoint"} {
		for _, canvas := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/canvas=%t", dimension, canvas), func(t *testing.T) {
				db, mock := newSQLMock(t)
				repo := &usageLogRepository{sql: db}
				start := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
				end := start.Add(24 * time.Hour)
				filter := "EXISTS (SELECT 1 FROM api_keys canvas_key WHERE canvas_key.id = api_key_id AND LEFT(canvas_key.name, LENGTH($4)) = $4)"
				if !canvas {
					filter = "NOT " + filter
				}
				query := regexp.QuoteMeta("COALESCE(SUM(actual_cost), 0) as actual_cost,") +
					`\s+` + regexp.QuoteMeta("COALESCE(SUM(COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1)), 0) as account_cost") +
					`.*` + regexp.QuoteMeta("AND account_id = $3 AND "+filter)
				expect := mock.ExpectQuery(query).WithArgs(start, end, int64(42), service.CanvasManagedAPIKeyNamePrefix)
				if dimension == "model" {
					expect.WillReturnRows(sqlmock.NewRows([]string{
						"model", "requests", "input_tokens", "output_tokens", "cache_creation_tokens", "cache_read_tokens", "total_tokens", "cost", "actual_cost", "account_cost",
					}).AddRow("gpt-image-2", 3, 10, 20, 0, 0, 30, 6.0, 2.4, 0.9))
					rows, err := repo.GetModelStatsWithUsageFiltersBySource(context.Background(), start, end,
						usagestats.UsageLogFilters{AccountID: 42, CanvasManaged: &canvas}, usagestats.ModelSourceRequested)
					require.NoError(t, err)
					require.Len(t, rows, 1)
					require.Equal(t, 2.4, rows[0].ActualCost)
					require.Equal(t, 0.9, rows[0].AccountCost)
				} else {
					expect.WillReturnRows(sqlmock.NewRows([]string{"endpoint", "requests", "total_tokens", "cost", "actual_cost", "account_cost"}).
						AddRow("/v1/images/generations", 3, 30, 6.0, 2.4, 0.9))
					rows, err := repo.getEndpointStatsByColumnWithFilters(context.Background(), dimension, start, end, 0, 0, 42, 0, "", "", nil, nil, nil, "", &canvas)
					require.NoError(t, err)
					require.Len(t, rows, 1)
					require.Equal(t, 2.4, rows[0].ActualCost)
					require.Equal(t, 0.9, rows[0].AccountCost)
				}
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
