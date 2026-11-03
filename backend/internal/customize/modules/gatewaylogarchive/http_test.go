package gatewaylogarchive

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type flagStore struct{ value string }

func (s flagStore) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{customize.Key(ID): s.value}, nil
}
func (flagStore) Set(context.Context, string, string) error { return nil }

type plainCipher struct{}

func (plainCipher) Encrypt(v string) (string, error) { return "enc:" + v, nil }
func (plainCipher) Decrypt(v string) (string, error) { return strings.TrimPrefix(v, "enc:"), nil }

// Disabled or corrupt switches reject before the store is read; enabled
// routes return the configuration with credentials masked.
func TestAdminRoutesRequireEnabledModuleAndMaskSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, value := range []string{"false", "invalid", "true"} {
		t.Run(value, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			h := &Handler{Runtime: NewRuntime(&Store{DB: db, Cipher: plainCipher{}}, nil, Hooks{}, t.TempDir())}
			r := gin.New()
			h.Register(r.Group("/api/v1/admin"), customize.NewManager(flagStore{value}))
			if value == "true" {
				raw, _ := json.Marshal(Config{Bucket: "incoming-04-20260624", Mode: ModeAll})
				mock.ExpectQuery("SELECT config, access_key_id, secret_access_key, session_token, updated_at FROM gateway_log_archive_config").
					WillReturnRows(sqlmock.NewRows([]string{"config", "access_key_id", "secret_access_key", "session_token", "updated_at"}).
						AddRow(string(raw), "enc:AKIAEXAMPLEKEY1234", "enc:very-secret-value", "enc:token-value", time.Now()))
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/gateway-log-archive/config", nil))
			switch value {
			case "false":
				require.Equal(t, http.StatusForbidden, w.Code)
			case "invalid":
				require.Equal(t, http.StatusServiceUnavailable, w.Code)
			default:
				require.Equal(t, http.StatusOK, w.Code)
				body := w.Body.String()
				require.Contains(t, body, `"bucket":"incoming-04-20260624"`)
				require.Contains(t, body, `"access_key_id":"AKIA****1234"`)
				require.Contains(t, body, `"secret_access_key_set":true`)
				require.NotContains(t, body, "very-secret-value")
				require.NotContains(t, body, "token-value")
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

type toggle struct{ on atomic.Bool }

func (s *toggle) Enabled(context.Context, string) (bool, error) { return s.on.Load(), nil }

// Switching the extension off removes the capture policy, so the gateway
// middleware becomes a pass-through, and drains the pipeline.
func TestRuntimeCaptureFollowsModuleSwitch(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	state := &toggle{}
	state.on.Store(true)
	r := NewRuntime(&Store{DB: db, Cipher: plainCipher{}}, state, Hooks{}, t.TempDir())
	mem := newMemStore()
	r.repo = mem
	r.objects = func(Config) (objectStore, error) { return newMemObjects(), nil }
	raw, _ := json.Marshal(Config{CaptureEnabled: true, Bucket: "b", Mode: ModeAll, MinFreeDiskGB: 0})
	mock.ExpectQuery("SELECT config").WillReturnRows(sqlmock.NewRows([]string{"config", "access_key_id", "secret_access_key", "session_token", "updated_at"}).
		AddRow(string(raw), "", "", "", nil))
	ctx := context.Background()
	r.reconcile(ctx)
	require.NotNil(t, r.policy.Load(), "enabled + configured: capturing")
	require.NotNil(t, r.current())

	state.on.Store(false)
	r.reconcile(ctx)
	require.Nil(t, r.policy.Load(), "disabled: capture middleware is a pass-through")
	require.Nil(t, r.current(), "disabled: pipeline stopped")
	require.False(t, r.Status(ctx).Capturing)
}
