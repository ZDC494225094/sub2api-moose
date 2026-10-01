// Compatibility aliases keep the existing host DTOs and repository adapters source-compatible.
// Marketing rules and data contracts are owned by the independent extension module.
package service

import "github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"

const (
	LotteryActivityStatusDraft    = marketing.LotteryActivityStatusDraft
	LotteryActivityStatusActive   = marketing.LotteryActivityStatusActive
	LotteryActivityStatusInactive = marketing.LotteryActivityStatusInactive
	LotteryActivityStatusEnded    = marketing.LotteryActivityStatusEnded
	LotteryPrizeTypeBalanceRedeem = marketing.LotteryPrizeTypeBalanceRedeem
	LotteryPrizeTypeCoupon        = marketing.LotteryPrizeTypeCoupon
	LotteryPrizeTypeThanks        = marketing.LotteryPrizeTypeThanks
	LotteryPrizeStatusActive      = marketing.LotteryPrizeStatusActive
	LotteryPrizeStatusInactive    = marketing.LotteryPrizeStatusInactive
	LotteryChanceSourceDefault    = marketing.LotteryChanceSourceDefault
	LotteryChanceSourceWallet     = marketing.LotteryChanceSourceWallet
	LotteryDrawResultWin          = marketing.LotteryDrawResultWin
	LotteryDrawResultThanks       = marketing.LotteryDrawResultThanks
)

type LotteryActivity = marketing.LotteryActivity
type LotteryPrize = marketing.LotteryPrize
type LotteryUserState = marketing.LotteryUserState
type LotteryChanceLog = marketing.LotteryChanceLog
type LotteryDrawRecord = marketing.LotteryDrawRecord
type LotteryOverview = marketing.LotteryOverview
type LotteryDrawEligibility = marketing.LotteryDrawEligibility
type CreateLotteryActivityInput = marketing.CreateLotteryActivityInput
type UpdateLotteryActivityInput = marketing.UpdateLotteryActivityInput
type LotteryActivityListFilter = marketing.LotteryActivityListFilter
type CreateLotteryPrizeInput = marketing.CreateLotteryPrizeInput
type UpdateLotteryPrizeInput = marketing.UpdateLotteryPrizeInput
type LotteryDrawInput = marketing.LotteryDrawInput
type LotteryDrawResult = marketing.LotteryDrawResult[RedeemCode]
