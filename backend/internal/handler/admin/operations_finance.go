package admin

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *DashboardHandler) GetOperationsFinance(c *gin.Context) {
	start, end, err := parseOperationsReportingRange(c, timezone.Now())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// Unlike the legacy funnel, never silently truncate a requested finance period.
	if requested := c.Query("start_date"); requested != "" && requested != start.Format("2006-01-02") {
		response.BadRequest(c, "Date range must not exceed 90 days")
		return
	}
	result, err := h.dashboardService.GetOperationsFinance(c.Request.Context(), start, end)
	if err != nil {
		slog.Error("operations_finance_failed", "error", err)
		response.Error(c, 500, "Failed to get operations finance")
		return
	}
	response.Success(c, result)
}

func (h *DashboardHandler) GetOperationsCustomers(c *gin.Context) {
	start, end, err := parseOperationsReportingRange(c, timezone.Now())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if requested := c.Query("start_date"); requested != "" && requested != start.Format("2006-01-02") {
		response.BadRequest(c, "Date range must not exceed 90 days")
		return
	}
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 1000000 {
		response.BadRequest(c, "Invalid page")
		return
	}
	churnDays, err := strconv.Atoi(c.DefaultQuery("churn_days", "30"))
	if err != nil || (churnDays != 7 && churnDays != 30 && churnDays != 60 && churnDays != 90) {
		response.BadRequest(c, "Invalid churn_days")
		return
	}
	segment := c.DefaultQuery("segment", "all")
	switch segment {
	case "all", "balance", "paying", "repeat", "new_paying", "active", "churned":
	default:
		response.BadRequest(c, "Invalid segment")
		return
	}
	result, err := h.dashboardService.GetOperationsCustomers(c.Request.Context(), service.OperationsCustomerFilter{
		Start: start, End: end, Segment: segment, Search: strings.TrimSpace(c.Query("search")), Page: page, PageSize: 20, ChurnDays: churnDays,
	})
	if err != nil {
		slog.Error("operations_customers_failed", "error", err)
		response.Error(c, 500, "Failed to get operations customers")
		return
	}
	response.Success(c, result)
}

// Reporting dates belong to the business timezone, not the caller's clock or timezone.
func parseOperationsReportingRange(c *gin.Context, now time.Time) (time.Time, time.Time, error) {
	loc := timezone.Location()
	if loc.String() == "Local" {
		loc, _ = time.LoadLocation("Asia/Shanghai")
	}
	now = now.In(loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start, end := today, today.AddDate(0, 0, 1)
	preset := c.Query("preset")
	switch preset {
	case "today":
	case "yesterday":
		start, end = today.AddDate(0, 0, -1), today
	case "7d":
		start = today.AddDate(0, 0, -6)
	case "30d":
		start = today.AddDate(0, 0, -29)
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	case "":
		if c.Query("start_date") != "" || c.Query("end_date") != "" {
			var err error
			start, err = time.ParseInLocation("2006-01-02", c.Query("start_date"), loc)
			if err != nil {
				return time.Time{}, time.Time{}, fmt.Errorf("invalid start_date")
			}
			end, err = time.ParseInLocation("2006-01-02", c.Query("end_date"), loc)
			if err != nil {
				return time.Time{}, time.Time{}, fmt.Errorf("invalid end_date")
			}
			end = end.AddDate(0, 0, 1)
		}
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("invalid preset")
	}
	if !start.Before(end) || start.Before(end.AddDate(0, 0, -90)) {
		return time.Time{}, time.Time{}, fmt.Errorf("Date range must be between 1 and 90 days")
	}
	return start, end, nil
}
