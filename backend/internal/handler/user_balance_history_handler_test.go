//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type balanceHistoryHandlerRepoStub struct {
	userID int64
	from   string
	to     string
}

func (repo *balanceHistoryHandlerRepoStub) EnsureCurrentMonthSnapshot(context.Context, int64, time.Time) error {
	return nil
}

func (repo *balanceHistoryHandlerRepoStub) SnapshotPreviousMonth(context.Context, time.Time) error {
	return nil
}

func (repo *balanceHistoryHandlerRepoStub) ListHistory(_ context.Context, userID int64, from, to string) ([]service.BalanceHistoryPoint, error) {
	repo.userID, repo.from, repo.to = userID, from, to
	return []service.BalanceHistoryPoint{{Period: to, Balance: 12.5}}, nil
}

func (repo *balanceHistoryHandlerRepoStub) GetCurrentBalance(context.Context, int64) (float64, error) {
	return 0, nil
}

func TestGetBalanceHistoryRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/balance/history", nil)
	(&UserHandler{}).GetBalanceHistory(ctx)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestGetBalanceHistoryQueryAndAuthenticatedUser(t *testing.T) {
	for _, test := range []struct {
		query  string
		months int
	}{
		{"", 12}, {"?months=3", 3}, {"?months=6", 6}, {"?months=12", 12},
		{"?months=4", 12}, {"?months=-1", 12}, {"?months=abc", 12}, {"?months=", 12},
	} {
		t.Run(test.query, func(t *testing.T) {
			repo := &balanceHistoryHandlerRepoStub{}
			handler := &UserHandler{balanceSnapshotService: service.NewUserBalanceSnapshotService(repo)}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/user/balance/history"+test.query, nil)
			ctx.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			handler.GetBalanceHistory(ctx)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, int64(42), repo.userID)
			currentMonth, err := time.Parse("2006-01", repo.to)
			require.NoError(t, err)
			require.Equal(t, currentMonth.AddDate(0, 1-test.months, 0).Format("2006-01"), repo.from)
			var body struct {
				Data []service.BalanceHistoryPoint `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, []service.BalanceHistoryPoint{{Period: repo.to, Balance: 12.5}}, body.Data)
		})
	}
}
