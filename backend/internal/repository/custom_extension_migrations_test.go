package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type migrationLifecycleProbe struct {
	prepare func(context.Context, *sql.Conn) error
	apply   func(context.Context, *sql.Conn) error
}

func (p migrationLifecycleProbe) Prepare(ctx context.Context, conn *sql.Conn) error {
	return p.prepare(ctx, conn)
}
func (p migrationLifecycleProbe) Apply(ctx context.Context, conn *sql.Conn) error {
	return p.apply(ctx, conn)
}

func TestExtensionMigrationLifecycleSharesLockedConnectionAndOrdering(t *testing.T) {
	for _, failure := range []string{"none", "prepare", "host", "apply"} {
		t.Run(failure, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			db.SetMaxOpenConns(1) // A second connection would deadlock: never acquire one in a hook.
			defer db.Close()
			mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(migrationsAdvisoryLockID).
				WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
			mock.ExpectExec("CREATE TABLE IF NOT EXISTS schema_migrations").WillReturnResult(sqlmock.NewResult(0, 0))
			before := mock.ExpectExec("SELECT 'extension-before'")
			if failure == "prepare" {
				before.WillReturnError(errors.New("prepare failed"))
			} else {
				before.WillReturnResult(sqlmock.NewResult(0, 0))
			}
			if failure != "prepare" {
				mock.ExpectQuery(`SELECT EXISTS \(`).WithArgs("schema_migrations").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				mock.ExpectQuery(`SELECT EXISTS \(`).WithArgs("atlas_schema_revisions").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM atlas_schema_revisions`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				host := mock.ExpectQuery(`SELECT checksum FROM schema_migrations WHERE filename = \$1`).WithArgs("001_host.sql")
				if failure == "host" {
					host.WillReturnError(errors.New("host failed"))
				} else {
					host.WillReturnRows(sqlmock.NewRows([]string{"checksum"}).AddRow(migrationChecksum("SELECT 'host';")))
					after := mock.ExpectExec("SELECT 'extension-after'")
					if failure == "apply" {
						after.WillReturnError(errors.New("apply failed"))
					} else {
						after.WillReturnResult(sqlmock.NewResult(0, 0))
					}
				}
			}
			mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(migrationsAdvisoryLockID).WillReturnResult(sqlmock.NewResult(0, 1))
			var pinned *sql.Conn
			applyCalled := false
			hooks := migrationLifecycleProbe{
				prepare: func(ctx context.Context, conn *sql.Conn) error {
					pinned = conn
					_, err := conn.ExecContext(ctx, "SELECT 'extension-before'")
					return err
				},
				apply: func(ctx context.Context, conn *sql.Conn) error {
					applyCalled = true
					require.Same(t, pinned, conn)
					_, err := conn.ExecContext(ctx, "SELECT 'extension-after'")
					return err
				},
			}
			err = applyMigrationsFS(context.Background(), db, fstest.MapFS{"001_host.sql": &fstest.MapFile{Data: []byte("SELECT 'host';")}}, hooks)
			if failure == "none" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, failure+" failed")
			}
			require.Equal(t, failure == "none" || failure == "apply", applyCalled)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
