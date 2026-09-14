package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"log/slog"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *DashboardHandler) GetOperationsFinance(c *gin.Context) {
	start, end, err := parseOperationsFunnelRange(c)
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
	start, end, err := parseOperationsFunnelRange(c)
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
