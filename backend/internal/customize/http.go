package customize

import (
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

// PageMiddleware runs before the embedded frontend, but never queries the
// database for APIs. API gates belong after authentication and rate limiting.
func (m *Manager) PageMiddleware() gin.HandlerFunc {
	gate := m.Middleware()
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		gate(c)
	}
}

// Require is attached to a module's new entry points after the host's existing
// authentication/rate-limit/audit chain. Existing task reads are left ungated.
func (m *Manager) Require(id string) gin.HandlerFunc {
	return func(c *gin.Context) {
		enabled, err := m.Enabled(c.Request.Context(), id)
		if err == nil && !enabled {
			err = Disabled(id)
		}
		if err != nil {
			c.Header("Cache-Control", "no-store")
			response.ErrorFrom(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}

// Middleware classifies extension paths. PageMiddleware is its global adapter;
// API registration uses Require so anonymous traffic cannot bypass host limits.
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// FullPath is Gin's matched route, including :id parameters. Using it prevents
		// path escaping / trailing slash variants from bypassing an API gate.
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		id := RequestExtension(c.Request.Method, strings.TrimSuffix(path, "/"))
		if path == "/" {
			id = RequestExtension(c.Request.Method, path)
		}
		if id == "" {
			c.Next()
			return
		}
		enabled, err := m.Enabled(c.Request.Context(), id)
		if err != nil {
			response.ErrorFrom(c, err)
			c.Abort()
			return
		}
		if !enabled {
			if id == "premium-home" {
				target := "/home"
				if query := c.Request.URL.Query().Encode(); query != "" {
					target += "?" + query
				}
				c.Header("Cache-Control", "no-store")
				c.Redirect(http.StatusTemporaryRedirect, target)
			} else {
				response.ErrorFrom(c, Disabled(id))
			}
			c.Abort()
			return
		}
		c.Next()
	}
}

func (m *Manager) PublicState(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	state, err := m.Snapshot(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"enabled": state})
}

func (m *Manager) AdminList(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	items, err := m.List(c.Request.Context())
	if response.ErrorFrom(c, err) {
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (m *Manager) AdminUpdate(c *gin.Context) {
	var req struct {
		Enabled *bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		response.BadRequest(c, "enabled must be a boolean")
		return
	}
	id := c.Param("id")
	if response.ErrorFrom(c, m.SetEnabled(c.Request.Context(), id, *req.Enabled)) {
		return
	}
	c.Header("Cache-Control", "no-store")
	response.Success(c, gin.H{"id": id, "enabled": *req.Enabled})
}
