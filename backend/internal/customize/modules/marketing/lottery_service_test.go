package marketing

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type lotteryRewardPayload struct {
	Code        string
	Value       float64
	NativeExtra string
}
type lotteryTxKey struct{}
type lotteryFixture struct {
	t                         *testing.T
	service                   *LotteryService[lotteryRewardPayload]
	activity                  *LotteryActivity
	state                     *LotteryUserState
	prize                     LotteryPrize
	record                    *LotteryDrawRecord
	reward                    *lotteryRewardPayload
	ctx                       context.Context
	steps                     []string
	failure                   string
	err                       error
	commits, rollbacks        int
	balance, delta, qualified float64
	issuerInput               BalanceRedeemInput
}

func (f *lotteryFixture) step(ctx context.Context, name string) error {
	f.t.Helper()
	require.Equal(f.t, f.ctx, ctx, "every draw operation must share the transaction context")
	require.Equal(f.t, "one transaction", ctx.Value(lotteryTxKey{}))
	f.steps = append(f.steps, name)
	if f.failure == name {
		return f.err
	}
	return nil
}
func (f *lotteryFixture) WithinLotteryTransaction(ctx context.Context, work func(context.Context) error) error {
	f.ctx = context.WithValue(ctx, lotteryTxKey{}, "one transaction")
	if err := f.step(f.ctx, "begin"); err != nil {
		return err
	}
	if err := work(f.ctx); err != nil {
		f.rollbacks++
		return err
	}
	if err := f.step(f.ctx, "commit"); err != nil {
		f.rollbacks++
		return err
	}
	f.commits++
	return nil
}

type lotteryActivities struct {
	LotteryActivityRepository
	f *lotteryFixture
}

func (r lotteryActivities) GetByIDForUpdate(ctx context.Context, id int64) (*LotteryActivity, error) {
	if err := r.f.step(ctx, "activity"); err != nil {
		return nil, err
	}
	return r.f.activity, nil
}

type lotteryStates struct {
	LotteryUserStateRepository
	f *lotteryFixture
}

func (r lotteryStates) GetOrCreateForUpdate(ctx context.Context, activity, user int64) (*LotteryUserState, error) {
	if err := r.f.step(ctx, "state"); err != nil {
		return nil, err
	}
	return r.f.state, nil
}
func (r lotteryStates) Update(ctx context.Context, state *LotteryUserState) error {
	return r.f.step(ctx, "state-update")
}

type lotteryPrizes struct {
	LotteryPrizeRepository
	f *lotteryFixture
}

func (r lotteryPrizes) ListActiveByActivityForUpdate(ctx context.Context, id int64) ([]LotteryPrize, error) {
	if err := r.f.step(ctx, "prizes"); err != nil {
		return nil, err
	}
	return []LotteryPrize{r.f.prize}, nil
}
func (r lotteryPrizes) DecrementStock(ctx context.Context, id int64) error {
	return r.f.step(ctx, "stock")
}

type lotteryChances struct {
	LotteryChanceLogRepository
	f *lotteryFixture
}

func (r lotteryChances) Create(ctx context.Context, log *LotteryChanceLog) error {
	return r.f.step(ctx, "chance-log")
}

type lotteryRecords struct {
	LotteryDrawRecordRepository
	f *lotteryFixture
}

func (r lotteryRecords) Create(ctx context.Context, record *LotteryDrawRecord) error {
	r.f.record = record
	return r.f.step(ctx, "record")
}

type lotteryProgress struct {
	LotteryConsumeProgressRepository
	f *lotteryFixture
}

