package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type adminUsageIPRiskCapture struct {
	adminUsageRepoCapture

	riskStart       time.Time
	riskEnd         time.Time
	riskMinAccounts int
	riskLimit       int
}

func (s *adminUsageIPRiskCapture) ListSharedIPs(_ context.Context, startTime, endTime time.Time, minAccounts, limit int) ([]usagestats.IPSharingStat, error) {
	s.riskStart = startTime
	s.riskEnd = endTime
	s.riskMinAccounts = minAccounts
	s.riskLimit = limit
	return []usagestats.IPSharingStat{{IPAddress: "203.0.113.7", UserCount: 2}}, nil
}

func (s *adminUsageIPRiskCapture) ListIPSharingUsers(_ context.Context, _ []string, _, _ time.Time) (map[string][]usagestats.IPSharingUser, error) {
	return map[string][]usagestats.IPSharingUser{
		"203.0.113.7": {{UserID: 1, Email: "a@example.com"}},
	}, nil
}

func newAdminUsageIPTestRouter(repo *adminUsageIPRiskCapture) *gin.Engine {
	gin.SetMode(gin.TestMode)
	usageSvc := service.NewUsageService(repo, nil, nil, nil)
	handler := NewUsageHandler(usageSvc, nil, nil, nil)
	router := gin.New()
	router.GET("/admin/usage", handler.List)
	router.GET("/admin/usage/stats", handler.Stats)
	router.GET("/admin/usage/ip-sharing", handler.IPSharingRisk)
	return router
}

func TestAdminUsageListIPFilter(t *testing.T) {
	repo := &adminUsageIPRiskCapture{}
	router := newAdminUsageIPTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?ip_address=203.0.113.7", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "203.0.113.7", repo.listFilters.IPAddress)
}

func TestAdminUsageStatsIPFilter(t *testing.T) {
	repo := &adminUsageIPRiskCapture{}
	router := newAdminUsageIPTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/stats?ip_address=2001:db8::1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "2001:db8::1", repo.statsFilters.IPAddress)
}

func TestAdminUsageIPFilterRejectsInvalidValue(t *testing.T) {
	repo := &adminUsageIPRiskCapture{}
	router := newAdminUsageIPTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage?ip_address=not-an-ip", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminUsageIPSharingDefaultsAndClamps(t *testing.T) {
	repo := &adminUsageIPRiskCapture{}
	router := newAdminUsageIPTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/ip-sharing?min_accounts=0&limit=999", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 2, repo.riskMinAccounts)
	require.Equal(t, 200, repo.riskLimit)
	require.True(t, repo.riskEnd.Sub(repo.riskStart) >= 6*24*time.Hour)
}

func TestAdminUsageIPSharingDateRange(t *testing.T) {
	repo := &adminUsageIPRiskCapture{}
	router := newAdminUsageIPTestRouter(repo)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/ip-sharing?start_date=2026-11-01&end_date=2026-11-02&timezone=UTC", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), repo.riskStart.UTC())
	require.Equal(t, time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), repo.riskEnd.UTC())
}
