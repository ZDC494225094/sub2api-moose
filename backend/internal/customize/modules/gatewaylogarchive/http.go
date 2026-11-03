package gatewaylogarchive

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct{ Runtime *Runtime }

func (h *Handler) Register(group *gin.RouterGroup, manager *customize.Manager) {
	g := group.Group("/gateway-log-archive", manager.Require(ID))
	g.GET("/config", h.getConfig)
	g.PUT("/config", h.saveConfig)
	g.POST("/config/test", h.testConfig)
	g.GET("/status", h.status)
	g.POST("/flush", h.flush)
	g.GET("/bundles", h.listBundles)
	g.GET("/bundles/:id", h.getBundle)
	g.POST("/bundles/:id/retry", h.retryBundle)
	g.GET("/bundles/:id/download", h.downloadBundle)
	g.GET("/entries", h.listEntries)
	g.GET("/entries/:id/content", h.entryContent)
}

// GatewayCapture is mounted by the gateway routes; it is a pass-through
// unless the module is enabled and capture is configured.
func (h *Handler) GatewayCapture() gin.HandlerFunc {
	if h == nil || h.Runtime == nil {
		return func(c *gin.Context) { c.Next() }
	}
	return h.Runtime.CaptureMiddleware()
}

type configView struct {
	Config
	AccessKeyID        string     `json:"access_key_id"`
	SecretAccessKeySet bool       `json:"secret_access_key_set"`
	SessionTokenSet    bool       `json:"session_token_set"`
	UpdatedAt          *time.Time `json:"updated_at"`
}

func maskKey(v string) string {
	if v == "" {
		return ""
	}
	if len(v) <= 8 {
		return "****"
	}
	return v[:4] + "****" + v[len(v)-4:]
}

func newConfigView(cfg Config, updated *time.Time) configView {
	return configView{Config: cfg, AccessKeyID: maskKey(cfg.AccessKeyID), SecretAccessKeySet: cfg.SecretAccessKey != "",
		SessionTokenSet: cfg.SessionToken != "", UpdatedAt: updated}
}

// saveRequest: omitted or empty credential fields keep the stored value.
type saveRequest struct {
	Config
	AccessKeyID       *string `json:"access_key_id"`
	SecretAccessKey   *string `json:"secret_access_key"`
	SessionToken      *string `json:"session_token"`
	ClearCredentials  bool    `json:"clear_credentials"`
	ClearSessionToken bool    `json:"clear_session_token"`
}

func (req saveRequest) merge(stored Config) (Config, error) {
	cfg := req.Config
	cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken = stored.AccessKeyID, stored.SecretAccessKey, stored.SessionToken
	if req.ClearCredentials {
		cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken = "", "", ""
	}
	if req.ClearSessionToken {
		cfg.SessionToken = ""
	}
	set := func(dst *string, v *string) {
		if v != nil && strings.TrimSpace(*v) != "" && !strings.Contains(*v, "****") {
			*dst = strings.TrimSpace(*v)
		}
	}
	set(&cfg.AccessKeyID, req.AccessKeyID)
	set(&cfg.SecretAccessKey, req.SecretAccessKey)
	set(&cfg.SessionToken, req.SessionToken)
	return cfg, cfg.Normalize()
}

func (h *Handler) getConfig(c *gin.Context) {
	cfg, updated, err := h.Runtime.Store.Config(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, newConfigView(cfg, updated))
	}
}

func (h *Handler) bindConfig(c *gin.Context) (Config, bool) {
	var req saveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "配置格式错误")
		return Config{}, false
	}
	stored, _, err := h.Runtime.Store.Config(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return Config{}, false
	}
	cfg, err := req.merge(stored)
	if err != nil {
		response.BadRequest(c, err.Error())
		return Config{}, false
	}
	return cfg, true
}

func (h *Handler) saveConfig(c *gin.Context) {
	cfg, ok := h.bindConfig(c)
	if !ok {
		return
	}
	if cfg.CaptureEnabled && !cfg.StorageReady() {
		response.BadRequest(c, "开启采集前请先填写存储桶")
		return
	}
	if err := h.Runtime.Store.SaveConfig(c.Request.Context(), cfg); response.ErrorFrom(c, err) {
		return
	}
	h.Runtime.signal()
	saved, updated, err := h.Runtime.Store.Config(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, newConfigView(saved, updated))
	}
}

func (h *Handler) testConfig(c *gin.Context) {
	cfg, ok := h.bindConfig(c)
	if !ok {
		return
	}
	if !cfg.StorageReady() {
		response.BadRequest(c, "请先填写存储桶")
		return
	}
	objects, err := h.Runtime.objects(cfg)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	steps := objects.Test(ctx)
	okAll := true
	for _, s := range steps {
		if !s.OK && !strings.HasPrefix(s.Name, "删除") && !strings.HasPrefix(s.Name, "访问") {
			okAll = false
		}
	}
	response.Success(c, gin.H{"ok": okAll, "steps": steps})
}

