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
	g.GET("/buckets", h.listBuckets)
	g.POST("/buckets", h.createBucket)
	g.POST("/buckets/test", h.testBucket)
	g.PUT("/buckets/:id", h.updateBucket)
	g.DELETE("/buckets/:id", h.deleteBucket)
	g.GET("/stats/models", h.modelStats)
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
	UpdatedAt *time.Time `json:"updated_at"`
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

func (h *Handler) getConfig(c *gin.Context) {
	cfg, updated, err := h.Runtime.Store.Config(c.Request.Context())
	if !response.ErrorFrom(c, err) {
		response.Success(c, configView{Config: cfg, UpdatedAt: updated})
	}
}

func (h *Handler) saveConfig(c *gin.Context) {
	var req Config
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "配置格式错误")
		return
	}
	ctx := c.Request.Context()
	stored, _, err := h.Runtime.Store.Config(ctx)
	if response.ErrorFrom(c, err) {
		return
	}
	req.keepStorageState(stored)
	if err := req.Normalize(); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.CaptureEnabled {
		buckets, err := h.Runtime.Store.Buckets(ctx)
		if response.ErrorFrom(c, err) {
			return
		}
		if len(enabledBuckets(buckets)) == 0 {
			response.BadRequest(c, "开启采集前请先添加并启用至少一个存储桶")
			return
		}
	}
	if err := h.Runtime.Store.SaveConfig(ctx, req); response.ErrorFrom(c, err) {
		return
	}
	h.Runtime.signal()
	saved, updated, err := h.Runtime.Store.Config(ctx)
	if !response.ErrorFrom(c, err) {
		response.Success(c, configView{Config: saved, UpdatedAt: updated})
	}
}

// bucketView never carries secrets: the access key is masked and the secret
// and session token are reported only as set / not set.
type bucketView struct {
	BucketConfig
	AccessKeyID        string      `json:"access_key_id"`
	SecretAccessKeySet bool        `json:"secret_access_key_set"`
	SessionTokenSet    bool        `json:"session_token_set"`
	Usage              BucketUsage `json:"usage"`
}

func newBucketView(b BucketConfig, usage BucketUsage) bucketView {
	return bucketView{BucketConfig: b, AccessKeyID: maskKey(b.AccessKeyID), SecretAccessKeySet: b.SecretAccessKey != "",
		SessionTokenSet: b.SessionToken != "", Usage: usage}
}

// bucketRequest: omitted, empty or masked credential fields keep the stored value.
type bucketRequest struct {
	ID                int64   `json:"id"`
	Name              string  `json:"name"`
	Provider          string  `json:"provider"`
	Bucket            string  `json:"bucket"`
	Region            string  `json:"region"`
	Endpoint          string  `json:"endpoint"`
	Prefix            string  `json:"prefix"`
	ForcePathStyle    bool    `json:"force_path_style"`
	Enabled           *bool   `json:"enabled"`
	Priority          *int    `json:"priority"`
	AccessKeyID       *string `json:"access_key_id"`
	SecretAccessKey   *string `json:"secret_access_key"`
	SessionToken      *string `json:"session_token"`
	ClearCredentials  bool    `json:"clear_credentials"`
	ClearSessionToken bool    `json:"clear_session_token"`
}

func (req bucketRequest) merge(stored BucketConfig) (BucketConfig, error) {
	b := stored
	b.Name, b.Provider, b.Bucket, b.Region = req.Name, req.Provider, req.Bucket, req.Region
	b.Endpoint, b.Prefix, b.ForcePathStyle = req.Endpoint, req.Prefix, req.ForcePathStyle
	if req.Enabled != nil {
		b.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		b.Priority = *req.Priority
	}
	if req.ClearCredentials {
		b.AccessKeyID, b.SecretAccessKey, b.SessionToken = "", "", ""
	}
	if req.ClearSessionToken {
		b.SessionToken = ""
	}
	set := func(dst *string, v *string) {
		if v != nil && strings.TrimSpace(*v) != "" && !strings.Contains(*v, "****") {
			*dst = strings.TrimSpace(*v)
		}
	}
	set(&b.AccessKeyID, req.AccessKeyID)
	set(&b.SecretAccessKey, req.SecretAccessKey)
	set(&b.SessionToken, req.SessionToken)
	return b, b.Normalize()
}

