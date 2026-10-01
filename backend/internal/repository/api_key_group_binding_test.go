package repository

import (
	"context"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/multigroupbilling"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"regexp"
	"testing"
)

func TestAPIKeyGroupBindingSQLPort(t *testing.T) {
	for _, replace := range []bool{false, true} {
		for _, outcome := range []string{"ok", "exec error", "result error"} {
			name := outcome
			if replace {
				name = "replace/" + name
			} else {
				name = "clear/" + name
			}
			t.Run(name, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer db.Close()
				repo := newAPIKeyRepositoryWithSQL(nil, db)
				query := multigroupbilling.ClearGroupBindingsSQL
				if replace {
					query = multigroupbilling.ReplaceGroupBindingsSQL
				}
				expectation := mock.ExpectExec(regexp.QuoteMeta(query))
				if replace {
					expectation.WithArgs(int64(7), int64(1), int64(2), "[1]", service.PlatformAnthropic)
				} else {
					expectation.WithArgs(int64(1), "[1]")
				}
				failure := errors.New("storage failure")
				switch outcome {
				case "exec error":
					expectation.WillReturnError(failure)
				case "result error":
					expectation.WillReturnResult(sqlmock.NewErrorResult(failure))
				default:
					expectation.WillReturnResult(sqlmock.NewResult(0, 3))
				}
				var affected int64
				if replace {
					affected, err = repo.UpdateGroupIDByUserAndGroup(context.Background(), 7, 1, 2)
				} else {
					affected, err = repo.ClearGroupIDByGroupID(context.Background(), 1)
				}
				if outcome == "ok" {
					require.NoError(t, err)
					require.EqualValues(t, 3, affected)
				} else {
					require.ErrorIs(t, err, failure)
					require.Zero(t, affected)
				}
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}
func TestAPIKeyGroupBindingTransactionUsesEnt(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "clear", true: "replace"}[replace], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			mock.ExpectBegin()
			tx, err := client.Tx(context.Background())
			require.NoError(t, err)
			ctx := dbent.NewTxContext(context.Background(), tx)
			repo := newAPIKeyRepositoryWithSQL(client, db)
			failure := errors.New("transaction read failure")
			if replace {
				mock.ExpectQuery(`SELECT .* FROM "groups"`).WillReturnError(failure)
			} else {
				mock.ExpectQuery(`SELECT .* FROM "api_keys"`).WillReturnError(failure)
			}
			if replace {
				_, err = repo.UpdateGroupIDByUserAndGroup(ctx, 7, 1, 2)
			} else {
				_, err = repo.ClearGroupIDByGroupID(ctx, 1)
			}
			require.ErrorIs(t, err, failure)
			mock.ExpectRollback()
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// These tests exercise the Ent mutation path inside the caller's transaction,
// including primary clearing and secondary replacement, without a root SQL write.
func TestAPIKeyGroupBindingTransactionMutations(t *testing.T) {
	for _, replace := range []bool{false, true} {
		t.Run(map[bool]string{false: "clear", true: "replace"}[replace], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			mock.ExpectBegin()
			tx, err := client.Tx(context.Background())
			require.NoError(t, err)
			ctx := dbent.NewTxContext(context.Background(), tx)
			repo := newAPIKeyRepositoryWithSQL(client, db)
			if replace {
				mock.ExpectQuery(`SELECT .* FROM "groups"`).WillReturnRows(sqlmock.NewRows([]string{"id", "platform"}).AddRow(3, service.PlatformOpenAI))
			}
			mock.ExpectQuery(`SELECT .* FROM "api_keys"`).WillReturnRows(sqlmock.NewRows([]string{"id", "group_id", "group_ids"}).AddRow(9, 1, `[1,2]`))
			// Assert preserved primary/list values, not merely that some UPDATE occurred.
			if replace {
				mock.ExpectExec(`UPDATE "api_keys" SET "updated_at" = \$1, "platform" = \$2, "group_ids" = \$3, "group_id" = \$4 WHERE`).WithArgs(sqlmock.AnyArg(), service.PlatformOpenAI, []byte(`[1,3]`), int64(1), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				mock.ExpectExec(`UPDATE "api_keys" SET "group_id" = NULL, "updated_at" = \$1, "group_ids" = \$2 WHERE`).WithArgs(sqlmock.AnyArg(), []byte(`[2]`), int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			var affected int64
			if replace {
				affected, err = repo.UpdateGroupIDByUserAndGroup(ctx, 7, 2, 3)
			} else {
				affected, err = repo.ClearGroupIDByGroupID(ctx, 1)
			}
			require.NoError(t, err)
			require.EqualValues(t, 1, affected)
			mock.ExpectRollback()
			require.NoError(t, tx.Rollback())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