func (r lotteryProgress) GetQualifiedAmount(ctx context.Context, activity, user int64, threshold float64) (float64, error) {
	return r.f.qualified, r.f.step(ctx, "progress")
}
func (f *lotteryFixture) GetBalance(ctx context.Context, user int64) (float64, error) {
	return f.balance, f.step(ctx, "wallet")
}
func (f *lotteryFixture) UpdateBalance(ctx context.Context, user int64, delta float64) error {
	f.delta += delta
	return f.step(ctx, "debit")
}
func (f *lotteryFixture) IssueBalanceRedeem(ctx context.Context, input BalanceRedeemInput) (string, *lotteryRewardPayload, error) {
	f.issuerInput = input
	if err := f.step(ctx, "reward"); err != nil {
		return "", nil, err
	}
	return f.reward.Code, f.reward, nil
}
func newLotteryFixture(t *testing.T) *lotteryFixture {
	t.Helper()
	amount := float64(5)
	f := &lotteryFixture{t: t, activity: &LotteryActivity{ID: 9, Name: "sale", Status: LotteryActivityStatusActive, WalletCostPerDraw: 2}, state: &LotteryUserState{ActivityID: 9, UserID: 42, DefaultGranted: true, AvailableDrawTimes: 1}, prize: LotteryPrize{ID: 11, ActivityID: 9, Name: "five", PrizeType: LotteryPrizeTypeBalanceRedeem, Stock: 1, RemainingStock: 1, BalanceAmount: &amount, Status: LotteryPrizeStatusActive}, balance: 20, qualified: 20, err: errors.New("injected repository failure"), reward: &lotteryRewardPayload{Code: "NATIVE-CODE", Value: 5, NativeExtra: "unchanged"}}
	f.service = NewLotteryServiceWithAdmission[lotteryRewardPayload](f, lotteryActivities{f: f}, lotteryPrizes{f: f}, lotteryStates{f: f}, lotteryChances{f: f}, lotteryRecords{f: f}, lotteryProgress{f: f}, f, f, nil, admissionFunc(func(context.Context) error { return nil }))
	return f
}
func (f *lotteryFixture) draw() (*LotteryDrawResult[lotteryRewardPayload], error) {
	return f.service.Draw(context.Background(), LotteryDrawInput{UserID: 42, ActivityID: 9, UseWallet: true})
}

func TestLotteryDrawUsesOneTransactionAndUnmodifiedHostReward(t *testing.T) {
	f := newLotteryFixture(t)
	result, err := f.draw()
	require.NoError(t, err)
	require.Same(t, f.reward, result.RedeemCode)
	require.Equal(t, "NATIVE-CODE", result.Record.RewardReference)
	require.Equal(t, LotteryChanceSourceDefault, result.Record.ChanceSource)
	require.Equal(t, float64(0), f.delta)
	require.Equal(t, 0, result.UserState.AvailableDrawTimes)
	require.Equal(t, 1, result.UserState.TotalDrawnTimes)
	require.Equal(t, 1, f.commits)
	require.Zero(t, f.rollbacks)
	require.Equal(t, BalanceRedeemInput{UserID: 42, Amount: 5, Notes: "lottery activity 9 prize 11 (five)"}, f.issuerInput)
	require.Equal(t, []string{"begin", "activity", "state", "state-update", "chance-log", "prizes", "stock", "reward", "record", "commit"}, f.steps)
}

func TestLotteryWalletDrawPreservesCostAndAuditSnapshot(t *testing.T) {
	f := newLotteryFixture(t)
	f.state.AvailableDrawTimes = 0
	result, err := f.draw()
	require.NoError(t, err)
	require.Equal(t, float64(-2), f.delta)
	require.Equal(t, float64(2), result.UserState.TotalWalletPaidAmount)
	require.Equal(t, float64(2), result.Record.WalletAmount)
	require.Equal(t, LotteryChanceSourceWallet, result.Record.ChanceSource)
	require.Equal(t, 1, f.commits)
}

func TestLotteryDrawErrorNeverReturnsRewardOrCommits(t *testing.T) {
	// This asserts the module/transaction-port protocol, not PostgreSQL locking.
	for _, step := range []string{"begin", "activity", "state", "progress", "wallet", "debit", "state-update", "chance-log", "prizes", "stock", "reward", "record", "commit"} {
		t.Run(step, func(t *testing.T) {
			f := newLotteryFixture(t)
			f.failure = step
			f.state.AvailableDrawTimes = 0
			f.activity.ConsumeThresholdAmount = 10
			result, err := f.draw()
			require.ErrorIs(t, err, f.err)
			require.Nil(t, result)
			require.Zero(t, f.commits)
			if step == "begin" {
				require.Zero(t, f.rollbacks)
			} else {
				require.Equal(t, 1, f.rollbacks)
			}
			require.Equal(t, step, f.steps[len(f.steps)-1], "no later mutation may run after the first failure")
		})
	}
}

