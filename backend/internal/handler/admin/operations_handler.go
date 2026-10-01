package admin

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OperationsHandler owns extension endpoints even where historical URLs remain
// under /admin/dashboard. It never changes the upstream DashboardHandler.
type OperationsHandler struct {
	operationsService     *service.OperationsService
	marketingEmailService *service.OperationsMarketingEmailService
}

func NewOperationsHandler(operations *service.OperationsService, marketing *service.OperationsMarketingEmailService) *OperationsHandler {
	return &OperationsHandler{operationsService: operations, marketingEmailService: marketing}
}

const (
	operationsFunnelDefaultDays = 30
	operationsFunnelMaxDays     = 90
)

func parseOperationsFunnelRange(c *gin.Context) (time.Time, time.Time, error) {
	userTZ := c.Query("timezone")
	now := timezone.NowInUserLocation(userTZ)
	startDate := strings.TrimSpace(c.Query("start_date"))
	endDate := strings.TrimSpace(c.Query("end_date"))

	startTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, -(operationsFunnelDefaultDays-1)), userTZ)
	endTime := timezone.StartOfDayInUserLocation(now.AddDate(0, 0, 1), userTZ)

	if startDate != "" {
		parsed, err := timezone.ParseInUserLocation("2006-01-02", startDate, userTZ)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid start_date, use YYYY-MM-DD")
		}
		startTime = parsed
	}
	if endDate != "" {
		parsed, err := timezone.ParseInUserLocation("2006-01-02", endDate, userTZ)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid end_date, use YYYY-MM-DD")
		}
		endTime = parsed.AddDate(0, 0, 1)
	}
	if !startTime.Before(endTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("start_date must be before or equal to end_date")
	}

	maxStart := endTime.AddDate(0, 0, -operationsFunnelMaxDays)
	if startTime.Before(maxStart) {
		startTime = maxStart
	}
	return startTime, endTime, nil
}

var dashboardOperationsFunnelCache = newSnapshotCache(30 * time.Second)

type dashboardOperationsFunnelCacheKey struct {
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// GetOperationsFunnel handles read-only operations funnel analytics.
// GET /api/v1/admin/dashboard/operations-funnel
func (h *OperationsHandler) GetOperationsFunnel(c *gin.Context) {
	startTime, endTime, err := parseOperationsFunnelRange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	cacheKey := mustMarshalDashboardCacheKey(dashboardOperationsFunnelCacheKey{
		StartTime: startTime.UTC().Format(time.RFC3339),
		EndTime:   endTime.UTC().Format(time.RFC3339),
	})
	entry, hit, err := dashboardOperationsFunnelCache.GetOrLoad(cacheKey, func() (any, error) {
		return h.operationsService.GetOperationsFunnel(c.Request.Context(), startTime, endTime)
	})
	if err != nil {
		slog.Error("operations_funnel_failed", "error", err, "start", startTime, "end", endTime)
		response.Error(c, 500, "Failed to get operations funnel")
		return
	}
	payload, err := snapshotPayloadAs[*service.OperationsFunnelResponse](entry.Payload)
	if err != nil {
		slog.Error("operations_funnel_payload_failed", "error", err)
		response.Error(c, 500, "Failed to get operations funnel")
		return
	}
	c.Header("X-Snapshot-Cache", cacheStatusValue(hit))
	response.Success(c, payload)
}

// GetOperationsUserDetails handles paginated drill-down rows for operations metrics.
// GET /api/v1/admin/dashboard/operations-users
func (h *OperationsHandler) GetOperationsUserDetails(c *gin.Context) {
	startTime, endTime, err := parseOperationsFunnelRange(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.operationsService.ListOperationsUserDetails(c.Request.Context(), service.OperationsUserDetailFilter{
		Segment:   strings.ToLower(strings.TrimSpace(c.DefaultQuery("segment", "all"))),
		StartTime: startTime,
		EndTime:   endTime,
		Pagination: pagination.PaginationParams{
			Page:     page,
			PageSize: pageSize,
		},
	})
	if err != nil {
		response.Error(c, 500, "Failed to get operations user details")
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// SendOperationsMarketingEmail previews or sends a limited marketing email batch.
// POST /api/v1/admin/dashboard/operations-marketing-email
func (h *OperationsHandler) SendOperationsMarketingEmail(c *gin.Context) {
	if h.marketingEmailService == nil {
		response.InternalError(c, "Operations marketing email service not available")
		return
	}
	var req service.OperationsMarketingEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.marketingEmailService.Send(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ListOperationsMarketingRecipients lists selectable marketing email recipients.
// GET /api/v1/admin/dashboard/operations-marketing-recipients
func (h *OperationsHandler) ListOperationsMarketingRecipients(c *gin.Context) {
	if h.marketingEmailService == nil {
		response.InternalError(c, "Operations marketing email service not available")
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	query := service.OperationsMarketingRecipientQuery{
		Keyword:           strings.TrimSpace(c.Query("keyword")),
		Audience:          strings.ToLower(strings.TrimSpace(c.DefaultQuery("audience", "all"))),
		Status:            strings.ToLower(strings.TrimSpace(c.DefaultQuery("status", service.StatusActive))),
		ActiveDays:        parsePositiveIntQuery(c, "active_days", marketingEmailDefaultActiveDaysForHandler),
		MinBalance:        parseOptionalFloatQuery(c, "min_balance"),
		MaxBalance:        parseOptionalFloatQuery(c, "max_balance"),
		MinTotalRecharged: parseOptionalFloatQuery(c, "min_total_recharged"),
		MaxTotalRecharged: parseOptionalFloatQuery(c, "max_total_recharged"),
		Page:              page,
		PageSize:          pageSize,
	}
	items, total, err := h.marketingEmailService.ListRecipients(c.Request.Context(), query)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

// ListOperationsMarketingEmailRecords lists previous marketing email send records.
// GET /api/v1/admin/dashboard/operations-marketing-email-records
func (h *OperationsHandler) ListOperationsMarketingEmailRecords(c *gin.Context) {
	if h.marketingEmailService == nil {
		response.InternalError(c, "Operations marketing email service not available")
		return
	}
	page, pageSize := response.ParsePagination(c)
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := h.marketingEmailService.ListRecords(c.Request.Context(), page, pageSize)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, page, pageSize)
}

const marketingEmailDefaultActiveDaysForHandler = 30

func parsePositiveIntQuery(c *gin.Context, key string, fallback int) int {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseOptionalFloatQuery(c *gin.Context, key string) *float64 {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}
