package service

import "time"

const (
	LotteryActivityStatusDraft    = "draft"
	LotteryActivityStatusActive   = "active"
	LotteryActivityStatusInactive = "inactive"
	LotteryActivityStatusEnded    = "ended"
)

const (
	LotteryPrizeTypeBalanceRedeem = "balance_redeem"
	LotteryPrizeTypeCoupon        = "coupon"
	LotteryPrizeTypeThanks        = "thanks"
)

const (
	LotteryPrizeStatusActive   = "active"
	LotteryPrizeStatusInactive = "inactive"
)

const (
	LotteryChanceSourceDefault = "default"
	LotteryChanceSourceWallet  = "wallet"
)

const (
	LotteryDrawResultWin    = "win"
	LotteryDrawResultThanks = "thanks"
)

type LotteryActivity struct {
	ID                     int64          `json:"id"`
	Name                   string         `json:"name"`
	Description            string         `json:"description"`
	Status                 string         `json:"status"`
	DefaultDrawTimes       int            `json:"default_draw_times"`
	ConsumeThresholdAmount float64        `json:"consume_threshold_amount"`
	WalletCostPerDraw      float64        `json:"wallet_cost_per_draw"`
	StartsAt               *time.Time     `json:"starts_at,omitempty"`
	EndsAt                 *time.Time     `json:"ends_at,omitempty"`
	SortOrder              int            `json:"sort_order"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	Prizes                 []LotteryPrize `json:"prizes,omitempty"`
}

type LotteryPrize struct {
	ID               int64           `json:"id"`
	ActivityID       int64           `json:"activity_id"`
	Name             string          `json:"name"`
	PrizeType        string          `json:"prize_type"`
	Stock            int             `json:"stock"`
	RemainingStock   int             `json:"remaining_stock"`
	BalanceAmount    *float64        `json:"balance_amount,omitempty"`
	CouponTemplateID *int64          `json:"coupon_template_id,omitempty"`
	DisplayOrder     int             `json:"display_order"`
	Status           string          `json:"status"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CouponTemplate   *CouponTemplate `json:"coupon_template,omitempty"`
}

type LotteryUserState struct {
	ActivityID            int64     `json:"activity_id"`
	UserID                int64     `json:"user_id"`
	DefaultGranted        bool      `json:"default_granted"`
	AvailableDrawTimes    int       `json:"available_draw_times"`
	TotalGrantedTimes     int       `json:"total_granted_times"`
	TotalDrawnTimes       int       `json:"total_drawn_times"`
	TotalWalletPaidAmount float64   `json:"total_wallet_paid_amount"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

type LotteryChanceLog struct {
	ID           int64     `json:"id"`
	ActivityID   int64     `json:"activity_id"`
	UserID       int64     `json:"user_id"`
	ChangeAmount int       `json:"change_amount"`
	BalanceAfter int       `json:"balance_after"`
	SourceType   string    `json:"source_type"`
	SourceRefID  *int64    `json:"source_ref_id,omitempty"`
	Notes        string    `json:"notes"`
	CreatedAt    time.Time `json:"created_at"`
}

type LotteryDrawRecord struct {
	ID              int64     `json:"id"`
	ActivityID      int64     `json:"activity_id"`
	UserID          int64     `json:"user_id"`
	PrizeID         *int64    `json:"prize_id,omitempty"`
	PrizeName       string    `json:"prize_name"`
	PrizeType       string    `json:"prize_type"`
	ResultCode      string    `json:"result_code"`
	ChanceSource    string    `json:"chance_source"`
	WalletAmount    float64   `json:"wallet_amount"`
	UserCouponID    *int64    `json:"user_coupon_id,omitempty"`
	RewardReference string    `json:"reward_reference"`
	CreatedAt       time.Time `json:"created_at"`
	UserName        string    `json:"user_name,omitempty"`
	UserEmail       string    `json:"user_email,omitempty"`
}

type LotteryOverview struct {
	Activity      *LotteryActivity    `json:"activity,omitempty"`
	UserState     *LotteryUserState   `json:"user_state,omitempty"`
	RecentWinners []LotteryDrawRecord `json:"recent_winners,omitempty"`
}

type CreateLotteryActivityInput struct {
	Name                   string     `json:"name"`
	Description            string     `json:"description"`
	Status                 string     `json:"status"`
	DefaultDrawTimes       int        `json:"default_draw_times"`
	ConsumeThresholdAmount float64    `json:"consume_threshold_amount"`
	WalletCostPerDraw      float64    `json:"wallet_cost_per_draw"`
	StartsAt               *time.Time `json:"starts_at,omitempty"`
	EndsAt                 *time.Time `json:"ends_at,omitempty"`
	SortOrder              int        `json:"sort_order"`
}

type UpdateLotteryActivityInput struct {
	Name                   *string    `json:"name,omitempty"`
	Description            *string    `json:"description,omitempty"`
	Status                 *string    `json:"status,omitempty"`
	DefaultDrawTimes       *int       `json:"default_draw_times,omitempty"`
	ConsumeThresholdAmount *float64   `json:"consume_threshold_amount,omitempty"`
	WalletCostPerDraw      *float64   `json:"wallet_cost_per_draw,omitempty"`
	StartsAt               *time.Time `json:"starts_at,omitempty"`
	EndsAt                 *time.Time `json:"ends_at,omitempty"`
	SortOrder              *int       `json:"sort_order,omitempty"`
}

type LotteryActivityListFilter struct {
	Status string
	Search string
}

type CreateLotteryPrizeInput struct {
	ActivityID       int64    `json:"activity_id"`
	Name             string   `json:"name"`
	PrizeType        string   `json:"prize_type"`
	Stock            int      `json:"stock"`
	BalanceAmount    *float64 `json:"balance_amount,omitempty"`
	CouponTemplateID *int64   `json:"coupon_template_id,omitempty"`
	DisplayOrder     int      `json:"display_order"`
	Status           string   `json:"status"`
}

type UpdateLotteryPrizeInput struct {
	Name             *string  `json:"name,omitempty"`
	PrizeType        *string  `json:"prize_type,omitempty"`
	Stock            *int     `json:"stock,omitempty"`
	RemainingStock   *int     `json:"remaining_stock,omitempty"`
	BalanceAmount    *float64 `json:"balance_amount,omitempty"`
	CouponTemplateID *int64   `json:"coupon_template_id,omitempty"`
	DisplayOrder     *int     `json:"display_order,omitempty"`
	Status           *string  `json:"status,omitempty"`
}

type LotteryDrawInput struct {
	ActivityID int64
	UserID     int64
	UseWallet  bool
}

type LotteryDrawResult struct {
	Activity   *LotteryActivity   `json:"activity,omitempty"`
	Prize      *LotteryPrize      `json:"prize,omitempty"`
	UserState  *LotteryUserState  `json:"user_state,omitempty"`
	Record     *LotteryDrawRecord `json:"record,omitempty"`
	UserCoupon *UserCoupon        `json:"user_coupon,omitempty"`
	RedeemCode *RedeemCode        `json:"redeem_code,omitempty"`
}
