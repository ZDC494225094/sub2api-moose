package marketing

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

// HTTP ports intentionally exclude native payment services and middleware.
type CouponHTTPService interface {
	CreateTemplate(ctx context.Context, input *CreateCouponTemplateInput) (*CouponTemplate, error)
	UpdateTemplate(ctx context.Context, id int64, input *UpdateCouponTemplateInput) (*CouponTemplate, error)
	ListTemplates(ctx context.Context, params pagination.PaginationParams, filter CouponTemplateListFilter) ([]CouponTemplate, *pagination.PaginationResult, error)
	ListUserCoupons(ctx context.Context, userID int64, params pagination.PaginationParams, filter UserCouponListFilter) ([]UserCoupon, *pagination.PaginationResult, error)
}
type LotteryHTTPService[T any] interface {
	DeleteActivity(ctx context.Context, id int64) error
	CreateActivity(ctx context.Context, input *CreateLotteryActivityInput) (*LotteryActivity, error)
	UpdateActivity(ctx context.Context, id int64, input *UpdateLotteryActivityInput) (*LotteryActivity, error)
	ListActivities(ctx context.Context, params pagination.PaginationParams, filter LotteryActivityListFilter) ([]LotteryActivity, *pagination.PaginationResult, error)
	GetActiveOverview(ctx context.Context, userID int64) (*LotteryOverview, error)
	CreatePrize(ctx context.Context, input *CreateLotteryPrizeInput) (*LotteryPrize, error)
	UpdatePrize(ctx context.Context, id int64, input *UpdateLotteryPrizeInput) (*LotteryPrize, error)
	Draw(ctx context.Context, input LotteryDrawInput) (*LotteryDrawResult[T], error)
	ListUserDrawRecords(ctx context.Context, userID, activityID int64, params pagination.PaginationParams) ([]LotteryDrawRecord, *pagination.PaginationResult, error)
}

// SubjectResolver returns only the authenticated user ID, never a body/query ID.
// The host supplies its native authentication adapter; routes inherit host guards.
type SubjectResolver func(*gin.Context) (int64, bool)
type Handler[T any] struct {
	couponService  CouponHTTPService
	lotteryService LotteryHTTPService[T]
	subject        SubjectResolver
}

func NewHandler[T any](coupons CouponHTTPService, lottery LotteryHTTPService[T], subject SubjectResolver) *Handler[T] {
	return &Handler[T]{couponService: coupons, lotteryService: lottery, subject: subject}
}
func (h *Handler[T]) requireUser(c *gin.Context) (int64, bool) {
	if h.subject != nil {
		if id, ok := h.subject(c); ok {
			return id, true
		}
	}
	response.Unauthorized(c, "User not authenticated")
	return 0, false
}

