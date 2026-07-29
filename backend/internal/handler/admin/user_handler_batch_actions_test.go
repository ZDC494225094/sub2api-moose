package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type batchUserActionServiceStub struct {
	service.AdminService
	updateCalls  []int64
	updateInputs []*service.UpdateUserInput
	deleteCalls  []int64
	updateErrors map[int64]error
	deleteErrors map[int64]error
}

func (s *batchUserActionServiceStub) UpdateUser(_ context.Context, id int64, input *service.UpdateUserInput) (*service.User, error) {
	s.updateCalls = append(s.updateCalls, id)
	s.updateInputs = append(s.updateInputs, input)
	if err := s.updateErrors[id]; err != nil {
		return nil, err
	}
	return &service.User{ID: id, Status: input.Status}, nil
}

func (s *batchUserActionServiceStub) DeleteUser(_ context.Context, id int64) error {
	s.deleteCalls = append(s.deleteCalls, id)
	return s.deleteErrors[id]
}

func setupBatchUserActionRouter(serviceStub service.AdminService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := NewUserHandler(serviceStub, nil, nil, nil, nil, nil, nil)
	router.POST("/api/v1/admin/users/batch-disable", handler.BatchDisable)
	router.POST("/api/v1/admin/users/batch-delete", handler.BatchDelete)
	return router
}

func postBatchUserAction(t *testing.T, router *gin.Engine, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeBatchUserActionResult(t *testing.T, recorder *httptest.ResponseRecorder) batchUserActionResult {
	t.Helper()
	var body struct {
		Data batchUserActionResult `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body.Data
}

func TestUserHandlerBatchDisableDeduplicatesAndReportsSkippedUsers(t *testing.T) {
	serviceStub := &batchUserActionServiceStub{
		updateErrors: map[int64]error{2: errors.New("cannot disable admin user")},
		deleteErrors: map[int64]error{},
	}
	recorder := postBatchUserAction(
		t,
		setupBatchUserActionRouter(serviceStub),
		"/api/v1/admin/users/batch-disable",
		[]byte(`{"user_ids":[1,2,2,3]}`),
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{1, 2, 3}, serviceStub.updateCalls)
	require.Len(t, serviceStub.updateInputs, 3)
	for _, input := range serviceStub.updateInputs {
		require.Equal(t, service.StatusDisabled, input.Status)
	}

	result := decodeBatchUserActionResult(t, recorder)
	require.Equal(t, 2, result.Affected)
	require.Equal(t, []batchUserActionSkipped{{UserID: 2, Reason: "cannot disable admin user"}}, result.Skipped)
}

func TestUserHandlerBatchDeleteReportsPerUserFailures(t *testing.T) {
	serviceStub := &batchUserActionServiceStub{
		updateErrors: map[int64]error{},
		deleteErrors: map[int64]error{5: errors.New("cannot delete admin user")},
	}
	recorder := postBatchUserAction(
		t,
		setupBatchUserActionRouter(serviceStub),
		"/api/v1/admin/users/batch-delete",
		[]byte(`{"user_ids":[4,5]}`),
	)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []int64{4, 5}, serviceStub.deleteCalls)

	result := decodeBatchUserActionResult(t, recorder)
	require.Equal(t, 1, result.Affected)
	require.Equal(t, []batchUserActionSkipped{{UserID: 5, Reason: "cannot delete admin user"}}, result.Skipped)
}

func TestUserHandlerBatchActionsRejectInvalidUserIDs(t *testing.T) {
	tooManyIDs := make([]int64, 501)
	for index := range tooManyIDs {
		tooManyIDs[index] = int64(index + 1)
	}
	tooManyBody, err := json.Marshal(map[string]any{"user_ids": tooManyIDs})
	require.NoError(t, err)

	tests := []struct {
		name string
		body []byte
	}{
		{name: "missing ids", body: []byte(`{}`)},
		{name: "non-positive id", body: []byte(`{"user_ids":[0]}`)},
		{name: "more than 500 ids", body: tooManyBody},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			serviceStub := &batchUserActionServiceStub{
				updateErrors: map[int64]error{},
				deleteErrors: map[int64]error{},
			}
			recorder := postBatchUserAction(
				t,
				setupBatchUserActionRouter(serviceStub),
				"/api/v1/admin/users/batch-disable",
				test.body,
			)

			require.Equal(t, http.StatusBadRequest, recorder.Code)
			require.Empty(t, serviceStub.updateCalls)
		})
	}
}
