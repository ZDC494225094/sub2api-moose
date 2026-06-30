package handler

import (
	"net/http"
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