func (h *Handler[T]) GetMyCoupons(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}
	if h.couponService == nil {
		response.Success(c, []UserCoupon{})
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.couponService.ListUserCoupons(c.Request.Context(), userID, paginationParams(page, pageSize), UserCouponListFilter{
		Status: strings.TrimSpace(c.Query("status")),
		Scope:  strings.TrimSpace(c.Query("scope")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total.Total, page, pageSize)
}

func (h *Handler[T]) GetActiveLottery(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}
	if h.lotteryService == nil {
		response.Success(c, gin.H{})
		return
	}
	overview, err := h.lotteryService.GetActiveOverview(c.Request.Context(), userID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

func (h *Handler[T]) ListMyDrawRecords(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}
	if h.lotteryService == nil {
		response.Success(c, gin.H{"items": []any{}, "total": 0})
		return
	}
	activityIDStr := c.Query("activity_id")
	var activityID int64
	if activityIDStr != "" {
		activityID, _ = strconv.ParseInt(activityIDStr, 10, 64)
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	records, pg, err := h.lotteryService.ListUserDrawRecords(c.Request.Context(), userID, activityID, pagination.PaginationParams{Page: page, PageSize: pageSize})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": records, "total": pg.Total, "page": pg.Page, "page_size": pg.PageSize})
}

func (h *Handler[T]) DrawLottery(c *gin.Context) {
	userID, ok := h.requireUser(c)
	if !ok {
		return
	}
	if h.lotteryService == nil {
		response.InternalError(c, "lottery service not configured")
		return
	}
	var req struct {
		ActivityID int64 `json:"activity_id" binding:"required,gt=0"`
		UseWallet  bool  `json:"use_wallet"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.lotteryService.Draw(c.Request.Context(), LotteryDrawInput{
		ActivityID: req.ActivityID,
		UserID:     userID,
		UseWallet:  req.UseWallet,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *Handler[T]) ListCouponTemplates(c *gin.Context) {
	if h.couponService == nil {
		response.Success(c, gin.H{"items": []CouponTemplate{}, "total": 0})
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.couponService.ListTemplates(c.Request.Context(), paginationParams(page, pageSize), CouponTemplateListFilter{
		Status: strings.TrimSpace(c.Query("status")),
		Scope:  strings.TrimSpace(c.Query("scope")),
		Search: strings.TrimSpace(c.Query("search")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total.Total, page, pageSize)
}

func (h *Handler[T]) CreateCouponTemplate(c *gin.Context) {
	if h.couponService == nil {
		response.InternalError(c, "coupon service not configured")
		return
	}
	var req CreateCouponTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.couponService.CreateTemplate(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *Handler[T]) UpdateCouponTemplate(c *gin.Context) {
	if h.couponService == nil {
		response.InternalError(c, "coupon service not configured")
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req UpdateCouponTemplateInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.couponService.UpdateTemplate(c.Request.Context(), id, &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler[T]) ListLotteryActivities(c *gin.Context) {
	if h.lotteryService == nil {
		response.Success(c, gin.H{"items": []LotteryActivity{}, "total": 0})
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, total, err := h.lotteryService.ListActivities(c.Request.Context(), paginationParams(page, pageSize), LotteryActivityListFilter{
		Status: strings.TrimSpace(c.Query("status")),
		Search: strings.TrimSpace(c.Query("search")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total.Total, page, pageSize)
}

func (h *Handler[T]) CreateLotteryActivity(c *gin.Context) {
	if h.lotteryService == nil {
		response.InternalError(c, "lottery service not configured")
		return
	}
	var req CreateLotteryActivityInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.lotteryService.CreateActivity(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *Handler[T]) DeleteLotteryActivity(c *gin.Context) {
	if h.lotteryService == nil {
		response.InternalError(c, "lottery service not configured")
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.lotteryService.DeleteActivity(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{})
}

func (h *Handler[T]) UpdateLotteryActivity(c *gin.Context) {
	if h.lotteryService == nil {
		response.InternalError(c, "lottery service not configured")
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req UpdateLotteryActivityInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.lotteryService.UpdateActivity(c.Request.Context(), id, &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *Handler[T]) ListActivityDrawRecords(c *gin.Context) {
	if h.lotteryService == nil {
		response.Success(c, gin.H{"items": []any{}, "total": 0})
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	records, pg, err := h.lotteryService.ListUserDrawRecords(c.Request.Context(), 0, id, paginationParams(page, pageSize))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, records, pg.Total, page, pageSize)
}

func (h *Handler[T]) CreateLotteryPrize(c *gin.Context) {
	if h.lotteryService == nil {
		response.InternalError(c, "lottery service not configured")
		return
	}
	var req CreateLotteryPrizeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.lotteryService.CreatePrize(c.Request.Context(), &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, item)
}

func (h *Handler[T]) UpdateLotteryPrize(c *gin.Context) {
	if h.lotteryService == nil {
		response.InternalError(c, "lottery service not configured")
		return
	}
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req UpdateLotteryPrizeInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.lotteryService.UpdatePrize(c.Request.Context(), id, &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func paginationParams(page, pageSize int) pagination.PaginationParams {
	return pagination.PaginationParams{Page: page, PageSize: pageSize}
}
func parseIDParam(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid "+name)
		return 0, false
	}
	return id, true
}
