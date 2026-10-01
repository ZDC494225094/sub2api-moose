package wiring

import (
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestMarketingHostAdapterRequiresNativeAuthSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, state := range []string{"missing", "wrong_type", "native"} {
		t.Run(state, func(t *testing.T) {
			r := gin.New()
			r.Use(func(c *gin.Context) {
				switch state {
				case "wrong_type":
					c.Set(string(middleware.ContextKeyUser), map[string]any{"UserID": 42})
				case "native":
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				}
				c.Next()
			})
			// Partial/isolated hosts must preserve the old nil-service response rather
			// than wrapping typed nil pointers in nonnil HTTP interfaces.
			h := ProvideMarketingHandler(nil, nil)
			h.RegisterUserRoutes(r.Group("/payment"))
			req := httptest.NewRequest("GET", "/payment/coupons?user_id=42", nil)
			req.Header.Set("X-User-ID", "42")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if state == "native" {
				require.Equal(t, 200, w.Code)
				require.JSONEq(t, `{"code":0,"message":"success","data":[]}`, w.Body.String())
			} else {
				require.Equal(t, 401, w.Code)
			}
		})
	}
}
