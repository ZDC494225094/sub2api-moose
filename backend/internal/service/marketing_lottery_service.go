package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrLotteryActivityNotFound = infraerrors.NotFound("LOTTERY_ACTIVITY_NOT_FOUND", "lottery activity not found")
	ErrLotteryPrizeNotFound    = infraerrors.NotFound("LOTTERY_PRIZE_NOT_FOUND", "lottery prize not found")
)

type realLotteryClock struct{}

func (realLotteryClock) Now() time.Time { return time.Now().UTC() }

type LotteryService struct {
	entClient           *dbent.Client
	activityRepo        LotteryActivityRepository
	prizeRepo           LotteryPrizeRepository
	userStateRepo       LotteryUserStateRepository
	chanceLogRepo       LotteryChanceLogRepository
	drawRecordRepo      LotteryDrawRecordRepository
	consumeProgressRepo LotteryConsumeProgressRepository
	userRepo            UserRepository
	redeemService       *RedeemService
	couponService       *CouponService
	clock               LotteryClock
}

func NewLotteryService(
	entClient *dbent.Client,
	activityRepo LotteryActivityRepository,
	prizeRepo LotteryPrizeRepository,
	userStateRepo LotteryUserStateRepository,
	chanceLogRepo LotteryChanceLogRepository,
	drawRecordRepo LotteryDrawRecordRepository,
	consumeProgressRepo LotteryConsumeProgressRepository,
	userRepo UserRepository,
	redeemService *RedeemService,
	couponService *CouponService,
) *LotteryService {
	return &LotteryService{
		entClient:           entClient,
		activityRepo:        activityRepo,
		prizeRepo:           prizeRepo,
		userStateRepo:       userStateRepo,
		chanceLogRepo:       chanceLogRepo,
		drawRecordRepo:      drawRecordRepo,
		consumeProgressRepo: consumeProgressRepo,
		userRepo:            userRepo,
		redeemService:       redeemService,
		couponService:       couponService,
		clock:               realLotteryClock{},
	}
}

func (s *LotteryService) DeleteActivity(ctx context.Context, id int64) error {
	return s.activityRepo.Delete(ctx, id)
}

func (s *LotteryService) CreateActivity(ctx context.Context, input *CreateLotteryActivityInput) (*LotteryActivity, error) {
	activity := &LotteryActivity{
		Name:                   strings.TrimSpace(input.Name),
		Description:            strings.TrimSpace(input.Description),
		Status:                 normalizeLotteryActivityStatus(input.Status),
		DefaultDrawTimes:       input.DefaultDrawTimes,
		ConsumeThresholdAmount: input.ConsumeThresholdAmount,
		WalletCostPerDraw:      input.WalletCostPerDraw,
		StartsAt:               normalizeLotteryTime(input.StartsAt),
		EndsAt:                 normalizeLotteryTime(input.EndsAt),
		SortOrder:              input.SortOrder,
	}
	if err := validateLotteryActivity(activity); err != nil {
		return nil, err
	}
	if err := s.activityRepo.Create(ctx, activity); err != nil {
		return nil, err
	}
	return activity, nil
}

func (s *LotteryService) UpdateActivity(ctx context.Context, id int64, input *UpdateLotteryActivityInput) (*LotteryActivity, error) {
	activity, err := s.activityRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		return nil, ErrLotteryActivityNotFound
	}
	if input.Name != nil {
		activity.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		activity.Description = strings.TrimSpace(*input.Description)
	}
	if input.Status != nil {
		activity.Status = normalizeLotteryActivityStatus(*input.Status)
	}
	if input.DefaultDrawTimes != nil {
		activity.DefaultDrawTimes = *input.DefaultDrawTimes
	}
	if input.ConsumeThresholdAmount != nil {
		activity.ConsumeThresholdAmount = *input.ConsumeThresholdAmount
	}
	if input.WalletCostPerDraw != nil {
		activity.WalletCostPerDraw = *input.WalletCostPerDraw
	}
	if input.StartsAt != nil {
		activity.StartsAt = normalizeLotteryTime(input.StartsAt)
	}
	if input.EndsAt != nil {
		activity.EndsAt = normalizeLotteryTime(input.EndsAt)
	}
	if input.SortOrder != nil {
		activity.SortOrder = *input.SortOrder
	}
	if err := validateLotteryActivity(activity); err != nil {
		return nil, err
	}
	if err := s.activityRepo.Update(ctx, activity); err != nil {
		return nil, err
	}
	return activity, nil
}

