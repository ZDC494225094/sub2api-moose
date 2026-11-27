package admin

import (
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/gin-gonic/gin"
)

func (h *OperationsHandler) GetOperationsBalanceHistory(c *gin.Context) {
	start, end, unit, err := parseBalanceHistoryRange(c, timezone.Now().In(operationsReportingLocation()))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	points, err := h.balanceSnapshotService.GetHistory(c.Request.Context(), start, end, unit)
	if err != nil {
		slog.Error("operations_balance_history_failed", "error", err)
		response.Error(c, 500, "Failed to get operations balance history")
		return
	}
	// Existing months-only clients expect numeric totals on every returned row.
	if !c.Request.URL.Query().Has("granularity") && !c.Request.URL.Query().Has("start_date") {
		complete := points[:0]
		for _, point := range points {
			if point.TotalBalance != nil && point.TotalSubscriptionRemaining != nil {
				complete = append(complete, point)
			}
		}
		points = complete
	}
	response.Success(c, points)
}

func parseBalanceHistoryRange(c *gin.Context, now time.Time) (time.Time, time.Time, string, error) {
	unit := c.DefaultQuery("granularity", "month")
	switch unit {
	case "day", "week", "month", "range":
	default:
		return time.Time{}, time.Time{}, "", fmt.Errorf("granularity must be day, week, month or range")
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	params := c.Request.URL.Query()
	if params.Has("start_date") || params.Has("end_date") {
		if params.Has("months") {
			return time.Time{}, time.Time{}, "", fmt.Errorf("choose a month preset or a date range")
		}
		start, e1 := time.ParseInLocation(time.DateOnly, c.Query("start_date"), now.Location())
		end, e2 := time.ParseInLocation(time.DateOnly, c.Query("end_date"), now.Location())
		if e1 != nil || e2 != nil || start.Year() < 1 || end.Before(start) || end.After(today) {
			return time.Time{}, time.Time{}, "", fmt.Errorf("select valid start_date and end_date through today")
		}
		return start, end, unit, nil
	}
	months, err := strconv.Atoi(c.DefaultQuery("months", "12"))
	if err != nil || (months != 3 && months != 6 && months != 12) {
		return time.Time{}, time.Time{}, "", fmt.Errorf("months must be 3, 6 or 12")
	}
	return time.Date(now.Year(), now.Month()+1-time.Month(months), 1, 0, 0, 0, 0, now.Location()), today, unit, nil
}
