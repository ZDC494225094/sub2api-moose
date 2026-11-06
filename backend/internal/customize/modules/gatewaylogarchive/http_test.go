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

var bucketRowColumns = []string{"id", "name", "provider", "bucket", "region", "endpoint", "prefix", "force_path_style",
	"access_key_id", "secret_access_key", "session_token", "enabled", "priority", "created_at", "updated_at"}

func migratedConfigRow() *sqlmock.Rows {
	raw, _ := json.Marshal(Config{Mode: ModeAll, BucketsMigrated: true})
	return sqlmock.NewRows([]string{"config", "access_key_id", "secret_access_key", "session_token"}).AddRow(string(raw), "", "", "")
}

// Disabled or corrupt switches reject before the store is read; enabled
// routes list the buckets with credentials masked.
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
				now := time.Now()
				mock.ExpectQuery("SELECT config, access_key_id, secret_access_key, session_token FROM gateway_log_archive_config").
					WillReturnRows(migratedConfigRow())
				mock.ExpectQuery("FROM gateway_log_archive_buckets ORDER BY priority, id").
					WillReturnRows(sqlmock.NewRows(bucketRowColumns).AddRow(3, "主存储", ProviderAWS, "incoming-04-20260624", "us-east-1", "", "logs",
						false, "enc:AKIAEXAMPLEKEY1234", "enc:very-secret-value", "enc:token-value", true, 10, now, now))
				mock.ExpectQuery("FROM gateway_log_archive_upload_stats GROUP BY bucket_id").
					WillReturnRows(sqlmock.NewRows([]string{"bucket_id", "bundles", "bytes"}).AddRow(3, 2, 2048).AddRow(9, 1, 512))
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/gateway-log-archive/buckets", nil))
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
				require.Contains(t, body, `"usage":{"bundle_count":2,"size_bytes":2048}`)
				require.Contains(t, body, `"unattributed":{"bundle_count":1,"size_bytes":512}`, "deleted bucket totals stay visible")
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
	r.objects = func(BucketConfig) (objectStore, error) { return newMemObjects(), nil }
	r.buckets = func(context.Context) ([]BucketConfig, error) {
		return []BucketConfig{{ID: 1, Bucket: "b", Enabled: true}}, nil
	}
	raw, _ := json.Marshal(Config{CaptureEnabled: true, Mode: ModeAll, MinFreeDiskGB: 0, BucketsMigrated: true})
	mock.ExpectQuery("SELECT config, updated_at").WillReturnRows(sqlmock.NewRows([]string{"config", "updated_at"}).AddRow(string(raw), nil))
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

// Without an enabled bucket capture stays off even when it is switched on.
func TestRuntimeNeedsAnEnabledBucket(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	state := &toggle{}
	state.on.Store(true)
	r := NewRuntime(&Store{DB: db, Cipher: plainCipher{}}, state, Hooks{}, t.TempDir())
	r.repo = newMemStore()
	r.buckets = func(context.Context) ([]BucketConfig, error) {
		return []BucketConfig{{ID: 1, Bucket: "b", Enabled: false}}, nil
	}
	raw, _ := json.Marshal(Config{CaptureEnabled: true, Mode: ModeAll, BucketsMigrated: true})
	mock.ExpectQuery("SELECT config, updated_at").WillReturnRows(sqlmock.NewRows([]string{"config", "updated_at"}).AddRow(string(raw), nil))
	r.reconcile(context.Background())
	t.Cleanup(r.Stop)
	require.Nil(t, r.policy.Load())
	require.Equal(t, "尚未配置可用的存储桶", r.reason)
	if pipe := r.current(); pipe != nil {
		pipe.stop(false)
	}
}