func (s *LotteryService) ListActivities(ctx context.Context, params pagination.PaginationParams, filter LotteryActivityListFilter) ([]LotteryActivity, *pagination.PaginationResult, error) {
	items, result, err := s.activityRepo.List(ctx, params, filter)
	if err != nil {
		return nil, nil, err
	}
	for i := range items {
		prizes, prizeErr := s.prizeRepo.ListByActivity(ctx, items[i].ID)
		if prizeErr != nil {
			return nil, nil, prizeErr
		}
		items[i].Prizes = prizes
	}
	return items, result, nil
}

func (s *LotteryService) GetActiveOverview(ctx context.Context, userID int64) (*LotteryOverview, error) {
	activities, _, err := s.ListActivities(ctx, pagination.PaginationParams{Page: 1, PageSize: 20}, LotteryActivityListFilter{Status: LotteryActivityStatusActive})
	if err != nil {
		return nil, err
	}
	if len(activities) == 0 {
		return &LotteryOverview{}, nil
	}
	activity := &activities[0]
	now := s.clock.Now()
	if err := ensureLotteryActivityOpen(activity, now); err != nil {
		return &LotteryOverview{}, nil
	}
	state, err := s.userStateRepo.GetOrCreate(ctx, activity.ID, userID)
	if err != nil {
		return nil, err
	}
	// Best-effort: grant default chances outside a transaction.
	// Failures here are non-fatal; the Draw transaction will re-attempt atomically.
	_, _ = s.tryGrantDefaultChances(ctx, activity, state, userID, now)
	recent, err := s.drawRecordRepo.ListRecentByActivity(ctx, activity.ID, 20)
	if err != nil {
		return nil, err
	}
	return &LotteryOverview{
		Activity:      activity,
		UserState:     state,
		RecentWinners: recent,
	}, nil
}

func (s *LotteryService) CreatePrize(ctx context.Context, input *CreateLotteryPrizeInput) (*LotteryPrize, error) {
	prize := &LotteryPrize{
		ActivityID:       input.ActivityID,
		Name:             strings.TrimSpace(input.Name),
		PrizeType:        normalizeLotteryPrizeType(input.PrizeType),
		Stock:            input.Stock,
		RemainingStock:   input.Stock,
		BalanceAmount:    input.BalanceAmount,
		CouponTemplateID: input.CouponTemplateID,
		DisplayOrder:     input.DisplayOrder,
		Status:           normalizeLotteryPrizeStatus(input.Status),
	}
	if err := validateLotteryPrize(prize); err != nil {
		return nil, err
	}
	if err := s.prizeRepo.Create(ctx, prize); err != nil {
		return nil, err
	}
	return prize, nil
}

func (s *LotteryService) UpdatePrize(ctx context.Context, id int64, input *UpdateLotteryPrizeInput) (*LotteryPrize, error) {
	prize, err := s.prizeRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if prize == nil {
		return nil, ErrLotteryPrizeNotFound
	}
	previousStock := prize.Stock
	if input.Name != nil {
		prize.Name = strings.TrimSpace(*input.Name)
	}
	if input.PrizeType != nil {
		prize.PrizeType = normalizeLotteryPrizeType(*input.PrizeType)
	}
	if input.Stock != nil {
		prize.Stock = *input.Stock
		if *input.Stock >= previousStock {
			prize.RemainingStock += *input.Stock - previousStock
		}
		if prize.RemainingStock > prize.Stock {
			prize.RemainingStock = prize.Stock
		}
	}
	if input.RemainingStock != nil {
		prize.RemainingStock = *input.RemainingStock
		if prize.RemainingStock > prize.Stock {
			prize.RemainingStock = prize.Stock
		}
	}
	if input.BalanceAmount != nil {
		prize.BalanceAmount = input.BalanceAmount
	}
	if input.CouponTemplateID != nil {
		prize.CouponTemplateID = input.CouponTemplateID
	}
	if input.DisplayOrder != nil {
		prize.DisplayOrder = *input.DisplayOrder
	}
	if input.Status != nil {
		prize.Status = normalizeLotteryPrizeStatus(*input.Status)
	}
	if err := validateLotteryPrize(prize); err != nil {
		return nil, err
	}
	if err := s.prizeRepo.Update(ctx, prize); err != nil {
		return nil, err
	}
	return prize, nil
}