// bindBucket merges the request with the stored bucket (id from the path or,
// for connection tests of an edited bucket, from the body).
func (h *Handler) bindBucket(c *gin.Context, id int64) (BucketConfig, bool) {
	var req bucketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "存储桶配置格式错误")
		return BucketConfig{}, false
	}
	if id == 0 {
		id = req.ID
	}
	stored := BucketConfig{Enabled: true, Priority: 100}
	if id > 0 {
		var err error
		stored, err = h.Runtime.Store.Bucket(c.Request.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			response.NotFound(c, "存储桶不存在")
			return BucketConfig{}, false
		}
		if response.ErrorFrom(c, err) {
			return BucketConfig{}, false
		}
	}
	b, err := req.merge(stored)
	if err != nil {
		response.BadRequest(c, err.Error())
		return BucketConfig{}, false
	}
	return b, true
}

func (h *Handler) listBuckets(c *gin.Context) {
	ctx := c.Request.Context()
	buckets, err := h.Runtime.Store.Buckets(ctx)
	if response.ErrorFrom(c, err) {
		return
	}
	usage, err := h.Runtime.Store.BucketUsage(ctx)
	if response.ErrorFrom(c, err) {
		return
	}
	items := make([]bucketView, 0, len(buckets))
	for _, b := range buckets {
		items = append(items, newBucketView(b, usage[b.ID]))
	}
	// Totals of deleted buckets (or uploads not yet attributed) stay visible.
	var orphan BucketUsage
	for id, u := range usage {
		if !bucketExists(buckets, id) {
			orphan.BundleCount += u.BundleCount
			orphan.SizeBytes += u.SizeBytes
		}
	}
	response.Success(c, gin.H{"items": items, "unattributed": orphan})
}

func bucketExists(buckets []BucketConfig, id int64) bool {
	for _, b := range buckets {
		if b.ID == id {
			return true
		}
	}
	return false
}

func (h *Handler) createBucket(c *gin.Context) {
	b, ok := h.bindBucket(c, 0)
	if !ok {
		return
	}
	b.ID = 0
	created, err := h.Runtime.Store.CreateBucket(c.Request.Context(), b)
	if errors.Is(err, errTooManyBuckets) {
		response.BadRequest(c, err.Error())
		return
	}
	if response.ErrorFrom(c, err) {
		return
	}
	h.Runtime.signal()
	response.Success(c, newBucketView(created, BucketUsage{}))
}

func (h *Handler) updateBucket(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	b, ok := h.bindBucket(c, id)
	if !ok {
		return
	}
	updated, err := h.Runtime.Store.UpdateBucket(c.Request.Context(), b)
	if errors.Is(err, sql.ErrNoRows) {
		response.NotFound(c, "存储桶不存在")
		return
	}
	if response.ErrorFrom(c, err) {
		return
	}
	h.Runtime.signal()
	usage, _ := h.Runtime.Store.BucketUsage(c.Request.Context())
	response.Success(c, newBucketView(updated, usage[updated.ID]))
}

func (h *Handler) deleteBucket(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	deleted, err := h.Runtime.Store.DeleteBucket(c.Request.Context(), id)
	if response.ErrorFrom(c, err) {
		return
	}
	if !deleted {
		response.NotFound(c, "存储桶不存在")
		return
	}
	h.Runtime.signal()
	response.Success(c, gin.H{"deleted": true})
}

func (h *Handler) testBucket(c *gin.Context) {
	b, ok := h.bindBucket(c, 0)
	if !ok {
		return
	}
	objects, err := h.Runtime.objects(b)
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

// modelStats drills the cumulative uploaded total down by model (archive name).
func (h *Handler) modelStats(c *gin.Context) {
	bucketID, _ := strconv.ParseInt(c.Query("bucket_id"), 10, 64)
	items, err := h.Runtime.Store.ModelStats(c.Request.Context(), bucketID)
	if response.ErrorFrom(c, err) {
		return
	}
	var bundles, entries, bytes int64
	for _, m := range items {
		bundles += m.BundleCount
		entries += m.EntryCount
		bytes += m.SizeBytes
	}
	response.Success(c, gin.H{"items": items, "total_bundles": bundles, "total_entries": entries, "total_bytes": bytes})
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
	objects, err := h.Runtime.objectStoreFor(c.Request.Context(), b)
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
