package botnotify

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{ Runtime *Runtime }

func (h *Handler) Register(group *gin.RouterGroup, manager *customize.Manager) {
	g := group.Group("/bot-notify", manager.Require(ID))
	g.GET("/config", h.config)
	g.PUT("/config", h.saveConfig)
	g.GET("/status", func(c *gin.Context) {
		status := h.Runtime.Status()
		cfg, err := h.Runtime.Store.Config(c.Request.Context(), false)
		if response.ErrorFrom(c, err) {
			return
		}
		status.Bot = Bot{ID: cfg.BotID, Username: cfg.BotUsername}
		response.Success(c, status)
	})
	g.POST("/test-send", h.testSend)
	g.GET("/messages", h.messages)
	g.GET("/rules", func(c *gin.Context) {
		items, err := h.Runtime.Store.Rules(c.Request.Context())
		if !response.ErrorFrom(c, err) {
			response.Success(c, gin.H{"items": items})
		}
	})
	g.POST("/rules", h.saveRule)
	g.PUT("/rules/:id", h.saveRule)
	g.DELETE("/rules/:id", h.deleteRule)
}

func (h *Handler) config(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	cfg, err := h.Runtime.Store.Config(c.Request.Context(), false)
	if !response.ErrorFrom(c, err) {
		response.Success(c, cfg)
	}
}

func (h *Handler) saveConfig(c *gin.Context) {
	var cfg Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "配置格式错误")
		return
	}
	if err := ValidateConfig(cfg); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	if err := h.Runtime.Store.SaveConfig(ctx, cfg, h.Runtime.Platform); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.Runtime.signal()
	h.config(c)
}

func (h *Handler) testSend(c *gin.Context) {
	var req struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	if c.ShouldBindJSON(&req) != nil || !ValidChatID(req.ChatID) || len(req.Text) > 32000 {
		response.BadRequest(c, "无效 chat_id 或消息过长")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	cfg, err := h.Runtime.Store.Config(ctx, true)
	if response.ErrorFrom(c, err) {
		return
	}
	if cfg.BotToken == "" {
		response.BadRequest(c, "请先配置 bot token")
		return
	}
	if req.Text == "" {
		req.Text = "sub2api 消息机器人测试成功"
	}
	if err = h.Runtime.Platform.SendMessage(ctx, cfg.BotToken, req.ChatID, req.Text); err != nil {
		h.Runtime.report(ctx, err)
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"sent": true})
}

func (h *Handler) messages(c *gin.Context) {
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	if page > 1000000 {
		response.BadRequest(c, "页码过大")
		return
	}
	items, total, err := h.Runtime.Store.Messages(c.Request.Context(), c.Query("chat_id"), c.Query("keyword"), page, size)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
	}
}

func (h *Handler) saveRule(c *gin.Context) {
	var rule Rule
	if c.ShouldBindJSON(&rule) != nil {
		response.BadRequest(c, "规则格式错误")
		return
	}
	rule.ID = 0
	if c.Request.Method == "PUT" {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id < 1 {
			response.BadRequest(c, "无效规则 ID")
			return
		}
		rule.ID = id
	}
	if err := ValidateRule(rule); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	id, err := h.Runtime.Store.SaveRule(c.Request.Context(), rule)
	if err == sql.ErrNoRows {
		response.NotFound(c, "规则不存在")
		return
	}
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"id": id})
	}
}

func (h *Handler) deleteRule(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		response.BadRequest(c, "无效规则 ID")
		return
	}
	result, err := h.Runtime.Store.DB.ExecContext(c.Request.Context(), "DELETE FROM bot_reply_rules WHERE id=$1", id)
	if response.ErrorFrom(c, err) {
		return
	}
	n, err := result.RowsAffected()
	if response.ErrorFrom(c, err) {
		return
	}
	if n == 0 {
		response.NotFound(c, "规则不存在")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
