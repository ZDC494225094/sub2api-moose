//go:build unit

package repository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestStorageUploadRecordsOwnerAndRefreshesLinks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer server.Close()
	store, err := NewS3ImageStorage(context.Background(), &config.ImageStorageConfig{Endpoint: server.URL, Region: "auto", AccessKeyID: "test", SecretAccessKey: "test", Bucket: "media", ForcePathStyle: true})
	require.NoError(t, err)
	store.db = db
	mock.ExpectExec("INSERT INTO object_storage_assets").WithArgs(int64(7), "media", "test.webm", "video/webm", int64(3)).WillReturnResult(sqlmock.NewResult(1, 1))
	_, err = store.Save(service.WithStorageUser(context.Background(), 7), "test.webm", "video/webm", []byte("abc"))
	require.NoError(t, err)
	mock.ExpectQuery("SELECT COUNT").WithArgs("7", "%7%").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT a.id").WithArgs("7", "%7%", 20, 0).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "email", "bucket", "key", "type", "bytes", "uploaded_at"}).AddRow(1, 7, "user@test", "media", "test.webm", "video/webm", 3, time.Now()))
	page, err := store.ListAssets(context.Background(), 1, 20, "7")
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, int64(7), *page.Items[0].UserID)
	require.Contains(t, page.Items[0].URL, "X-Amz-Signature=")
	require.NoError(t, mock.ExpectationsWereMet())
}
