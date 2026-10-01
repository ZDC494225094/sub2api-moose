//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/stretchr/testify/require"
)

func TestMarketingLotteryTransactionBridgeCommitRollbackAndBeginFailure(t *testing.T) {
	for _, failure := range []string{"", "begin", "write", "callback", "commit"} {
		t.Run("failure="+failure, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			// A root-client call cannot secretly escape the in-use transaction.
			db.SetMaxOpenConns(1)
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			type requestKey struct{}
			ctx = context.WithValue(ctx, requestKey{}, "caller request")
			wantErr := errors.New("injected " + failure)
			begin := mock.ExpectBegin()
			if failure == "begin" {
				begin.WillReturnError(wantErr)
			} else {
				stmt := mock.ExpectExec(regexp.QuoteMeta("UPDATE marketing_bridge_probe SET value=$1")).WithArgs(42)
				if failure == "write" {
					stmt.WillReturnError(wantErr)
				} else {
					stmt.WillReturnResult(sqlmock.NewResult(0, 1))
				}
				if failure == "write" || failure == "callback" {
					mock.ExpectRollback()
				} else {
					commit := mock.ExpectCommit()
					if failure == "commit" {
						commit.WillReturnError(wantErr)
					}
				}
			}
			called := 0
			err = (lotteryEntTransactions{client}).WithinLotteryTransaction(ctx, func(txCtx context.Context) error {
				called++
				require.Equal(t, "caller request", txCtx.Value(requestKey{}))
				tx := dbent.TxFromContext(txCtx)
				require.NotNil(t, tx)
				_, writeErr := tx.Client().ExecContext(txCtx, "UPDATE marketing_bridge_probe SET value=$1", 42)
				if writeErr != nil {
					return writeErr
				}
				if failure == "callback" {
					return wantErr
				}
				return nil
			})
			if failure == "" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, wantErr)
			}
			if failure == "begin" {
				require.Zero(t, called)
			} else {
				require.Equal(t, 1, called)
			}
			mock.ExpectClose()
			require.NoError(t, client.Close())
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestMarketingLotteryTransactionBridgeRejectsMissingHost(t *testing.T) {
	err := (lotteryEntTransactions{}).WithinLotteryTransaction(context.Background(), func(context.Context) error { t.Fatal("must not enter an unconfigured transaction"); return nil })
	require.ErrorContains(t, err, "lottery ent client is not configured")
}

type marketingRedeemRepo struct {
	RedeemCodeRepository
	ctx     context.Context
	created *RedeemCode
	err     error
}

func (r *marketingRedeemRepo) Create(ctx context.Context, code *RedeemCode) error {
	r.ctx = ctx
	r.created = code
	code.ID = 71
	return r.err
}
func TestMarketingLotteryRewardBridgeUsesNativeRedeemServiceAndJSON(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "host transaction")
	repo := &marketingRedeemRepo{}
	issuer := lotteryHostRedeemIssuer{&RedeemService{redeemRepo: repo}}
	code, reward, err := issuer.IssueBalanceRedeem(ctx, marketing.BalanceRedeemInput{UserID: 42, Amount: 5, Notes: "lottery activity 9 prize 11 (five)"})
	require.NoError(t, err)
	require.Equal(t, ctx, repo.ctx)
	require.Same(t, repo.created, reward)
	require.Regexp(t, "^[0-9A-F]{32}$", code)
	require.Equal(t, RedeemTypeBalance, reward.Type)
	require.Equal(t, StatusUnused, reward.Status)
	require.Equal(t, float64(5), reward.Value)
	require.Equal(t, int64(71), reward.ID)
	require.Equal(t, "lottery activity 9 prize 11 (five)", reward.Notes)
	// Additional upstream fields continue to serialize because T is the actual
	// host type, not an extension-maintained copy of the RedeemCode DTO.
	batch := "native-batch"
	expires := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	reward.BatchID = &batch
	reward.ExpiresAt = &expires
	reward.ValidityDays = 12
	wantJSON, err := json.Marshal(struct {
		RedeemCode *RedeemCode `json:"redeem_code,omitempty"`
	}{reward})
	require.NoError(t, err)
	gotJSON, err := json.Marshal(&LotteryDrawResult{RedeemCode: reward})
	require.NoError(t, err)
	require.JSONEq(t, string(wantJSON), string(gotJSON))
	repo.err = errors.New("native redeem insert failed")
	code, reward, err = issuer.IssueBalanceRedeem(ctx, marketing.BalanceRedeemInput{Amount: 5})
	require.ErrorIs(t, err, repo.err)
	require.Empty(t, code)
	require.Nil(t, reward)
}

type marketingWalletRepo struct {
	UserRepository
	ctx            context.Context
	balance, delta float64
	err            error
}

func (r *marketingWalletRepo) GetByID(ctx context.Context, id int64) (*User, error) {
	r.ctx = ctx
	if r.err != nil {
		return nil, r.err
	}
	return &User{ID: id, Balance: r.balance}, nil
}
func (r *marketingWalletRepo) UpdateBalance(ctx context.Context, id int64, delta float64) error {
	r.ctx = ctx
	r.delta = delta
	return r.err
}
func TestMarketingLotteryWalletBridgePreservesNativeAmountsAndContext(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "host transaction")
	repo := &marketingWalletRepo{balance: 12.34}
	port := lotteryHostWallet{repo}
	balance, err := port.GetBalance(ctx, 42)
	require.NoError(t, err)
	require.Equal(t, 12.34, balance)
	require.Equal(t, ctx, repo.ctx)
	require.NoError(t, port.UpdateBalance(ctx, 42, -2.5))
	require.Equal(t, -2.5, repo.delta)
	require.Equal(t, ctx, repo.ctx)
	repo.err = errors.New("host wallet failed")
	_, err = port.GetBalance(ctx, 42)
	require.ErrorIs(t, err, repo.err)
	require.ErrorIs(t, port.UpdateBalance(ctx, 42, -2.5), repo.err)
}
