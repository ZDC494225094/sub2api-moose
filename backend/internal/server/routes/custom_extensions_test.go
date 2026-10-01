package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/rechargecampaigns"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type extensionStoreStub struct{ reads int }

func (s *extensionStoreStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	s.reads++
	values := map[string]string{}
	for _, key := range keys {
		values[key] = "false"
	}
	return values, nil
}
func (s *extensionStoreStub) Set(context.Context, string, string) error { return nil }

// Test real route registrars, not only the path matcher. A missing Require would
// invoke an unconfigured handler instead of returning CUSTOM_EXTENSION_DISABLED.
func TestCustomRouteRegistrarsEnforceGatesAfterHostMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := &extensionStoreStub{}
	m := customize.NewManager(s)
	r := gin.New()
	r.Use(m.PageMiddleware())
	h := &handler.Handlers{Extensions: &handler.ExtensionHandlers{Playground: &handler.PlaygroundHandler{}, Operations: &adminhandler.OperationsHandler{}, RechargeCampaigns: &rechargecampaigns.Handler{}}}
	authenticated := r.Group("/api/v1", func(c *gin.Context) {
		if c.GetHeader("X-Test-Auth") != "yes" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	})
	registerCustomUserRoutes(authenticated, h, m)
	admin := authenticated.Group("/admin")
	registerCustomDashboardRoutes(admin.Group("/dashboard"), h, m)
	registerCustomPaymentUserRoutes(authenticated.Group("/payment"), h.Extensions, m)
	registerCustomPaymentAdminRoutes(admin.Group("/payment"), h.Extensions, m)
	publicLimited := false
	registerCustomPaymentPublicRoutes(r.Group("/api/v1/payment/public"), h.Extensions, m, func(c *gin.Context) {
		if publicLimited {
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}
		c.Next()
	})
	admin.GET("/dashboard/stats", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/v1/playground/runs"},
		{"POST", "/api/v1/canvas/runs"}, {"GET", "/api/v1/canvas/config"},
		{"GET", "/api/v1/admin/dashboard/operations-funnel"},
		{"GET", "/api/v1/admin/dashboard/operations-finance"},
		{"GET", "/api/v1/admin/dashboard/operations-customers"},
		{"GET", "/api/v1/admin/dashboard/operations-users"},
		{"GET", "/api/v1/admin/dashboard/operations-marketing-recipients"},
		{"GET", "/api/v1/admin/dashboard/operations-marketing-email-records"},
		{"POST", "/api/v1/admin/dashboard/operations-marketing-email"},
		{"GET", "/api/v1/payment/campaigns"},
		{"GET", "/api/v1/admin/payment/campaigns"},
		{"POST", "/api/v1/admin/payment/campaigns"}, {"PUT", "/api/v1/admin/payment/campaigns/1"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			reads := s.reads
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, http.StatusUnauthorized, w.Code)
			require.Equal(t, reads, s.reads, "authentication must run before extension storage")
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-Test-Auth", "yes")
			w = httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusForbidden, w.Code)
			require.Contains(t, w.Body.String(), "CUSTOM_EXTENSION_DISABLED")
		})
	}
	for _, code := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		publicLimited = code == http.StatusTooManyRequests
		reads := s.reads
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/payment/public/campaigns", nil))
		require.Equal(t, code, w.Code)
		if publicLimited {
			require.Equal(t, reads, s.reads, "public rate limiting must run first")
		}
	}
	reads := s.reads
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/admin/dashboard/stats", nil)
	req.Header.Set("X-Test-Auth", "yes")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, reads, s.reads, "core dashboard must not inherit the extension gate")
}

