package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func marketingMockDB(t *testing.T) (*sql.DB, *dbent.Client, sqlmock.Sqlmock, context.Context) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(func() {
		cancel()
		mock.ExpectClose()
		require.NoError(t, client.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
	return db, client, mock, ctx
}

// Uses the real host service, Ent adapter, marketing repositories and native
// redeem repository. A leaked root-pool query deadlocks until the short context
// expires because the host transaction owns the only connection. This verifies
// connection/rollback protocol, not real PostgreSQL locking or concurrent load.
func TestMarketingDrawKeepsRepositoriesAndRewardsInsideHostTransaction(t *testing.T) {
	for _, kind := range []string{service.LotteryPrizeTypeBalanceRedeem, service.LotteryPrizeTypeCoupon, service.LotteryPrizeTypeThanks} {
		for _, outcome := range []string{"commit", "record_failure", "debit_rejected"} {
			t.Run(kind+"/"+outcome, func(t *testing.T) {
				failRecord := outcome == "record_failure"
				db, client, mock, ctx := marketingMockDB(t)
				now := time.Now().UTC()
				wantErr := errors.New("draw record insert failed")
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT .* FROM lottery_activities .* FOR UPDATE").WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "status", "default_draw_times", "consume_threshold_amount", "wallet_cost_per_draw", "starts_at", "ends_at", "sort_order", "created_at", "updated_at"}).AddRow(9, "activity", "", service.LotteryActivityStatusActive, 0, 0, 2, nil, nil, 0, now, now)).RowsWillBeClosed()
				mock.ExpectExec("INSERT INTO lottery_user_states").WithArgs(int64(9), int64(42)).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectQuery("SELECT .* FROM lottery_user_states .* FOR UPDATE").WithArgs(int64(9), int64(42)).WillReturnRows(sqlmock.NewRows([]string{"activity_id", "user_id", "default_granted", "available_draw_times", "total_granted_times", "total_drawn_times", "total_wallet_paid_amount", "created_at", "updated_at"}).AddRow(9, 42, true, 0, 0, 0, 0, now, now)).RowsWillBeClosed()
				mock.ExpectQuery("SELECT .*balance.* FROM .users.").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(20)).RowsWillBeClosed()
				if outcome == "debit_rejected" {
					// Another activity spent this balance after the read. The
					// conditional debit must reject without granting any reward.
					mock.ExpectExec("UPDATE .users. SET .* WHERE .*balance.* >=").WillReturnResult(sqlmock.NewResult(0, 0))
					mock.ExpectRollback()
					users := NewUserRepository(client, db)
					lottery := newRepositoryMarketingTestLotteryService(client, NewLotteryActivityRepository(db), NewLotteryPrizeRepository(db), NewLotteryUserStateRepository(db), NewLotteryChanceLogRepository(db), NewLotteryDrawRecordRepository(db), NewLotteryConsumeProgressRepository(db), users, nil, nil)
					result, err := lottery.Draw(ctx, service.LotteryDrawInput{UserID: 42, ActivityID: 9, UseWallet: true})
					require.ErrorIs(t, err, marketing.ErrLotteryWalletBalanceInsufficient)
					require.Nil(t, result)
					return
				}
				mock.ExpectExec("UPDATE .users. SET .* WHERE .*balance.* >=").WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery("UPDATE lottery_user_states .* RETURNING updated_at").WithArgs(int64(9), int64(42), true, 0, 0, 1, float64(2)).WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(now)).RowsWillBeClosed()
				mock.ExpectQuery("INSERT INTO lottery_chance_logs").WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(10, now)).RowsWillBeClosed()
				mock.ExpectQuery("SELECT .* FROM lottery_prizes .* FOR UPDATE").WithArgs(int64(9), true).WillReturnRows(sqlmock.NewRows([]string{"id", "activity_id", "name", "prize_type", "stock", "remaining_stock", "balance_amount", "coupon_template_id", "display_order", "status", "created_at", "updated_at"}).AddRow(11, 9, "prize", kind, 1, 1, 5, 12, 0, service.LotteryPrizeStatusActive, now, now)).RowsWillBeClosed()
				mock.ExpectExec("UPDATE lottery_prizes").WithArgs(int64(11)).WillReturnResult(sqlmock.NewResult(0, 1))
				switch kind {
				case service.LotteryPrizeTypeBalanceRedeem:
					mock.ExpectQuery("INSERT INTO .redeem_codes.").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(77)).RowsWillBeClosed()
				case service.LotteryPrizeTypeCoupon:
					mock.ExpectQuery("SELECT .* FROM coupon_templates").WithArgs(int64(12)).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "description", "scope", "discount_amount", "threshold_amount", "valid_days", "valid_from", "valid_until", "status", "notes", "created_at", "updated_at"}).AddRow(12, "coupon", "", service.CouponScopeBalance, 5, 10, nil, nil, nil, service.CouponTemplateStatusActive, "", now, now)).RowsWillBeClosed()
					mock.ExpectQuery("INSERT INTO user_coupons").WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(88, now, now)).RowsWillBeClosed()
				}
				record := mock.ExpectQuery("INSERT INTO lottery_draw_records")
				if failRecord {
					record.WillReturnError(wantErr)
					mock.ExpectRollback()
				} else {
					record.WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(99, now)).RowsWillBeClosed()
					mock.ExpectCommit()
				}
				users := NewUserRepository(client, db)
				redeem := service.NewRedeemService(NewRedeemCodeRepository(client), users, nil, nil, nil, client, nil, nil)
				coupons := newRepositoryMarketingTestCouponService(NewCouponTemplateRepository(db), NewUserCouponRepository(db), NewPaymentOrderDiscountRepository(db))
				lottery := newRepositoryMarketingTestLotteryService(client, NewLotteryActivityRepository(db), NewLotteryPrizeRepository(db), NewLotteryUserStateRepository(db), NewLotteryChanceLogRepository(db), NewLotteryDrawRecordRepository(db), NewLotteryConsumeProgressRepository(db), users, redeem, coupons)
				result, err := lottery.Draw(ctx, service.LotteryDrawInput{UserID: 42, ActivityID: 9, UseWallet: true})
				if failRecord {
					require.ErrorIs(t, err, wantErr)
					require.Nil(t, result)
					return
				}
				require.NoError(t, err)
				require.NotNil(t, result.Record)
				require.Equal(t, int64(99), result.Record.ID)
				require.Equal(t, float64(2), result.Record.WalletAmount)
				switch kind {
				case service.LotteryPrizeTypeBalanceRedeem:
					require.Equal(t, int64(77), result.RedeemCode.ID)
				case service.LotteryPrizeTypeCoupon:
					require.Equal(t, int64(88), result.UserCoupon.ID)
				case service.LotteryPrizeTypeThanks:
					require.Equal(t, service.LotteryDrawResultThanks, result.Record.ResultCode)
				}
			})
		}
	}
}

