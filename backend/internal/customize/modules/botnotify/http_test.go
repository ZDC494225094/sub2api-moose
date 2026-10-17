package botnotify

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type flagStore struct{ enabled bool }

func (s flagStore) GetMultiple(context.Context, []string) (map[string]string, error) {
	value := "false"
	if s.enabled {
		value = "true"
	}
	return map[string]string{customize.Key(ID): value}, nil
}
func (s flagStore) Set(context.Context, string, string) error { return nil }

func TestRoutesGateDisabledModuleAndMaskConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	h := &Handler{Runtime: NewRuntime(&Store{DB: db, Cipher: testCipher{}}, map[string]Platform{"telegram": &testPlatform{}}, &testData{}, enabledState{true})}
	router := gin.New()
	h.Register(router.Group("/admin"), customize.NewManager(flagStore{false}))
	for _, path := range []string{"bots", "bots/1", "bots/1/status", "bots/1/messages", "bots/1/rules"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/bot-notify/"+path, nil))
		require.Equal(t, http.StatusForbidden, w.Code)
	}
	router = gin.New()
	h.Register(router.Group("/admin"), customize.NewManager(flagStore{true}))
	mock.ExpectQuery("SELECT id,name,platform,config FROM bot_notify_bots").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "platform", "config"}).AddRow(1, "默认机器人", "telegram", []byte(`{"bot_token":"encrypted:secret1234"}`)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/bot-notify/bots/1", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "****1234")
	require.NotContains(t, w.Body.String(), "secret")
	require.NoError(t, mock.ExpectationsWereMet())
}
