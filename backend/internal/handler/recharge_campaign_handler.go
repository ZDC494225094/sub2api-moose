package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

func (h *PaymentHandler) ListRechargeCampaigns(c *gin.Context) {
	items, err := h.paymentService.ListRechargeCampaigns(c.Request.Context(), true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