func (h *Handler) status(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	response.Success(c, h.Runtime.Status(c.Request.Context()))
}

func (h *Handler) flush(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	if err := h.Runtime.FlushNow(ctx); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, h.Runtime.Status(c.Request.Context()))
}

func pageParams(c *gin.Context) (int, int) {
	page, size := response.ParsePagination(c)
	if page < 1 {
		page = 1
	}
	if page > 100000 {
		page = 100000
	}
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func (h *Handler) listBundles(c *gin.Context) {
	page, size := pageParams(c)
	items, total, err := h.Runtime.Store.ListBundles(c.Request.Context(), BundleFilter{
		Day: strings.TrimSpace(c.Query("day")), Model: strings.TrimSpace(c.Query("model")), Status: strings.TrimSpace(c.Query("status")),
	}, page, size)
	if !response.ErrorFrom(c, err) {
		if items == nil {
			items = []Bundle{}
		}
		response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
	}
}

func parseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		response.BadRequest(c, "无效 ID")
		return 0, false
	}
	return id, true
}

func (h *Handler) loadBundle(c *gin.Context) (Bundle, bool) {
	id, ok := parseID(c)
	if !ok {
		return Bundle{}, false
	}
	b, err := h.Runtime.Store.Bundle(c.Request.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "压缩包不存在")
		return Bundle{}, false
	}
	return b, !response.ErrorFrom(c, err)
}

func (h *Handler) getBundle(c *gin.Context) {
	if b, ok := h.loadBundle(c); ok {
		response.Success(c, b)
	}
}

func (h *Handler) retryBundle(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	retried, err := h.Runtime.Store.RetryBundle(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	if !retried {
		response.BadRequest(c, "只有上传失败且本地文件仍在的压缩包可以重试")
		return
	}
	if pipe := h.Runtime.current(); pipe != nil {
		pipe.wakeUploader()
	}
	response.Success(c, gin.H{"retried": true})
}

func (h *Handler) downloadBundle(c *gin.Context) {
	b, ok := h.loadBundle(c)
	if !ok {
		return
	}
	if b.Status != StatusUploaded || b.ObjectKey == "" {
		response.BadRequest(c, "压缩包上传完成后才能下载")
		return
	}
	objects, err := h.Runtime.objectStore(c.Request.Context())
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	url, err := objects.Presign(c.Request.Context(), b.ObjectKey, b.FileName, time.Hour)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"url": url, "expires_in": 3600})
}

func (h *Handler) listEntries(c *gin.Context) {
	page, size := pageParams(c)
	bundleID, _ := strconv.ParseInt(c.Query("bundle_id"), 10, 64)
	userID, _ := strconv.ParseInt(c.Query("user_id"), 10, 64)
	items, total, err := h.Runtime.Store.ListEntries(c.Request.Context(), EntryFilter{
		BundleID: bundleID, UserID: userID, SessionID: strings.TrimSpace(c.Query("session_id")),
		Day: strings.TrimSpace(c.Query("day")), Model: strings.TrimSpace(c.Query("model")),
	}, page, size)
	if !response.ErrorFrom(c, err) {
		if items == nil {
			items = []Entry{}
		}
		response.Success(c, gin.H{"items": items, "total": total, "page": page, "page_size": size})
	}
}

const (
	defaultPreview = 1 << 20
	maxPreview     = 16 << 20
)

func (h *Handler) entryContent(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	e, b, err := h.Runtime.Store.Entry(c.Request.Context(), id)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "数据不存在")
		return
	}
	if response.ErrorFrom(c, err) {
		return
	}
	rc, err := h.Runtime.OpenEntry(c.Request.Context(), e, b)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	defer rc.Close()
	c.Header("Cache-Control", "no-store")
	if c.Query("download") == "1" {
		c.Header("Content-Disposition", contentDisposition(strings.TrimSuffix(b.FileName, ".zip")+"-"+e.Name))
		c.Header("Content-Length", strconv.FormatInt(e.USize, 10))
		c.Status(http.StatusOK)
		c.Header("Content-Type", "application/json; charset=utf-8")
		_, _ = io.Copy(c.Writer, rc)
		return
	}
	limitBytes := int64(defaultPreview)
	if v, err := strconv.ParseInt(c.Query("limit"), 10, 64); err == nil && v > 0 {
		limitBytes = v
	}
	if limitBytes > maxPreview {
		limitBytes = maxPreview
	}
	data, err := io.ReadAll(io.LimitReader(rc, limitBytes))
	if err != nil {
		response.BadRequest(c, "读取数据失败："+err.Error())
		return
	}
	response.Success(c, gin.H{
		"entry": e, "bundle": b, "content": strings.ToValidUTF8(string(data), ""),
		"truncated": int64(len(data)) < e.USize, "size": e.USize,
	})
}
