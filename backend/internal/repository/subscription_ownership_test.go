package repository

import (
	"context"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestSubscriptionOwnerLocksBeforeNativeLookup(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := NewUserSubscriptionRepository(client).(*userSubscriptionRepository)
	ctx := context.Background()
	require.ErrorContains(t, repo.LockSubscriptionOwner(ctx, 7), "transaction")
	_, err = repo.FindNativeForAssignment(ctx, 7, 1)
	require.ErrorContains(t, err, "transaction")
	mock.ExpectBegin()
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	ctx = dbent.NewTxContext(ctx, tx)
	mock.ExpectQuery(`SELECT .* FROM "users" WHERE .* FOR UPDATE`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	require.NoError(t, repo.LockSubscriptionOwner(ctx, 7))
	mock.ExpectQuery(`SELECT .* FROM "user_subscriptions" WHERE .*"user_id" = .*"group_id" = .*"custom_subscription_policy" = .*`).WithArgs(int64(7), int64(1), "upstream-v1").WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "group_id", "custom_subscription_policy"}).AddRow(10, 7, 1, "upstream-v1"))
	sub, err := repo.FindNativeForAssignment(ctx, 7, 1)
	require.NoError(t, err)
	require.Equal(t, "upstream-v1", sub.CustomSubscriptionPolicy)
	require.Equal(t, int64(10), sub.ID)
	mock.ExpectQuery(`SELECT .* FROM "user_subscriptions"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	_, err = repo.FindNativeForAssignment(ctx, 7, 2)
	require.ErrorIs(t, err, service.ErrSubscriptionNotFound)
	mock.ExpectQuery(`SELECT .* FROM "users".*FOR UPDATE`).WillReturnError(errors.New("lock unavailable"))
	require.ErrorContains(t, repo.LockSubscriptionOwner(ctx, 7), "lock unavailable")
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
	require.Equal(t, "independent-v1", subscriptionPolicyOrLegacy(""))
	require.True(t, strings.HasSuffix(subscriptionPolicyOrLegacy("upstream-v1"), "v1"))
}