func (s *LotteryService) Draw(ctx context.Context, input LotteryDrawInput) (*LotteryDrawResult, error) {
	if s.entClient == nil {
		return nil, errors.New("lottery ent client is not configured")
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	activity, err := s.activityRepo.GetByIDForUpdate(txCtx, input.ActivityID)
	if err != nil {
		return nil, err
	}
	if activity == nil {
		return nil, ErrLotteryActivityNotFound
	}
	now := s.clock.Now()
	state, err := s.userStateRepo.GetOrCreateForUpdate(txCtx, activity.ID, input.UserID)
	if err != nil {
		return nil, err
	}
	if err := ensureLotteryActivityOpen(activity, now); err != nil {
		return nil, err
	}
	if _, err := s.tryGrantDefaultChances(txCtx, activity, state, input.UserID, now); err != nil {
		return nil, err
	}

	chanceSource := LotteryChanceSourceDefault
	walletAmount := 0.0
	if state.AvailableDrawTimes > 0 {
		state.AvailableDrawTimes--
		state.TotalDrawnTimes++
	} else {
		if !input.UseWallet {
			if activity.DefaultDrawTimes > 0 && !state.DefaultGranted && activity.ConsumeThresholdAmount > 0 {
				return nil, infraerrors.Conflict("LOTTERY_CONSUME_THRESHOLD_UNMET", "lottery consume threshold not met")
			}
			return nil, infraerrors.Conflict("LOTTERY_NO_AVAILABLE_CHANCES", "no available lottery chances")
		}
		if activity.WalletCostPerDraw <= 0 {
			return nil, infraerrors.BadRequest("LOTTERY_WALLET_DISABLED", "wallet payment for lottery is disabled")
		}
		user, err := s.userRepo.GetByID(txCtx, input.UserID)
		if err != nil {
			return nil, err
		}
		if user.Balance < activity.WalletCostPerDraw {
			return nil, infraerrors.BadRequest("LOTTERY_WALLET_BALANCE_INSUFFICIENT", "insufficient wallet balance")
		}
		if err := s.userRepo.UpdateBalance(txCtx, input.UserID, -activity.WalletCostPerDraw); err != nil {
			return nil, err
		}
		state.TotalDrawnTimes++
		state.TotalWalletPaidAmount += activity.WalletCostPerDraw
		chanceSource = LotteryChanceSourceWallet
		walletAmount = activity.WalletCostPerDraw
	}
	if err := s.userStateRepo.Update(txCtx, state); err != nil {
		return nil, err
	}
	if err := s.chanceLogRepo.Create(txCtx, &LotteryChanceLog{
		ActivityID:   activity.ID,
		UserID:       input.UserID,
		ChangeAmount: -1,
		BalanceAfter: state.AvailableDrawTimes,
		SourceType:   chanceSource,
		Notes:        "lottery draw consumed",
	}); err != nil {
		return nil, err
	}

	prizes, err := s.prizeRepo.ListActiveByActivityForUpdate(txCtx, activity.ID)
	if err != nil {
		return nil, err
	}
	prize := pickLotteryPrize(prizes)
	if prize == nil {
		return nil, infraerrors.Conflict("LOTTERY_PRIZE_POOL_EMPTY", "lottery prize pool is empty")
	}
	if err := s.prizeRepo.DecrementStock(txCtx, prize.ID); err != nil {
		return nil, err
	}

	record := &LotteryDrawRecord{
		ActivityID:      activity.ID,
		UserID:          input.UserID,
		PrizeID:         &prize.ID,
		PrizeName:       prize.Name,
		PrizeType:       prize.PrizeType,
		ResultCode:      LotteryDrawResultWin,
		ChanceSource:    chanceSource,
		WalletAmount:    walletAmount,
		RewardReference: "",
	}
	if prize.PrizeType == LotteryPrizeTypeThanks {
		record.ResultCode = LotteryDrawResultThanks
	}

	result := &LotteryDrawResult{
		Activity:  activity,
		Prize:     prize,
		UserState: state,
		Record:    record,
	}
	switch prize.PrizeType {
	case LotteryPrizeTypeBalanceRedeem:
		if prize.BalanceAmount == nil || *prize.BalanceAmount <= 0 {
			return nil, infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "balance prize amount is invalid")
		}
		code, err := s.createBalanceRedeemPrize(txCtx, input.UserID, activity.ID, prize.ID, prize.Name, *prize.BalanceAmount)
		if err != nil {
			return nil, err
		}
		record.RewardReference = code.Code
		result.RedeemCode = code
	case LotteryPrizeTypeCoupon:
		if prize.CouponTemplateID == nil || *prize.CouponTemplateID <= 0 {
			return nil, infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "coupon prize template is invalid")
		}
		sourceRefID := prize.ID
		coupon, err := s.couponService.IssueCouponFromTemplate(txCtx, *prize.CouponTemplateID, input.UserID, UserCouponSourceLottery, &sourceRefID)
		if err != nil {
			return nil, err
		}
		record.UserCouponID = &coupon.ID
		record.RewardReference = coupon.CouponCode
		result.UserCoupon = coupon
	case LotteryPrizeTypeThanks:
	default:
		return nil, infraerrors.BadRequest("LOTTERY_PRIZE_TYPE_UNSUPPORTED", "unsupported lottery prize type")
	}
	if err := s.drawRecordRepo.Create(txCtx, record); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *LotteryService) ListUserDrawRecords(ctx context.Context, userID, activityID int64, params pagination.PaginationParams) ([]LotteryDrawRecord, *pagination.PaginationResult, error) {
	return s.drawRecordRepo.ListByUser(ctx, userID, activityID, params)
}

