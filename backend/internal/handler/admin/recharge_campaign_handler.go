package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *PaymentHandler) ListRechargeCampaigns(c *gin.Context) {
	items, err := h.paymentService.ListRechargeCampaigns(c.Request.Context(), false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *PaymentHandler) SaveRechargeCampaign(c *gin.Context) {
	var a service.RechargeCampaign
	if err := c.ShouldBindJSON(&a); err != nil {
		response.BadRequest(c, "Invalid campaign")
		return
	}
	a.ID = 0
	if c.Param("id") != "" {
		id, ok := parseIDParam(c, "id")
		if !ok {
			return
		}
		a.ID = id
	}
	item, err := h.paymentService.SaveRechargeCampaign(c.Request.Context(), a)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
