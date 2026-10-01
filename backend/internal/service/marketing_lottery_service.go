// Host adapters for marketing. No lottery eligibility, stock, coupon or prize
// selection rules belong here; the host owns Ent transactions and native rewards.
package service

import (
	"context"
	"errors"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
)

var (
	ErrLotteryActivityNotFound = marketing.ErrLotteryActivityNotFound
	ErrLotteryPrizeNotFound    = marketing.ErrLotteryPrizeNotFound
)

type LotteryService = marketing.LotteryService[RedeemCode]

func NewLotteryService(
	client *dbent.Client,
	activities LotteryActivityRepository,
	prizes LotteryPrizeRepository,
	states LotteryUserStateRepository,
	chances LotteryChanceLogRepository,
	records LotteryDrawRecordRepository,
	progress LotteryConsumeProgressRepository,
	users UserRepository,
	redeem *RedeemService,
	coupons *CouponService,
) *LotteryService {
	return marketing.NewLotteryService[RedeemCode](
		lotteryEntTransactions{client}, activities, prizes, states, chances, records,
		progress, lotteryHostWallet{users}, lotteryHostRedeemIssuer{redeem}, coupons,
	)
}

func NewLotteryServiceWithAdmission(
	client *dbent.Client,
	activities LotteryActivityRepository,
	prizes LotteryPrizeRepository,
	states LotteryUserStateRepository,
	chances LotteryChanceLogRepository,
	records LotteryDrawRecordRepository,
	progress LotteryConsumeProgressRepository,
	users UserRepository,
	redeem *RedeemService,
	coupons *CouponService,
	admission marketing.NewBusinessAdmission,
) *LotteryService {
	return marketing.NewLotteryServiceWithAdmission[RedeemCode](
		lotteryEntTransactions{client}, activities, prizes, states, chances, records,
		progress, lotteryHostWallet{users}, lotteryHostRedeemIssuer{redeem}, coupons, admission,
	)
}

type lotteryEntTransactions struct{ client *dbent.Client }

func (h lotteryEntTransactions) WithinLotteryTransaction(ctx context.Context, work func(context.Context) error) error {
	if h.client == nil {
		return errors.New("lottery ent client is not configured")
	}
	tx, err := h.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := work(dbent.NewTxContext(ctx, tx)); err != nil {
		return err
	}
	return tx.Commit()
}

type lotteryHostWallet struct{ users UserRepository }

func (h lotteryHostWallet) GetBalance(ctx context.Context, userID int64) (float64, error) {
	// Only balance is needed. The general user repository also loads profile and
	// group data through root connections; do not escape the draw transaction.
	if tx := dbent.TxFromContext(ctx); tx != nil {
		user, err := tx.Client().User.Query().Where(dbuser.IDEQ(userID)).Select(dbuser.FieldBalance).Only(ctx)
		if dbent.IsNotFound(err) {
			return 0, ErrUserNotFound
		}
		if err != nil {
			return 0, err
		}
		return user.Balance, nil
	}
	user, err := h.users.GetByID(ctx, userID)
	if err != nil {
		return 0, err
	}
	return user.Balance, nil
}
func (h lotteryHostWallet) UpdateBalance(ctx context.Context, userID int64, delta float64) error {
	if tx := dbent.TxFromContext(ctx); tx != nil && delta < 0 {
		// The earlier balance read is not a debit claim. Concurrent draws in distinct
		// activities must not both spend the same remaining wallet balance.
		n, err := tx.Client().User.Update().Where(dbuser.IDEQ(userID), dbuser.BalanceGTE(-delta)).AddBalance(delta).Save(ctx)
		if err != nil {
			return err
		}
		if n == 0 {
			return marketing.ErrLotteryWalletBalanceInsufficient
		}
		return nil
	}
	return h.users.UpdateBalance(ctx, userID, delta)
}

type lotteryHostRedeemIssuer struct{ redeem *RedeemService }

func (h lotteryHostRedeemIssuer) IssueBalanceRedeem(ctx context.Context, input marketing.BalanceRedeemInput) (string, *RedeemCode, error) {
	if h.redeem == nil {
		return "", nil, errors.New("redeem service is not configured")
	}
	code, err := h.redeem.GenerateRandomCode()
	if err != nil {
		return "", nil, err
	}
	reward := &RedeemCode{Code: code, Type: RedeemTypeBalance, Value: input.Amount, Status: StatusUnused, Notes: input.Notes}
	if err := h.redeem.CreateCode(ctx, reward); err != nil {
		return "", nil, err
	}
	return code, reward, nil
}
