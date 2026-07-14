package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupAccountUpstreamGroupsRouter() (*gin.Engine, *stubAdminService) {
	gin.SetMode(gin.TestMode)
	adminSvc := newStubAdminService()
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.GET("/api/v1/admin/accounts/upstream-groups", handler.ListUpstreamGroups)
	return router, adminSvc
}

func TestAccountHandlerListUpstreamGroups(t *testing.T) {
	router, adminSvc := setupAccountUpstreamGroupsRouter()
	adminSvc.accountUpstreamGroups = []service.AccountUpstreamGroup{
		{Key: "edge-china", Name: "Edge China", AccountCount: 3},
		{Key: "official", Name: "Official", AccountCount: 1},
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/upstream-groups", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Groups []service.AccountUpstreamGroup `json:"groups"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Zero(t, envelope.Code)
	require.Equal(t, "success", envelope.Message)
	require.Equal(t, adminSvc.accountUpstreamGroups, envelope.Data.Groups)
}

func TestAccountHandlerListUpstreamGroupsReturnsServiceError(t *testing.T) {
	router, adminSvc := setupAccountUpstreamGroupsRouter()
	adminSvc.accountUpstreamGroupsErr = infraerrors.InternalServer(
		"ACCOUNT_UPSTREAM_GROUPS_FAILED",
		"unable to list upstream groups",
	)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/accounts/upstream-groups", nil)

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var envelope struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Reason  string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	require.Equal(t, http.StatusInternalServerError, envelope.Code)
	require.Equal(t, "unable to list upstream groups", envelope.Message)
	require.Equal(t, "ACCOUNT_UPSTREAM_GROUPS_FAILED", envelope.Reason)
}
