package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type accountSortOrderAdminServiceStub struct {
	*stubAdminService
	updates []service.AccountSortOrderUpdate
	err     error
}

func (s *accountSortOrderAdminServiceStub) UpdateAccountSortOrders(_ context.Context, updates []service.AccountSortOrderUpdate) error {
	s.updates = append([]service.AccountSortOrderUpdate(nil), updates...)
	return s.err
}

func setupAccountSortOrderRouter() (*gin.Engine, *accountSortOrderAdminServiceStub) {
	gin.SetMode(gin.TestMode)
	adminSvc := &accountSortOrderAdminServiceStub{stubAdminService: newStubAdminService()}
	handler := NewAccountHandler(adminSvc, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	router := gin.New()
	router.PUT("/api/v1/admin/accounts/sort-order", handler.UpdateSortOrder)
	return router, adminSvc
}

func TestAccountHandlerUpdateSortOrder(t *testing.T) {
	router, adminSvc := setupAccountSortOrderRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/accounts/sort-order",
		strings.NewReader(`{"updates":[{"id":12,"sort_order":100},{"id":34,"sort_order":200}]}`),
	)
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []service.AccountSortOrderUpdate{
		{ID: 12, SortOrder: 100},
		{ID: 34, SortOrder: 200},
	}, adminSvc.updates)
}

func TestAccountHandlerUpdateSortOrderRejectsInvalidAccountID(t *testing.T) {
	router, adminSvc := setupAccountSortOrderRouter()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/admin/accounts/sort-order",
		strings.NewReader(`{"updates":[{"id":0,"sort_order":100}]}`),
	)
	request.Header.Set("Content-Type", "application/json")

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Empty(t, adminSvc.updates)
}
