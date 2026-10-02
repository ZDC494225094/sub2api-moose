package customize

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type memoryStore struct {
	mu     sync.Mutex
	values map[string]string
	err    error
	reads  int
}

func (s *memoryStore) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	out := map[string]string{}
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, s.err
}
func (s *memoryStore) Set(_ context.Context, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return s.err
	}
	s.values[key] = value
	return nil
}
func newStore() *memoryStore { return &memoryStore{values: map[string]string{}} }

func TestCatalogAndDefaultDisabled(t *testing.T) {
	ctx := context.Background()
	m := NewManager(newStore())
	states, err := m.List(ctx)
	require.NoError(t, err)
	ids := map[string]bool{}
	count := 0
	for _, state := range states {
		require.False(t, ids[state.ID], "duplicate id")
		ids[state.ID] = true
		require.NotEmpty(t, state.DisableBehavior)
		if state.Managed {
			count++
			require.NotNil(t, state.Enabled)
			require.False(t, *state.Enabled)
			require.True(t, len(state.Paths) > 0 || len(state.Slots) > 0 || len(state.Gates) > 0, "managed extensions must declare routes, UI slots or admission gates")
		} else {
			require.Nil(t, state.Enabled)
		}
	}
	require.Equal(t, 14, count)
}

func TestPersistentIndependentSwitchesAcrossManagers(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	a, b := NewManager(store), NewManager(store)
	require.NoError(t, a.SetEnabled(ctx, "playground", false))
	require.NoError(t, b.SetEnabled(ctx, "premium-home", false))
	for _, m := range []*Manager{a, b, NewManager(store)} {
		states, err := m.Snapshot(ctx)
		require.NoError(t, err)
		require.False(t, states["playground"])
		require.False(t, states["premium-home"])
		require.False(t, states[RechargeCampaigns])
	}
	require.NoError(t, a.SetEnabled(ctx, "playground", true))
	enabled, err := b.Enabled(ctx, "playground")
	require.NoError(t, err)
	require.True(t, enabled)
	require.Len(t, store.values, 2)
}

func TestInvalidUnknownAndFailedUpdates(t *testing.T) {
	ctx := context.Background()
	store := newStore()
	m := NewManager(store)
	require.Error(t, m.SetEnabled(ctx, "../anything", false))
	require.Error(t, m.SetEnabled(ctx, "unknown-module", false))
	require.Empty(t, store.values)
	store.err = errors.New("private database error")
	require.Error(t, m.SetEnabled(ctx, "playground", false))
	require.Empty(t, store.values)
	enabled, err := m.Enabled(ctx, "playground")
	require.Error(t, err)
	require.False(t, enabled)
	require.NotContains(t, err.Error(), "private")
	store.err = nil
	store.values[Key("playground")] = "yes"
	_, err = m.Snapshot(ctx)
	require.Error(t, err)
	_, err = NewManager(nil).Snapshot(ctx)
	require.Error(t, err)
}

func TestHTTPGateMatrixAndDrain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		method, path, id string
		code             int
	}{
		{"GET", "/", "premium-home", 307}, {"GET", "/docs", "premium-home", 307}, {"HEAD", "/docs/guide", "premium-home", 307},
		{"POST", "/api/v1/playground/runs", "playground", 403}, {"GET", "/api/v1/playground/runs/123", "playground", 200}, {"DELETE", "/api/v1/playground/runs/123", "playground", 200},
		{"GET", "/api/v1/playground/runs/123/images/0", "playground", 200},
		{"GET", "/canvas/", "infinite-canvas", 403}, {"GET", "/canvas/assets/app.js", "infinite-canvas", 403},
		{"POST", "/api/v1/canvas/runs", "infinite-canvas", 403}, {"GET", "/api/v1/canvas/config", "infinite-canvas", 403},
		{"GET", "/api/v1/canvas/runs/123/videos/0", "infinite-canvas", 200}, {"DELETE", "/api/v1/canvas/runs/123", "infinite-canvas", 200},
		{"GET", "/api/v1/admin/dashboard/operations-finance", "operations-analytics", 403}, {"POST", "/api/v1/admin/dashboard/operations-marketing-email", "operations-analytics", 403},
		{"GET", "/api/v1/payment/public/campaigns", RechargeCampaigns, 403}, {"POST", "/api/v1/admin/payment/campaigns", RechargeCampaigns, 403}, {"PUT", "/api/v1/admin/payment/campaigns/1", RechargeCampaigns, 403},
		{"POST", "/api/v1/payment/orders", RechargeCampaigns, 200}, {"POST", "/api/v1/payment/webhook/notify", RechargeCampaigns, 200},
		{"GET", "/api/v1/admin/dashboard/stats", "operations-analytics", 200}, {"GET", "/home", "premium-home", 200}, {"GET", "/documentation", "premium-home", 200},
		{"GET", "/api/v1/admin/plugins", "playground", 200},
	}
	for _, tc := range tests {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			store := newStore()
			m := NewManager(store)
			require.NoError(t, m.SetEnabled(context.Background(), tc.id, false))
			r := gin.New()
			r.Use(m.Middleware())
			r.Handle(tc.method, tc.path, func(c *gin.Context) { c.Status(200) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, tc.code, w.Code)
			if tc.code == 307 {
				require.Equal(t, "/home", w.Header().Get("Location"))
			}
		})
	}
}