func TestMarketingCouponHistoricalTransitionsShareCallerTransaction(t *testing.T) {
	for _, op := range []string{"consume", "release"} {
		for _, failSecond := range []bool{false, true} {
			t.Run(op+map[bool]string{true: "/rollback", false: "/commit"}[failSecond], func(t *testing.T) {
				db, client, mock, ctx := marketingMockDB(t)
				wantErr := errors.New("coupon transition failed")
				mock.ExpectBegin()
				tx, err := client.Tx(ctx)
				require.NoError(t, err)
				txCtx := dbent.NewTxContext(ctx, tx)
				mock.ExpectExec("UPDATE payment_order_discounts").WillReturnResult(sqlmock.NewResult(0, 1))
				second := mock.ExpectExec("UPDATE user_coupons")
				if failSecond {
					second.WillReturnError(wantErr)
					mock.ExpectRollback()
				} else {
					second.WillReturnResult(sqlmock.NewResult(0, 1))
					mock.ExpectCommit()
				}
				coupons := newRepositoryMarketingTestCouponService(nil, NewUserCouponRepository(db), NewPaymentOrderDiscountRepository(db))
				if op == "consume" {
					err = coupons.ConsumeReservedCouponByOrderID(txCtx, 91)
				} else {
					err = coupons.ReleaseCouponReservationByOrderID(txCtx, 91)
				}
				if failSecond {
					require.ErrorIs(t, err, wantErr)
					require.NoError(t, tx.Rollback())
				} else {
					require.NoError(t, err)
					require.NoError(t, tx.Commit())
				}
			})
		}
	}
}

func TestMarketingSQLUsesRootOnlyOutsideTransaction(t *testing.T) {
	db, client, mock, ctx := marketingMockDB(t)
	require.Same(t, db, marketingSQL(ctx, db))
	mock.ExpectBegin()
	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	require.Same(t, tx.Client(), marketingSQL(dbent.NewTxContext(ctx, tx), db))
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
}

