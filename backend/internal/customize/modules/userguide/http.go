package userguide

import (
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

const maxTitleRunes = 200
const maxContentBytes = 1 << 20 // 1 MiB of markdown is far beyond any manual.

type Handler struct {
	Store *Store
}

// Public serves the current manual for the public docs page. It is mounted
// behind the extension switch, next to the public announcements route.
func (h *Handler) Public(c *gin.Context) {
	cfg, err := h.Store.Config(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	c.Header("Cache-Control", "public, max-age=60")
	response.Success(c, cfg)
}

// RegisterAdmin mounts the admin config endpoints under the caller's admin
// group (host auth/audit chain applies), gated by the extension switch.
func (h *Handler) RegisterAdmin(group *gin.RouterGroup, manager *customize.Manager) {
	g := group.Group("/guide", manager.Require(ID))
	g.GET("/config", h.getConfig)
	g.PUT("/config", h.saveConfig)
	g.POST("/config/reset", h.resetConfig)
}

func (h *Handler) getConfig(c *gin.Context) {
	cfg, err := h.Store.Config(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{
		"title": cfg.Title, "content_md": cfg.ContentMD,
		"updated_at": cfg.UpdatedAt, "is_default": cfg.IsDefault,
		"default_title": DefaultTitle, "default_content_md": defaultContent,
	})
}

type saveRequest struct {
	Title     string `json:"title"`
	ContentMD string `json:"content_md"`
}

func (h *Handler) saveConfig(c *gin.Context) {
	var req saveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || utf8.RuneCountInString(req.Title) > maxTitleRunes {
		response.BadRequest(c, "title must be 1-200 characters")
		return
	}
	if strings.TrimSpace(req.ContentMD) == "" || len(req.ContentMD) > maxContentBytes {
		response.BadRequest(c, "content_md must be non-empty and at most 1MiB")
		return
	}
	cfg, err := h.Store.Save(c.Request.Context(), req.Title, req.ContentMD)
	if response.ErrorFrom(c, err) {
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, cfg)
}

func (h *Handler) resetConfig(c *gin.Context) {
	cfg, err := h.Store.Reset(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, cfg)
}
