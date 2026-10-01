//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type routingOwnershipGroupRepo struct {
	service.GroupRepository
	candidate *service.Group
	calls     int
}

func (r *routingOwnershipGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	r.calls++
	if r.candidate != nil && r.candidate.ID == id {
		return r.candidate, nil
	}
	return nil, service.ErrGroupNotFound
}

func TestAPIKeyRoutingOwnershipAuthEntrypoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, entry := range []string{"openai", "google"} {
		t.Run(entry, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				policy     string
				disabled   bool
				missing    bool
				wantStatus int
				wantGroup  int64
				wantCalls  int
				wantError  string
			}{
				{name: "legacy_falls_back", policy: multigroupbilling.LegacyRouting, disabled: true, wantStatus: http.StatusOK, wantGroup: 102, wantCalls: 1},
				{name: "empty_legacy_falls_back", disabled: true, wantStatus: http.StatusOK, wantGroup: 102, wantCalls: 1},
				{name: "native_preserves_loaded_group", policy: multigroupbilling.NativeRouting, wantStatus: http.StatusOK, wantGroup: 101},
				{name: "native_disabled_rejects_without_fallback", policy: multigroupbilling.NativeRouting, disabled: true, wantStatus: http.StatusForbidden},
				{name: "native_missing_rejects_without_fallback", policy: multigroupbilling.NativeRouting, missing: true, wantStatus: http.StatusForbidden},
				{name: "unknown_policy_rejects", policy: "future-v99", wantStatus: http.StatusForbidden, wantError: "unknown API key routing policy"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					primary := &service.Group{ID: 101, Status: service.StatusActive, Platform: service.PlatformOpenAI, Hydrated: true}
					if tc.disabled {
						primary.Status = service.StatusDisabled
					}
					groups := &routingOwnershipGroupRepo{candidate: &service.Group{ID: 102, Status: service.StatusActive, Platform: service.PlatformOpenAI, Hydrated: true}}
					key := &service.APIKey{ID: 1, Key: "routing-test-key", UserID: 7, Status: service.StatusActive,
						User:    &service.User{ID: 7, Status: service.StatusActive, Role: service.RoleUser, Balance: 10},
						GroupID: &primary.ID, Group: primary, GroupIDs: []int64{101, 102},
						Platform: service.PlatformOpenAI, RoutingPolicy: tc.policy}
					if tc.missing {
						key.Group = nil
					}
					repo := &stubApiKeyRepo{getByKey: func(_ context.Context, credential string) (*service.APIKey, error) {
						if credential != key.Key {
							return nil, service.ErrAPIKeyNotFound
						}
						clone := *key
						return &clone, nil
					}}
					cfg := &config.Config{RunMode: config.RunModeSimple}
					svc := service.NewAPIKeyService(repo, nil, groups, nil, nil, nil, cfg)
					router := gin.New()
					if entry == "google" {
						router.Use(APIKeyAuthWithSubscriptionGoogle(svc, nil, cfg))
					} else {
						router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(svc, nil, cfg)))
					}
					handled := false
					router.GET("/t", func(c *gin.Context) {
						handled = true
						authenticated, ok := c.Get(string(ContextKeyAPIKey))
						require.True(t, ok)
						selected, ok := authenticated.(*service.APIKey)
						require.True(t, ok)
						require.NotNil(t, selected.GroupID)
						require.Equal(t, tc.wantGroup, *selected.GroupID)
						require.NotNil(t, selected.Group)
						require.Equal(t, tc.wantGroup, selected.Group.ID)
						contextGroup, ok := c.Request.Context().Value(ctxkey.Group).(*service.Group)
						require.True(t, ok)
						require.Equal(t, tc.wantGroup, contextGroup.ID)
						c.JSON(http.StatusOK, gin.H{"ok": true})
					})
					w := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodGet, "/t", nil)
					req.Header.Set("x-api-key", key.Key)
					router.ServeHTTP(w, req)
					require.Equal(t, tc.wantStatus, w.Code, w.Body.String())
					require.Equal(t, tc.wantStatus == http.StatusOK, handled)
					require.Equal(t, tc.wantCalls, groups.calls)
					if tc.wantError != "" {
						require.Contains(t, w.Body.String(), tc.wantError)
					}
					if tc.policy == multigroupbilling.NativeRouting && tc.wantStatus == http.StatusForbidden && entry == "openai" {
						code := "GROUP_DISABLED"
						if tc.missing {
							code = "GROUP_DELETED"
						}
						require.Contains(t, w.Body.String(), code)
					}
				})
			}
		})
	}
}