func TestMarketingCouponReservationUsesCallerTransaction(t *testing.T) {
	for _, failure := range []string{"", "claim_lost", "discount_insert"} {
		t.Run("reservation/"+failure, func(t *testing.T) {
			db, client, mock, ctx := marketingMockDB(t)
			now := time.Now().UTC()
			mock.ExpectBegin()
			tx, err := client.Tx(ctx)
			require.NoError(t, err)
			txCtx := dbent.NewTxContext(ctx, tx)
			mock.ExpectQuery("SELECT .* FROM user_coupons WHERE id = ").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "template_id", "user_id", "coupon_code", "source_type", "source_ref_id", "scope", "discount_amount", "threshold_amount", "valid_from", "valid_until", "status", "reserved_order_id", "reserved_at", "used_order_id", "used_at", "created_at", "updated_at"}).AddRow(7, 3, 42, "coupon", "admin", nil, service.CouponScopeBalance, 5, 10, nil, nil, service.UserCouponStatusUnused, nil, nil, nil, nil, now, now)).RowsWillBeClosed()
			changed := int64(1)
			if failure == "claim_lost" {
				changed = 0
			}
			mock.ExpectExec("UPDATE user_coupons .* WHERE id = .* AND status = 'unused'").WithArgs(int64(7), int64(91), service.UserCouponStatusReserved, sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, changed))
			wantErr := errors.New("discount insert failed")
			if failure != "claim_lost" {
				insert := mock.ExpectQuery("INSERT INTO payment_order_discounts").WithArgs(int64(91), int64(7), int64(3), "coupon", service.CouponScopeBalance, float64(5), float64(10), float64(50), float64(45), service.OrderDiscountStatusReserved, sqlmock.AnyArg())
				if failure == "discount_insert" {
					insert.WillReturnError(wantErr)
					// The legacy best-effort release uses the same failed transaction; it
					// cannot replace rollback or escape through the root pool.
					mock.ExpectExec("UPDATE user_coupons").WillReturnError(errors.New("transaction aborted"))
				} else {
					insert.WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(22, now, now)).RowsWillBeClosed()
				}
			}
			if failure != "" {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			coupons := newRepositoryMarketingTestCouponService(nil, NewUserCouponRepository(db), NewPaymentOrderDiscountRepository(db))
			result, err := coupons.ReserveCouponForOrder(txCtx, 91, service.ApplyPaymentCouponInput{UserID: 42, OrderType: "balance", OrderAmount: 50, UserCouponID: 7})
			switch failure {
			case "claim_lost":
				require.ErrorIs(t, err, service.ErrUserCouponUnavailable)
			case "discount_insert":
				require.ErrorIs(t, err, wantErr)
			default:
				require.NoError(t, err)
				require.Equal(t, float64(45), result.DiscountedAmount)
			}
			if failure != "" {
				require.Nil(t, result)
				require.NoError(t, tx.Rollback())
			} else {
				require.NoError(t, tx.Commit())
			}
		})
	}
}

func TestMarketingRestorePaidCouponUsesTransactionAndConditionalUpdate(t *testing.T) {
	for _, changed := range []int64{0, 1} {
		t.Run(map[int64]string{0: "lost", 1: "restored"}[changed], func(t *testing.T) {
			db, client, mock, ctx := marketingMockDB(t)
			mock.ExpectBegin()
			tx, err := client.Tx(ctx)
			require.NoError(t, err)
			mock.ExpectExec(`UPDATE payment_order_discounts.*WHERE order_id = \$1 AND status = 'released'`).WithArgs(int64(91), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, changed))
			restored, err := NewPaymentOrderDiscountRepository(db).RestoreReservationByOrderID(dbent.NewTxContext(ctx, tx), 91, time.Now())
			require.NoError(t, err)
			require.Equal(t, changed == 1, restored)
			mock.ExpectRollback()
			require.NoError(t, tx.Rollback())
		})
	}
}

func TestMarketingLotteryStateReadDoesNotCreateRows(t *testing.T) {
	db, _, mock, ctx := marketingMockDB(t)
	mock.ExpectQuery("SELECT activity_id, user_id, default_granted").WithArgs(int64(9), int64(42)).WillReturnRows(sqlmock.NewRows([]string{"activity_id", "user_id", "default_granted", "available_draw_times", "total_granted_times", "total_drawn_times", "total_wallet_paid_amount", "created_at", "updated_at"}))
	state, err := NewLotteryUserStateRepository(db).Get(ctx, 9, 42)
	require.NoError(t, err)
	require.Nil(t, state)
}
