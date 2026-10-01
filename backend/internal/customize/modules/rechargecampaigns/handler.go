package rechargecampaigns

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
	"strconv"
)

type catalog interface {
	List(context.Context, bool) ([]Campaign, error)
	Save(context.Context, Campaign) (*Campaign, error)
}

// Handler is mounted by the host inside its existing auth/rate-limit/audit
// groups, never on webhook or fulfillment routes.
type Handler struct{ catalog catalog }

func NewHandler(service *Service) *Handler   { return &Handler{catalog: service} }
func (h *Handler) ListPublic(c *gin.Context) { h.list(c, true) }
func (h *Handler) ListAdmin(c *gin.Context)  { h.list(c, false) }
func (h *Handler) list(c *gin.Context, public bool) {
	items, err := h.catalog.List(c.Request.Context(), public)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *Handler) Save(c *gin.Context) {
	var a Campaign
	if err := c.ShouldBindJSON(&a); err != nil {
		response.BadRequest(c, "Invalid campaign")
		return
	}
	a.ID = 0 // URL identity is authoritative; POST cannot replace an existing row.
	if c.Param("id") != "" {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid id")
			return
		}
		a.ID = id
	}
	item, err := h.catalog.Save(c.Request.Context(), a)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}