func TestLotteryEligibilityAndStockRejectionsPrecedeReward(t *testing.T) {
	cases := []struct {
		name string
		edit func(*lotteryFixture)
		want string
	}{
		{"inactive", func(f *lotteryFixture) { f.activity.Status = LotteryActivityStatusInactive }, "inactive"},
		{"not started", func(f *lotteryFixture) { v := time.Now().Add(time.Hour); f.activity.StartsAt = &v }, "not started"},
		{"ended", func(f *lotteryFixture) { v := time.Now().Add(-time.Hour); f.activity.EndsAt = &v }, "ended"},
		{"threshold", func(f *lotteryFixture) { f.activity.ConsumeThresholdAmount = 50 }, "threshold"},
		{"wallet disabled", func(f *lotteryFixture) { f.activity.WalletCostPerDraw = 0; f.state.AvailableDrawTimes = 0 }, "disabled"},
		{"insufficient", func(f *lotteryFixture) { f.balance = 1; f.state.AvailableDrawTimes = 0 }, "insufficient"},
		{"no stock", func(f *lotteryFixture) { f.prize.RemainingStock = 0 }, "empty"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := newLotteryFixture(t)
			tt.edit(f)
			result, err := f.draw()
			require.ErrorContains(t, err, tt.want)
			require.Nil(t, result)
			require.Zero(t, f.commits)
			require.Equal(t, 1, f.rollbacks)
			require.NotContains(t, f.steps, "reward")
			require.NotContains(t, f.steps, "record")
		})
	}
}

func TestLotteryGrantsDefaultChancesOnceInsideDrawTransaction(t *testing.T) {
	f := newLotteryFixture(t)
	f.state.DefaultGranted = false
	f.state.AvailableDrawTimes = 0
	f.activity.DefaultDrawTimes = 3
	f.activity.ConsumeThresholdAmount = 10
	result, err := f.draw()
	require.NoError(t, err)
	require.True(t, result.UserState.DefaultGranted)
	require.Equal(t, 2, result.UserState.AvailableDrawTimes)
	require.Equal(t, 3, result.UserState.TotalGrantedTimes)
	require.Zero(t, f.delta)
	_, err = f.draw()
	require.NoError(t, err)
	require.Equal(t, 1, f.state.AvailableDrawTimes)
	require.Equal(t, 3, f.state.TotalGrantedTimes)
}

func TestLotteryThanksPrizeDoesNotIssueReward(t *testing.T) {
	f := newLotteryFixture(t)
	f.prize.PrizeType = LotteryPrizeTypeThanks
	result, err := f.draw()
	require.NoError(t, err)
	require.Nil(t, result.RedeemCode)
	require.Nil(t, result.UserCoupon)
	require.Equal(t, LotteryDrawResultThanks, result.Record.ResultCode)
	require.Empty(t, result.Record.RewardReference)
	require.NotContains(t, f.steps, "reward")
}

type lotteryCouponTemplates struct {
	CouponTemplateRepository
	f *lotteryFixture
}

func (r lotteryCouponTemplates) GetByID(ctx context.Context, id int64) (*CouponTemplate, error) {
	if err := r.f.step(ctx, "coupon-template"); err != nil {
		return nil, err
	}
	return &CouponTemplate{ID: id, Name: "prize coupon", Scope: CouponScopeBalance, DiscountAmount: 5, Status: CouponTemplateStatusActive}, nil
}

type lotteryCouponRows struct {
	UserCouponRepository
	f *lotteryFixture
}

