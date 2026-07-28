package handler

import (
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type PlaygroundHandler struct {
	runService *service.PlaygroundRunService
}

func NewPlaygroundHandler(runService *service.PlaygroundRunService) *PlaygroundHandler {
	return &PlaygroundHandler{runService: runService}
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
