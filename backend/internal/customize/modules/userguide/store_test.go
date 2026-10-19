package userguide

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func mockStore(t *testing.T) (*Store, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return &Store{DB: db}, mock
}

const configQuery = `SELECT title, content_md, updated_at FROM user_guide_config WHERE id = 1`

func TestConfigFallsBackToDefaultWhenUntouched(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectQuery(regexp.QuoteMeta(configQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"title", "content_md", "updated_at"}).
			AddRow("", "", time.Now()))
	cfg, err := store.Config(context.Background())
	require.NoError(t, err)
	require.True(t, cfg.IsDefault)
	require.Equal(t, DefaultTitle, cfg.Title)
	require.Equal(t, defaultContent, cfg.ContentMD)
	require.Contains(t, cfg.ContentMD, "/guide-images/")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestConfigReturnsSavedValues(t *testing.T) {
	store, mock := mockStore(t)
	now := time.Now()
	mock.ExpectQuery(regexp.QuoteMeta(configQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"title", "content_md", "updated_at"}).
			AddRow("我的手册", "# 你好", now))
	cfg, err := store.Config(context.Background())
	require.NoError(t, err)
	require.False(t, cfg.IsDefault)
	require.Equal(t, "我的手册", cfg.Title)
	require.Equal(t, "# 你好", cfg.ContentMD)
	require.NotNil(t, cfg.UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSaveUpsertsAndReloads(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectExec("INSERT INTO user_guide_config").
		WithArgs("标题", "正文").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(configQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"title", "content_md", "updated_at"}).
			AddRow("标题", "正文", time.Now()))
	cfg, err := store.Save(context.Background(), "标题", "正文")
	require.NoError(t, err)
	require.Equal(t, "正文", cfg.ContentMD)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestResetClearsToDefault(t *testing.T) {
	store, mock := mockStore(t)
	mock.ExpectExec("UPDATE user_guide_config SET title = '', content_md = ''").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(configQuery)).
		WillReturnRows(sqlmock.NewRows([]string{"title", "content_md", "updated_at"}).
			AddRow("", "", time.Now()))
	cfg, err := store.Reset(context.Background())
	require.NoError(t, err)
	require.True(t, cfg.IsDefault)
	require.NoError(t, mock.ExpectationsWereMet())
}