// The single bucket of earlier versions becomes bucket #1 exactly once, with
// its credentials copied still encrypted and old uploads attributed to it.
func TestLegacyBucketIsAdoptedOnce(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := &Store{DB: db, Cipher: plainCipher{}}
	legacy := `{"capture_enabled":true,"mode":"all","rules":[],"provider":"aws","bucket":"incoming-04","region":"us-west-2","prefix":"logs"}`
	mock.ExpectQuery("SELECT config, access_key_id, secret_access_key, session_token FROM gateway_log_archive_config").
		WillReturnRows(sqlmock.NewRows([]string{"config", "access_key_id", "secret_access_key", "session_token"}).
			AddRow(legacy, "enc:AKIAEXAMPLEKEY1234", "enc:secret", ""))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE gateway_log_archive_config SET config = \\$1").
		WithArgs(sqlmock.AnyArg(), legacy).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("INSERT INTO gateway_log_archive_buckets").
		WithArgs("默认存储桶", ProviderAWS, "incoming-04", "us-west-2", "", "logs", false, "enc:AKIAEXAMPLEKEY1234", "enc:secret", "").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectExec("UPDATE gateway_log_archive_bundles SET bucket_id = \\$1").WithArgs(int64(7), "incoming-04").WillReturnResult(sqlmock.NewResult(0, 3))
	mock.ExpectExec("UPDATE gateway_log_archive_upload_stats SET bucket_id = \\$1").WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	require.NoError(t, s.ensureBuckets(context.Background()))
	require.NoError(t, s.ensureBuckets(context.Background()), "second call issues no queries")
	require.NoError(t, mock.ExpectationsWereMet())
}

// A concurrent instance that adopted first makes the compare-and-swap miss;
// nothing is inserted and the next call re-reads the migrated row.
func TestLegacyBucketAdoptionLosesRaceSafely(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s := &Store{DB: db, Cipher: plainCipher{}}
	legacy := `{"mode":"all","bucket":"incoming-04"}`
	mock.ExpectQuery("SELECT config, access_key_id").WillReturnRows(sqlmock.NewRows([]string{"config", "access_key_id", "secret_access_key", "session_token"}).AddRow(legacy, "", "", ""))
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE gateway_log_archive_config SET config").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	mock.ExpectQuery("SELECT config, access_key_id").WillReturnRows(migratedConfigRow())
	require.NoError(t, s.ensureBuckets(context.Background()))
	require.NoError(t, s.ensureBuckets(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBucketRequestKeepsStoredSecrets(t *testing.T) {
	stored := BucketConfig{ID: 3, Bucket: "bucket", AccessKeyID: "AKIAEXAMPLEKEY1234", SecretAccessKey: "secret", SessionToken: "tok", Enabled: true, Priority: 5}
	str := func(v string) *string { return &v }

	b, err := bucketRequest{Bucket: "bucket", AccessKeyID: str("AKIA****1234"), SecretAccessKey: str("")}.merge(stored)
	require.NoError(t, err)
	require.Equal(t, "AKIAEXAMPLEKEY1234", b.AccessKeyID, "masked value keeps the stored key")
	require.Equal(t, "secret", b.SecretAccessKey)
	require.True(t, b.Enabled, "omitted switch keeps the stored state")
	require.Equal(t, 5, b.Priority)

	off, prio := false, 1
	b, err = bucketRequest{Bucket: "bucket", SecretAccessKey: str("new-secret"), Enabled: &off, Priority: &prio, ClearSessionToken: true}.merge(stored)
	require.NoError(t, err)
	require.Equal(t, "new-secret", b.SecretAccessKey)
	require.Empty(t, b.SessionToken)
	require.False(t, b.Enabled)
	require.Equal(t, 1, b.Priority)

	b, err = bucketRequest{Bucket: "bucket", ClearCredentials: true}.merge(stored)
	require.NoError(t, err)
	require.Empty(t, b.AccessKeyID+b.SecretAccessKey+b.SessionToken)
}

func TestResolveBucketByIDThenLegacyName(t *testing.T) {
	all := []BucketConfig{{ID: 1, Bucket: "a"}, {ID: 2, Bucket: "b"}}
	id := int64(2)
	got, ok := resolveBucket(all, Bundle{BucketID: &id, Bucket: "a"})
	require.True(t, ok)
	require.Equal(t, int64(2), got.ID, "the recorded ID wins over the name")
	got, ok = resolveBucket(all, Bundle{Bucket: "a"})
	require.True(t, ok)
	require.Equal(t, int64(1), got.ID)
	gone := int64(9)
	_, ok = resolveBucket(all, Bundle{BucketID: &gone, Bucket: "a"})
	require.False(t, ok, "a deleted bucket is not silently replaced by a namesake")
}
