// Compatibility aliases keep the existing host DTOs and repository adapters source-compatible.
// Marketing rules and data contracts are owned by the independent extension module.
package service

import "github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"

type LotteryActivityRepository = marketing.LotteryActivityRepository
type LotteryPrizeRepository = marketing.LotteryPrizeRepository
type LotteryUserStateRepository = marketing.LotteryUserStateRepository
type LotteryChanceLogRepository = marketing.LotteryChanceLogRepository
type LotteryDrawRecordRepository = marketing.LotteryDrawRecordRepository
type LotteryConsumeProgressRepository = marketing.LotteryConsumeProgressRepository
type LotteryClock = marketing.LotteryClock
