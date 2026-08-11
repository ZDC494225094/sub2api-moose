package handler

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type PlaygroundHandler struct {
	runService     *service.PlaygroundRunService
	apiKeyService  *service.APIKeyService
	gatewayService *service.GatewayService
}

func NewPlaygroundHandler(runService *service.PlaygroundRunService, apiKeyService *service.APIKeyService, gatewayService *service.GatewayService) *PlaygroundHandler {
	return &PlaygroundHandler{runService: runService, apiKeyService: apiKeyService, gatewayService: gatewayService}
}

type canvasRunRequest struct {
	GroupID int64 `json:"groupId"`
	service.PlaygroundRunRequest
}

type canvasGroupConfig struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	Platform       string   `json:"platform"`
	RateMultiplier float64  `json:"rateMultiplier"`
	Models         []string `json:"models"`
}

// GetCanvasConfig exposes only the current user's allowed groups and the
// models schedulable within each group. It intentionally contains no gateway
// credential because canvas requests are signed by the logged-in user session.
func (h *PlaygroundHandler) GetCanvasConfig(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.apiKeyService == nil {
		response.InternalError(c, "Canvas service is not available")
		return
	}

	groups, err := h.apiKeyService.GetAvailableGroups(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result := make([]canvasGroupConfig, 0, len(groups))
	for i := range groups {
		group := groups[i]
		platform := group.Platform
		if platform == service.PlatformComposite {
			platform = ""
		}
		models := make([]string, 0)
		if h.gatewayService != nil {
			models = h.gatewayService.GetAvailableModels(c.Request.Context(), &group.ID, platform)
		}
		if models == nil {
			models = make([]string, 0)
		}
		result = append(result, canvasGroupConfig{
			ID:             group.ID,
			Name:           group.Name,
			Platform:       group.Platform,
			RateMultiplier: h.canvasGroupRateMultiplier(c.Request.Context(), subject.UserID, group, timezone.Now()),
			Models:         models,
		})
	}
	response.Success(c, result)
}

// canvasGroupRateMultiplier follows the same user override and peak-rate
// order as gateway usage billing. Image/video-specific pricing remains on the
// normal gateway path and is deliberately not duplicated in canvas metadata.
func (h *PlaygroundHandler) canvasGroupRateMultiplier(ctx context.Context, userID int64, group service.Group, now time.Time) float64 {
	rate := group.RateMultiplier
	if h.gatewayService != nil {
		rate = h.gatewayService.ResolveUserGroupRateMultiplier(ctx, userID, group.ID, rate)
	}
	if peakRate, active := group.PeakRateAt(now); active {
		return peakRate
	}
	return rate
}

// StartCanvasRun binds a canvas job to a user-selected group without exposing
// or requiring a user-managed API key. The internally managed key enters the
// normal gateway path, so account selection and billing semantics remain the
// same as external API traffic.
func (h *PlaygroundHandler) StartCanvasRun(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.runService == nil || h.apiKeyService == nil {
		response.InternalError(c, "Canvas service is not available")
		return
	}

	var req canvasRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if !h.canvasModelAvailable(c.Request.Context(), subject.UserID, req.GroupID, req.Model) {
		response.BadRequest(c, "Model is not available in the selected group")
		return
	}
	managedKey, err := h.apiKeyService.GetOrCreateCanvasManagedKey(c.Request.Context(), subject.UserID, req.GroupID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if req.Mode == "audio" && managedKey.Platform != service.PlatformGrok {
		response.BadRequest(c, "Audio generation is currently available only for Grok groups")
		return
	}
	req.APIKey = managedKey.Key
	req.Platform = managedKey.Platform
	req.EndpointBase = ""
	req.DisplayEndpoint = ""

	run, err := h.runService.Start(subject.UserID, req.PlaygroundRunRequest, playgroundRequestBaseURL(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Accepted(c, run)
}

func (h *PlaygroundHandler) canvasModelAvailable(ctx context.Context, userID, groupID int64, model string) bool {
	if h.gatewayService == nil || groupID <= 0 || strings.TrimSpace(model) == "" {
		return false
	}
	groups, err := h.apiKeyService.GetAvailableGroups(ctx, userID)
	if err != nil {
		return false
	}
	for i := range groups {
		group := groups[i]
		if group.ID != groupID {
			continue
		}
		platform := group.Platform
		if platform == service.PlatformComposite {
			platform = ""
		}
		for _, available := range h.gatewayService.GetAvailableModels(ctx, &group.ID, platform) {
			if available == model {
				return true
			}
		}
		return false
	}
	return false
}

func (h *PlaygroundHandler) StartRun(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.runService == nil {
		response.InternalError(c, "Playground run service is not available")
		return
	}

	var req service.PlaygroundRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	run, err := h.runService.Start(subject.UserID, req, playgroundRequestBaseURL(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Accepted(c, run)
}

func (h *PlaygroundHandler) GetRun(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.runService == nil {
		response.InternalError(c, "Playground run service is not available")
		return
	}

	run, ok := h.runService.Get(subject.UserID, c.Param("id"))
	if !ok {
		response.NotFound(c, "Playground run not found")
		return
	}
	response.Success(c, run)
}

func (h *PlaygroundHandler) GetRunImage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.runService == nil {
		response.InternalError(c, "Playground run service is not available")
		return
	}

	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		response.BadRequest(c, "Invalid image index")
		return
	}
	asset, found, err := h.runService.GetImage(subject.UserID, c.Param("id"), index)
	if !found {
		response.NotFound(c, "Playground image not found")
		return
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Length", strconv.Itoa(len(asset.Data)))
	c.Data(http.StatusOK, asset.ContentType, asset.Data)
}

func (h *PlaygroundHandler) GetRunVideo(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.runService == nil {
		response.InternalError(c, "Playground run service is not available")
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		response.BadRequest(c, "Invalid video index")
		return
	}
	asset, found, err := h.runService.GetVideoContext(c.Request.Context(), subject.UserID, c.Param("id"), index)
	if !found {
		response.NotFound(c, "Playground video not found")
		return
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Length", strconv.Itoa(len(asset.Data)))
	c.Data(http.StatusOK, asset.ContentType, asset.Data)
}

func (h *PlaygroundHandler) GetRunAudio(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.runService == nil {
		response.InternalError(c, "Playground run service is not available")
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		response.BadRequest(c, "Invalid audio index")
		return
	}
	asset, found, err := h.runService.GetAudio(subject.UserID, c.Param("id"), index)
	if !found {
		response.NotFound(c, "Playground audio not found")
		return
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Length", strconv.Itoa(len(asset.Data)))
	c.Data(http.StatusOK, asset.ContentType, asset.Data)
}

func (h *PlaygroundHandler) CancelRun(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	if h.runService == nil {
		response.InternalError(c, "Playground run service is not available")
		return
	}

	run, ok := h.runService.Cancel(subject.UserID, c.Param("id"))
	if !ok {
		response.NotFound(c, "Playground run not found")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    run,
	})
}

func playgroundRequestBaseURL(c *gin.Context) string {
	// Playground jobs call this server's gateway asynchronously. Use the actual
	// listener address so long image requests do not hairpin through a CDN.
	if c.Request != nil {
		if localAddr, ok := c.Request.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
			address := strings.TrimSpace(localAddr.String())
			if address != "" {
				scheme := "http"
				if c.Request.TLS != nil {
					scheme = "https"
				}
				return scheme + "://" + address
			}
		}
	}

	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = strings.TrimSpace(c.GetHeader("X-Forwarded-Scheme"))
	}
	if scheme == "" {
		if c.Request != nil && c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	if comma := strings.Index(scheme, ","); comma >= 0 {
		scheme = strings.TrimSpace(scheme[:comma])
	}

	host := strings.TrimSpace(c.GetHeader("X-Forwarded-Host"))
	if host == "" && c.Request != nil {
		host = c.Request.Host
	}
	if comma := strings.Index(host, ","); comma >= 0 {
		host = strings.TrimSpace(host[:comma])
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return scheme + "://" + host
}