func TestStorageFailureClosesExtensionsNotUpstream(t *testing.T) {
	store := newStore()
	store.err = errors.New("offline")
	r := gin.New()
	r.Use(NewManager(store).Middleware())
	for _, path := range []string{"/api/v1/canvas/config", "/api/v1/admin/dashboard/stats"} {
		r.GET(path, func(c *gin.Context) { c.Status(200) })
	}
	for path, code := range map[string]int{"/api/v1/canvas/config": 503, "/api/v1/admin/dashboard/stats": 200} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		require.Equal(t, code, w.Code)
	}
	require.Equal(t, 1, store.reads)
}

func TestStateHTTPContracts(t *testing.T) {
	m := NewManager(newStore())
	r := gin.New()
	r.GET("/public", m.PublicState)
	r.GET("/admin", m.AdminList)
	r.PUT("/admin/:id", m.AdminUpdate)
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"false"}`, `no`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/admin/playground", strings.NewReader(body)))
		require.Equal(t, 400, w.Code)
	}
	for id, code := range map[string]int{"playground": 200, "marketing-tools": 200, "multi-group-billing": 200, "billing-scheduling": 200, "site-customization": 200, "access-policy": 200, "subscription-extensions": 200, "media-gateway": 200, "unknown": 404} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/admin/"+id, strings.NewReader(`{"enabled":false}`)))
		require.Equal(t, code, w.Code)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/public", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	var response struct {
		Data struct {
			Enabled map[string]bool `json:"enabled"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	require.False(t, response.Data.Enabled["playground"])
	managedCount := 0
	for _, manifest := range Catalog() {
		if manifest.Managed {
			managedCount++
		}
	}
	require.Len(t, response.Data.Enabled, managedCount)
	require.NotContains(t, w.Body.String(), "待解耦")
}

func TestEnabledIsolatesUnrelatedCorruptSetting(t *testing.T) {
	store := newStore()
	store.values[Key("playground")] = "invalid"
	store.values[Key(RechargeCampaigns)] = "true"
	m := NewManager(store)
	enabled, err := m.Enabled(context.Background(), RechargeCampaigns)
	require.NoError(t, err)
	require.True(t, enabled)
	_, err = m.Enabled(context.Background(), "playground")
	require.Error(t, err)
}

func TestPageGateNeverReadsAPIStateAheadOfAuthentication(t *testing.T) {
	store := newStore()
	store.err = errors.New("offline")
	m := NewManager(store)
	r := gin.New()
	r.Use(m.PageMiddleware())
	api := r.Group("/api/v1")
	api.Use(func(c *gin.Context) { c.AbortWithStatus(http.StatusUnauthorized) })
	api.POST("/playground/runs", m.Require("playground"), func(c *gin.Context) { t.Fatal("handler must not run") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/playground/runs", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.Zero(t, store.reads)
}

func TestDisabledHomeKeepsReferralQuery(t *testing.T) {
	store := newStore()
	store.values[Key("premium-home")] = "false"
	r := gin.New()
	r.Use(NewManager(store).PageMiddleware())
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/?aff=ref123", nil))
	require.Equal(t, http.StatusTemporaryRedirect, w.Code)
	require.Equal(t, "/home?aff=ref123", w.Header().Get("Location"))
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

// The switch controls configuration admission, not request enforcement.
func TestAccessPolicySwitchDoesNotGateSecurityEndpoints(t *testing.T) {
	store := newStore()
	m := NewManager(store)
	for _, enabled := range []bool{true, false, true} {
		require.NoError(t, m.SetEnabled(context.Background(), "access-policy", enabled))
		got, err := m.Enabled(context.Background(), "access-policy")
		require.NoError(t, err)
		require.Equal(t, enabled, got)
	}
	for _, path := range []string{"/api/v1/auth/register", "/api/v1/auth/registration-proof", "/api/v1/settings/public", "/access-restricted"} {
		require.Empty(t, RequestExtension("POST", path))
		require.Empty(t, RequestExtension("GET", path))
	}
}

func TestAdminEfficiencySwitchPreservesHistoricalPaths(t *testing.T) {
	store := newStore()
	m := NewManager(store)
	for _, enabled := range []bool{true, false, true} {
		require.NoError(t, m.SetEnabled(context.Background(), "admin-efficiency", enabled))
		got, err := m.Enabled(context.Background(), "admin-efficiency")
		require.NoError(t, err)
		require.Equal(t, enabled, got)
	}
	for _, path := range []string{"/admin/accounts", "/admin/groups", "/admin/users", "/api/v1/admin/accounts/upstream-groups", "/api/v1/admin/groups/1/accounts"} {
		require.Empty(t, RequestExtension("GET", path))
	}
}