func (r lotteryCouponRows) Create(ctx context.Context, coupon *UserCoupon) error {
	coupon.ID = 88
	return r.f.step(ctx, "coupon-issue")
}
func TestLotteryCouponIssueSharesDrawTransactionAndFailureProtocol(t *testing.T) {
	for _, failure := range []string{"", "coupon-template", "coupon-issue", "record"} {
		t.Run("failure="+failure, func(t *testing.T) {
			f := newLotteryFixture(t)
			f.failure = failure
			id := int64(12)
			f.prize.PrizeType = LotteryPrizeTypeCoupon
			f.prize.CouponTemplateID = &id
			f.service.couponService = newTestCouponService(lotteryCouponTemplates{f: f}, lotteryCouponRows{f: f}, nil)
			result, err := f.draw()
			if failure != "" {
				require.ErrorIs(t, err, f.err)
				require.Nil(t, result)
				require.Zero(t, f.commits)
				require.Equal(t, 1, f.rollbacks)
				return
			}
			require.NoError(t, err)
			require.Nil(t, result.RedeemCode)
			require.Equal(t, int64(88), *result.Record.UserCouponID)
			require.Equal(t, result.UserCoupon.CouponCode, result.Record.RewardReference)
			require.Equal(t, f.prize.ID, *result.UserCoupon.SourceRefID)
			require.Equal(t, UserCouponSourceLottery, result.UserCoupon.SourceType)
			require.Equal(t, 1, f.commits)
		})
	}
}

// Only read methods are implemented. Any hidden insert/grant/transaction panics.
type overviewActivities struct {
	LotteryActivityRepository
	activity LotteryActivity
}

func (r overviewActivities) List(context.Context, pagination.PaginationParams, LotteryActivityListFilter) ([]LotteryActivity, *pagination.PaginationResult, error) {
	return []LotteryActivity{r.activity}, nil, nil
}

type overviewPrizes struct{ LotteryPrizeRepository }

func (overviewPrizes) ListByActivity(context.Context, int64) ([]LotteryPrize, error) { return nil, nil }

type overviewStates struct {
	LotteryUserStateRepository
	state *LotteryUserState
}

func (r overviewStates) Get(context.Context, int64, int64) (*LotteryUserState, error) {
	return r.state, nil
}

type overviewRecords struct{ LotteryDrawRecordRepository }

func (overviewRecords) ListRecentByActivity(context.Context, int64, int) ([]LotteryDrawRecord, error) {
	return nil, nil
}

type overviewProgress struct {
	LotteryConsumeProgressRepository
	amount float64
}

func (r overviewProgress) GetQualifiedAmount(context.Context, int64, int64, float64) (float64, error) {
	return r.amount, nil
}
func TestLotteryOverviewIsReadOnlyAndPreviewsOnlyEligibleDefaultChances(t *testing.T) {
	for _, mode := range []string{"new", "existing", "already_granted", "threshold_unmet"} {
		t.Run(mode, func(t *testing.T) {
			activity := LotteryActivity{ID: 9, Status: LotteryActivityStatusActive, DefaultDrawTimes: 3, ConsumeThresholdAmount: 10}
			var state *LotteryUserState
			if mode != "new" {
				state = &LotteryUserState{ActivityID: 9, UserID: 42, DefaultGranted: mode == "already_granted"}
			}
			amount := float64(10)
			if mode == "threshold_unmet" {
				amount = 5
			}
			svc := NewLotteryService[int](nil, overviewActivities{activity: activity}, overviewPrizes{}, overviewStates{state: state}, nil, overviewRecords{}, overviewProgress{amount: amount}, nil, nil, nil)
			for i := 0; i < 2; i++ {
				result, err := svc.GetActiveOverview(context.Background(), 42)
				require.NoError(t, err)
				require.Zero(t, result.UserState.AvailableDrawTimes)
				require.Equal(t, mode == "already_granted", result.UserState.DefaultGranted)
				pending := 0
				if mode == "new" || mode == "existing" {
					pending = 3
				}
				require.Equal(t, pending, result.DrawEligibility.PendingDefaultDrawTimes)
				if pending > 0 {
					require.Empty(t, result.DrawEligibility.BlockReason)
				}
			}
		})
	}
}
