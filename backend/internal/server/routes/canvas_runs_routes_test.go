package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	servermiddleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCanvasRunListRequiresAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{}, Extensions: &handler.ExtensionHandlers{Playground: handler.NewPlaygroundHandler(nil, nil, nil)}}
	adminAuth := servermiddleware.AdminAuthMiddleware(func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.AbortWithStatus(http.StatusForbidden)
	})
	audit := servermiddleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() })
	stepUp := servermiddleware.StepUpAuthMiddleware(func(c *gin.Context) { c.Next() })
	RegisterAdminRoutes(router.Group("/api/v1"), handlers, adminAuth, audit, stepUp, nil, nil)
	for _, auth := range []string{"", "Bearer user-token"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/canvas-runs", nil)
		request.Header.Set("Authorization", auth)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		want := http.StatusForbidden
		if auth == "" {
			want = http.StatusUnauthorized
		}
		require.Equal(t, want, recorder.Code)
	}
}
