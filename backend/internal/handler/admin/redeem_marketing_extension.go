package admin

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/redeemmarketing"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *RedeemHandler) GenerateMarketingBatch(c *gin.Context) {
	var req struct {
		GenerateRedeemCodesRequest
		redeemmarketing.Options
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Type != service.RedeemTypeMarketing {
		response.BadRequest(c, "marketing type is required")
		return
	}
	expiresAt, err := resolveRedeemCodeExpiresAt(req.ExpiresAt, req.ExpiresInDays)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	executeAdminIdempotentJSON(c, "custom.redeem_marketing.generate", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		codes, err := h.redeemService.GenerateMarketingBatch(ctx, req.Options, func(txCtx context.Context) ([]service.RedeemCode, error) {
			return h.adminService.GenerateRedeemCodes(txCtx, &service.GenerateRedeemCodesInput{
				Count: req.Count, Type: req.Type, Value: req.Value, ExpiresAt: expiresAt,
			})
		})
		if err != nil {
			return nil, err
		}
		out := make([]dto.AdminRedeemCode, 0, len(codes))
		for i := range codes {
			out = append(out, *dto.RedeemCodeFromServiceAdmin(&codes[i]))
		}
		return out, nil
	})
}

func (h *RedeemHandler) MarketingBatches(c *gin.Context) {
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	batch := strings.TrimSpace(c.Query("batch_id"))
	if len(batch) > 64 {
		response.BadRequest(c, "batch_id is too long")
		return
	}
	items, err := h.redeemService.MarketingBatches(c.Request.Context(), batch, page, size)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "page": page, "page_size": size})
}