func ensureLotteryActivityOpen(activity *LotteryActivity, now time.Time) error {
	if activity.Status != LotteryActivityStatusActive {
		return infraerrors.Conflict("LOTTERY_ACTIVITY_INACTIVE", "lottery activity is inactive")
	}
	if activity.StartsAt != nil && now.Before(activity.StartsAt.UTC()) {
		return infraerrors.Conflict("LOTTERY_ACTIVITY_NOT_STARTED", "lottery activity has not started")
	}
	if activity.EndsAt != nil && now.After(activity.EndsAt.UTC()) {
		return infraerrors.Conflict("LOTTERY_ACTIVITY_ENDED", "lottery activity has ended")
	}
	return nil
}

func (s *LotteryService) tryGrantDefaultChances(ctx context.Context, activity *LotteryActivity, state *LotteryUserState, userID int64, now time.Time) (bool, error) {
	if state.DefaultGranted || activity.DefaultDrawTimes <= 0 {
		return false, nil
	}
	if activity.ConsumeThresholdAmount > 0 {
		qualifiedAmount, err := s.consumeProgressRepo.GetQualifiedAmount(ctx, activity.ID, userID, activity.ConsumeThresholdAmount)
		if err != nil {
			return false, err
		}
		if qualifiedAmount < activity.ConsumeThresholdAmount {
			return false, nil
		}
	}
	state.DefaultGranted = true
	state.AvailableDrawTimes += activity.DefaultDrawTimes
	state.TotalGrantedTimes += activity.DefaultDrawTimes
	if err := s.userStateRepo.Update(ctx, state); err != nil {
		return false, err
	}
	if err := s.chanceLogRepo.Create(ctx, &LotteryChanceLog{
		ActivityID:   activity.ID,
		UserID:       state.UserID,
		ChangeAmount: activity.DefaultDrawTimes,
		BalanceAfter: state.AvailableDrawTimes,
		SourceType:   LotteryChanceSourceDefault,
		Notes:        "default lottery chances granted",
		CreatedAt:    now,
	}); err != nil {
		return false, err
	}
	return true, nil
}

func (s *LotteryService) createBalanceRedeemPrize(ctx context.Context, userID, activityID, prizeID int64, prizeName string, amount float64) (*RedeemCode, error) {
	if s.redeemService == nil {
		return nil, errors.New("redeem service is not configured")
	}
	code, err := s.redeemService.GenerateRandomCode()
	if err != nil {
		return nil, err
	}
	redeemCode := &RedeemCode{
		Code:   code,
		Type:   RedeemTypeBalance,
		Value:  amount,
		Status: StatusUnused,
		Notes:  fmt.Sprintf("lottery activity %d prize %d (%s)", activityID, prizeID, prizeName),
	}
	if err := s.redeemService.CreateCode(ctx, redeemCode); err != nil {
		return nil, err
	}
	return redeemCode, nil
}