func TestCoreRoutesDoNotRequireExtensionContainer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, h := range []*handler.Handlers{nil, {}, {Extensions: &handler.ExtensionHandlers{}}} {
		r := gin.New()
		group := r.Group("/api/v1")
		require.NotPanics(t, func() {
			registerCustomUserRoutes(group, h, nil)
			registerCustomDashboardRoutes(group.Group("/admin/dashboard"), h, nil)
			var extensions *handler.ExtensionHandlers
			if h != nil {
				extensions = h.Extensions
			}
			registerCustomPaymentUserRoutes(group.Group("/payment"), extensions, nil)
			registerCustomPaymentPublicRoutes(group.Group("/payment/public"), extensions, nil, nil)
			registerCustomPaymentAdminRoutes(group.Group("/admin/payment"), extensions, nil)
		})
		group.GET("/admin/dashboard/stats", func(c *gin.Context) { c.Status(http.StatusOK) })
		require.Len(t, r.Routes(), 1, "an absent extension container must not register broken handlers")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/admin/dashboard/stats", nil))
		require.Equal(t, http.StatusOK, w.Code)
	}
}

// A marketing-only container must not depend on the campaign module existing.
// All 14 legacy endpoints retain the host guard chain and URL/method contract.
func TestMarketingRoutesInheritHostGuardsWithoutCampaignDependency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	authenticated := func(c *gin.Context) {
		if c.GetHeader("X-Test-Auth") != "yes" {
			c.AbortWithStatus(401)
			return
		}
		c.Next()
	}
	admin := func(c *gin.Context) {
		if c.GetHeader("X-Test-Admin") != "yes" {
			c.AbortWithStatus(403)
			return
		}
		c.Next()
	}
	limited := func(c *gin.Context) {
		if c.GetHeader("X-Test-Limited") == "yes" {
			c.AbortWithStatus(429)
			return
		}
		c.Next()
	}
	audited := func(c *gin.Context) { c.Header("X-Test-Audited", "yes"); c.Next() }
	h := &handler.ExtensionHandlers{Marketing: marketing.NewHandler[service.RedeemCode](nil, nil, func(*gin.Context) (int64, bool) { return 42, true })}
	registerCustomPaymentUserRoutes(r.Group("/api/v1/payment", authenticated, limited), h, nil)
	registerCustomPaymentAdminRoutes(r.Group("/api/v1/admin/payment", authenticated, admin, audited), h, nil)
	registerCustomPaymentPublicRoutes(r.Group("/api/v1/payment/public"), h, nil, limited)
	cases := []struct {
		method, path string
		status       int
	}{
		{"GET", "/coupons", 200}, {"GET", "/lottery/active", 200}, {"POST", "/lottery/draw", 500}, {"GET", "/lottery/my-records", 200},
		{"GET", "/admin/coupon-templates", 200}, {"POST", "/admin/coupon-templates", 500}, {"PUT", "/admin/coupon-templates/3", 500},
		{"GET", "/admin/lottery/activities", 200}, {"POST", "/admin/lottery/activities", 500}, {"PUT", "/admin/lottery/activities/3", 500}, {"DELETE", "/admin/lottery/activities/3", 500},
		{"GET", "/admin/lottery/activities/3/draw-records", 200}, {"POST", "/admin/lottery/prizes", 500}, {"PUT", "/admin/lottery/prizes/3", 500},
	}
	require.Len(t, r.Routes(), len(cases))
	for _, tc := range cases {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			isAdmin := strings.HasPrefix(tc.path, "/admin/")
			path := "/api/v1/payment" + tc.path
			if isAdmin {
				path = "/api/v1/admin/payment" + strings.TrimPrefix(tc.path, "/admin")
			}
			for _, state := range []string{"anonymous", "user", "limited", "admin"} {
				req := httptest.NewRequest(tc.method, path, nil)
				if state != "anonymous" {
					req.Header.Set("X-Test-Auth", "yes")
				}
				if state == "admin" {
					req.Header.Set("X-Test-Admin", "yes")
				}
				if state == "limited" {
					req.Header.Set("X-Test-Limited", "yes")
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				want := tc.status
				switch {
				case state == "anonymous":
					want = 401
				case isAdmin && state != "admin":
					want = 403
				case !isAdmin && state == "limited":
					want = 429
				}
				require.Equal(t, want, w.Code, tc.path+" "+state)
				if isAdmin && state == "admin" {
					require.Equal(t, "yes", w.Header().Get("X-Test-Audited"))
				}
			}
		})
	}
	for _, path := range []string{"/api/v1/payment/public/coupons", "/api/v1/payment/public/lottery/active"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		require.Equal(t, 404, w.Code)
	}
}
