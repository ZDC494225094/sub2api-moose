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
	g.GET("/bots", h.listBots)
	g.POST("/bots", h.createBot)
	g.GET("/bots/:id", h.getBot)
	g.PUT("/bots/:id", h.saveBot)
	g.DELETE("/bots/:id", h.deleteBot)
	g.GET("/bots/:id/status", h.botStatus)
	g.POST("/bots/:id/test-send", h.testSend)
	g.POST("/bots/:id/push-now", h.pushNow)
	g.GET("/bots/:id/messages", h.messages)
	g.GET("/bots/:id/rules", h.listRules)
	g.POST("/bots/:id/rules", h.saveRule)
	g.PUT("/bots/:id/rules/:ruleId", h.saveRule)
	g.DELETE("/bots/:id/rules/:ruleId", h.deleteRule)
	g.GET("/bots/:id/ai/config", h.aiConfig)
	g.PUT("/bots/:id/ai/config", h.saveAIConfig)
	g.GET("/bots/:id/ai/keys", h.aiKeys)
	g.GET("/bots/:id/ai/records", h.aiRecords)
}

func (h *Handler) aiConfig(c *gin.Context) {
	id, ok := parseBotID(c)
	if !ok {
		return
	}
	if _, ok = h.loadBot(c, false); !ok {
		return
	}
	value, err := h.Runtime.Store.AIConfig(c.Request.Context(), id)
	if !response.ErrorFrom(c, err) {
		response.Success(c, value)
	}
}

func (h *Handler) saveAIConfig(c *gin.Context) {
	id, ok := parseBotID(c)
	if !ok {
		return
	}
	if _, ok = h.loadBot(c, false); !ok {
		return
	}
	var value AIConfig
	if err := c.ShouldBindJSON(&value); err != nil {
		response.BadRequest(c, "AI 配置格式错误")
		return
	}
	value.BotID = id
	if err := h.Runtime.Store.SaveAIConfig(c.Request.Context(), value); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.Runtime.signal()
	response.Success(c, value)
}

func (h *Handler) aiKeys(c *gin.Context) {
	if _, ok := h.loadBot(c, false); !ok {
		return
	}
	items, err := h.Runtime.Store.AIKeys(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) aiRecords(c *gin.Context) {
	id, ok := parseBotID(c)
	if !ok {
		return
	}
	if _, ok = h.loadBot(c, false); !ok {
		return
	}
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	if page < 1 {
		page = 1
	}
	items, total, err := h.Runtime.Store.AIRecords(c.Request.Context(), id, c.Query("keyword"), c.Query("blocked"), page, size)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
	}
}

func parseBotID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		response.BadRequest(c, "无效机器人 ID")
		return 0, false
	}
	return id, true
}

// loadBot fetches a bot for the handler or writes the error response itself.
func (h *Handler) loadBot(c *gin.Context, decrypt bool) (BotEntry, bool) {
	id, ok := parseBotID(c)
	if !ok {
		return BotEntry{}, false
	}
	entry, err := h.Runtime.Store.BotByID(c.Request.Context(), id, decrypt)
	if err == sql.ErrNoRows {
		response.NotFound(c, "机器人不存在")
		return BotEntry{}, false
	}
	if response.ErrorFrom(c, err) {
		return BotEntry{}, false
	}
	return entry, true
}

func (h *Handler) listBots(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	items, err := h.Runtime.Store.Bots(c.Request.Context(), false)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) getBot(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	if entry, ok := h.loadBot(c, false); ok {
		response.Success(c, entry)
	}
}

func (h *Handler) createBot(c *gin.Context) {
	var entry BotEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		response.BadRequest(c, "配置格式错误")
		return
	}
	entry.ID = 0
	platform, ok := h.Runtime.PlatformFor(entry.Platform)
	if !ok {
		response.BadRequest(c, "不支持的机器人平台")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	id, err := h.Runtime.Store.CreateBot(ctx, entry, platform)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.Runtime.signal()
	created, err := h.Runtime.Store.BotByID(c.Request.Context(), id, false)
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, created)
}