func validateLotteryActivity(activity *LotteryActivity) error {
	if activity == nil {
		return infraerrors.BadRequest("LOTTERY_ACTIVITY_INVALID", "lottery activity is required")
	}
	if strings.TrimSpace(activity.Name) == "" {
		return infraerrors.BadRequest("LOTTERY_ACTIVITY_NAME_REQUIRED", "lottery activity name is required")
	}
	if activity.DefaultDrawTimes < 0 {
		return infraerrors.BadRequest("LOTTERY_ACTIVITY_DEFAULT_TIMES_INVALID", "default draw times must be greater than or equal to zero")
	}
	if activity.ConsumeThresholdAmount < 0 {
		return infraerrors.BadRequest("LOTTERY_ACTIVITY_CONSUME_THRESHOLD_INVALID", "consume threshold must be greater than or equal to zero")
	}
	if activity.WalletCostPerDraw < 0 {
		return infraerrors.BadRequest("LOTTERY_ACTIVITY_WALLET_COST_INVALID", "wallet cost per draw must be greater than or equal to zero")
	}
	switch activity.Status {
	case LotteryActivityStatusDraft, LotteryActivityStatusActive, LotteryActivityStatusInactive, LotteryActivityStatusEnded:
	default:
		return infraerrors.BadRequest("LOTTERY_ACTIVITY_STATUS_INVALID", "lottery activity status is invalid")
	}
	if activity.StartsAt != nil && activity.EndsAt != nil && activity.EndsAt.Before(activity.StartsAt.UTC()) {
		return infraerrors.BadRequest("LOTTERY_ACTIVITY_TIME_RANGE_INVALID", "activity end time must be after start time")
	}
	return nil
}

func validateLotteryPrize(prize *LotteryPrize) error {
	if prize == nil {
		return infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "lottery prize is required")
	}
	if prize.ActivityID <= 0 {
		return infraerrors.BadRequest("LOTTERY_PRIZE_ACTIVITY_REQUIRED", "lottery activity is required")
	}
	if strings.TrimSpace(prize.Name) == "" {
		return infraerrors.BadRequest("LOTTERY_PRIZE_NAME_REQUIRED", "lottery prize name is required")
	}
	switch prize.PrizeType {
	case LotteryPrizeTypeBalanceRedeem, LotteryPrizeTypeCoupon, LotteryPrizeTypeThanks:
	default:
		return infraerrors.BadRequest("LOTTERY_PRIZE_TYPE_INVALID", "lottery prize type is invalid")
	}
	switch prize.Status {
	case LotteryPrizeStatusActive, LotteryPrizeStatusInactive:
	default:
		return infraerrors.BadRequest("LOTTERY_PRIZE_STATUS_INVALID", "lottery prize status is invalid")
	}
	if prize.Stock < 0 || prize.RemainingStock < 0 || prize.RemainingStock > prize.Stock {
		return infraerrors.BadRequest("LOTTERY_PRIZE_STOCK_INVALID", "lottery prize stock is invalid")
	}
	if prize.PrizeType == LotteryPrizeTypeBalanceRedeem {
		if prize.BalanceAmount == nil || *prize.BalanceAmount <= 0 {
			return infraerrors.BadRequest("LOTTERY_PRIZE_BALANCE_AMOUNT_INVALID", "balance prize amount must be greater than zero")
		}
	}
	if prize.PrizeType == LotteryPrizeTypeCoupon {
		if prize.CouponTemplateID == nil || *prize.CouponTemplateID <= 0 {
			return infraerrors.BadRequest("LOTTERY_PRIZE_COUPON_TEMPLATE_REQUIRED", "coupon template is required for coupon prize")
		}
	}
	return nil
}

func normalizeLotteryActivityStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", LotteryActivityStatusDraft:
		return LotteryActivityStatusDraft
	case LotteryActivityStatusActive:
		return LotteryActivityStatusActive
	case LotteryActivityStatusInactive:
		return LotteryActivityStatusInactive
	case LotteryActivityStatusEnded:
		return LotteryActivityStatusEnded
	default:
		return strings.TrimSpace(status)
	}
}

func normalizeLotteryPrizeType(prizeType string) string {
	return strings.TrimSpace(prizeType)
}

func normalizeLotteryPrizeStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", LotteryPrizeStatusActive:
		return LotteryPrizeStatusActive
	case LotteryPrizeStatusInactive:
		return LotteryPrizeStatusInactive
	default:
		return strings.TrimSpace(status)
	}
}

func normalizeLotteryTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	t := v.UTC()
	return &t
}

func pickLotteryPrize(prizes []LotteryPrize) *LotteryPrize {
	weighted := make([]LotteryPrize, 0, len(prizes))
	for _, prize := range prizes {
		if prize.Status != LotteryPrizeStatusActive {
			continue
		}
		if prize.RemainingStock > 0 {
			for i := 0; i < prize.RemainingStock; i++ {
				weighted = append(weighted, prize)
			}
		}
	}
	if len(weighted) == 0 {
		return nil
	}
	selected := weighted[rand.Intn(len(weighted))]
	return &selected
}