func (h *Handler) saveBot(c *gin.Context) {
	id, ok := parseBotID(c)
	if !ok {
		return
	}
	var entry BotEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		response.BadRequest(c, "配置格式错误")
		return
	}
	entry.ID = id
	stored, ok := h.loadBot(c, false)
	if !ok {
		return
	}
	entry.Platform = stored.Platform // a bot's platform never changes after creation
	platform, ok := h.Runtime.PlatformFor(stored.Platform)
	if !ok {
		response.BadRequest(c, "不支持的机器人平台")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	err := h.Runtime.Store.SaveBot(ctx, entry, platform)
	if err == sql.ErrNoRows {
		response.NotFound(c, "机器人不存在")
		return
	}
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	h.Runtime.signal()
	if saved, ok := h.loadBot(c, false); ok {
		response.Success(c, saved)
	}
}

func (h *Handler) deleteBot(c *gin.Context) {
	id, ok := parseBotID(c)
	if !ok {
		return
	}
	deleted, err := h.Runtime.Store.DeleteBot(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	if !deleted {
		response.NotFound(c, "机器人不存在")
		return
	}
	h.Runtime.signal()
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) botStatus(c *gin.Context) {
	entry, ok := h.loadBot(c, false)
	if !ok {
		return
	}
	status := h.Runtime.Status(entry.ID)
	status.Bot = Bot{ID: entry.BotID, Username: entry.BotUsername}
	response.Success(c, status)
}

func (h *Handler) testSend(c *gin.Context) {
	var req struct {
		ChatID string `json:"chat_id"`
		Text   string `json:"text"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Text) > 32000 {
		response.BadRequest(c, "请求格式错误或消息过长")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	entry, ok := h.loadBot(c, true)
	if !ok {
		return
	}
	if !ValidTargetID(entry.Platform, req.ChatID) {
		response.BadRequest(c, "无效的目标 ID 格式")
		return
	}
	if entry.BotToken == "" {
		response.BadRequest(c, "请先配置机器人凭证")
		return
	}
	platform, ok := h.Runtime.PlatformFor(entry.Platform)
	if !ok {
		response.BadRequest(c, "不支持的机器人平台")
		return
	}
	if req.Text == "" {
		req.Text = "sub2api 消息机器人测试成功"
	}
	if err := platform.SendMessage(ctx, entry.BotToken, req.ChatID, req.Text); err != nil {
		h.Runtime.report(entry.ID, ctx, err)
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"sent": true})
}

func (h *Handler) pushNow(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	entry, ok := h.loadBot(c, true)
	if !ok {
		return
	}
	if err := h.Runtime.PushNow(ctx, entry); err != nil {
		h.Runtime.report(entry.ID, ctx, err)
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"sent": true})
}

func (h *Handler) messages(c *gin.Context) {
	id, ok := parseBotID(c)
	if !ok {
		return
	}
	page, size := response.ParsePagination(c)
	if size > 100 {
		size = 100
	}
	if page > 1000000 {
		response.BadRequest(c, "页码过大")
		return
	}
	items, total, err := h.Runtime.Store.Messages(c.Request.Context(), id, c.Query("chat_id"), c.Query("keyword"), page, size)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
	}
}

func (h *Handler) listRules(c *gin.Context) {
	id, ok := parseBotID(c)
	if !ok {
		return
	}
	items, err := h.Runtime.Store.Rules(c.Request.Context(), id)
	if !response.ErrorFrom(c, err) {
		response.Success(c, gin.H{"items": items})
	}
}

func (h *Handler) saveRule(c *gin.Context) {
	botID, ok := parseBotID(c)
	if !ok {
		return
	}
	var rule Rule
	if c.ShouldBindJSON(&rule) != nil {
		response.BadRequest(c, "规则格式错误")
		return
	}
	rule.BotID = botID
	rule.ID = 0
	if c.Request.Method == "PUT" {
		ruleID, err := strconv.ParseInt(c.Param("ruleId"), 10, 64)
		if err != nil || ruleID < 1 {
			response.BadRequest(c, "无效规则 ID")
			return
		}
		rule.ID = ruleID
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
	botID, ok := parseBotID(c)
	if !ok {
		return
	}
	ruleID, err := strconv.ParseInt(c.Param("ruleId"), 10, 64)
	if err != nil || ruleID < 1 {
		response.BadRequest(c, "无效规则 ID")
		return
	}
	deleted, err := h.Runtime.Store.DeleteRule(c.Request.Context(), botID, ruleID)
	if response.ErrorFrom(c, err) {
		return
	}
	if !deleted {
		response.NotFound(c, "规则不存在")
		return
	}
	response.Success(c, gin.H{"deleted": true})
}
