package service

import "time"

const (
	CouponScopeBalance      = "balance"
	CouponScopeSubscription = "subscription"
	CouponScopeUniversal    = "universal"
)

const (
	CouponTemplateStatusActive   = "active"
	CouponTemplateStatusDisabled = "disabled"
)

const (
	UserCouponStatusUnused   = "unused"
	UserCouponStatusReserved = "reserved"
	UserCouponStatusUsed     = "used"
	UserCouponStatusExpired  = "expired"
	UserCouponStatusDisabled = "disabled"
)

const (
	UserCouponSourceLottery = "lottery"
)

const (
	OrderDiscountStatusReserved = "reserved"
	OrderDiscountStatusReleased = "released"
	OrderDiscountStatusUsed     = "used"
)

type CouponTemplate struct {
	ID              int64      `json:"id"`
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Scope           string     `json:"scope"`
	DiscountAmount  float64    `json:"discount_amount"`
	ThresholdAmount float64    `json:"threshold_amount"`
	ValidDays       *int       `json:"valid_days,omitempty"`
	ValidFrom       *time.Time `json:"valid_from,omitempty"`
	ValidUntil      *time.Time `json:"valid_until,omitempty"`
	Status          string     `json:"status"`
	Notes           string     `json:"notes"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type UserCoupon struct {
	ID              int64           `json:"id"`
	TemplateID      int64           `json:"template_id"`
	UserID          int64           `json:"user_id"`
	CouponCode      string          `json:"coupon_code"`
	SourceType      string          `json:"source_type"`
	SourceRefID     *int64          `json:"source_ref_id,omitempty"`
	Scope           string          `json:"scope"`
	DiscountAmount  float64         `json:"discount_amount"`
	ThresholdAmount float64         `json:"threshold_amount"`
	ValidFrom       *time.Time      `json:"valid_from,omitempty"`
	ValidUntil      *time.Time      `json:"valid_until,omitempty"`
	Status          string          `json:"status"`
	ReservedOrderID *int64          `json:"reserved_order_id,omitempty"`
	ReservedAt      *time.Time      `json:"reserved_at,omitempty"`
	UsedOrderID     *int64          `json:"used_order_id,omitempty"`
	UsedAt          *time.Time      `json:"used_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Template        *CouponTemplate `json:"template,omitempty"`
}

func (c *UserCoupon) IsExpired(at time.Time) bool {
	if c == nil || c.ValidUntil == nil {
		return false
	}
	return at.After(c.ValidUntil.UTC())
}

func (c *UserCoupon) SupportsOrderType(orderType string) bool {
	if c == nil {
		return false
	}
	switch c.Scope {
	case CouponScopeUniversal:
		return orderType == "balance" || orderType == "subscription"
	case CouponScopeBalance:
		return orderType == "balance"
	case CouponScopeSubscription:
		return orderType == "subscription"
	default:
		return false
	}
}

type PaymentOrderDiscount struct {
	ID               int64      `json:"id"`
	OrderID          int64      `json:"order_id"`
	UserCouponID     *int64     `json:"user_coupon_id,omitempty"`
	CouponTemplateID *int64     `json:"coupon_template_id,omitempty"`
	CouponCode       string     `json:"coupon_code"`
	Scope            string     `json:"scope"`
	DiscountAmount   float64    `json:"discount_amount"`
	ThresholdAmount  float64    `json:"threshold_amount"`
	OriginalAmount   float64    `json:"original_amount"`
	DiscountedAmount float64    `json:"discounted_amount"`
	Status           string     `json:"status"`
	ReservedAt       time.Time  `json:"reserved_at"`
	ReleasedAt       *time.Time `json:"released_at,omitempty"`
	UsedAt           *time.Time `json:"used_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type CreateCouponTemplateInput struct {
	Name            string     `json:"name"`
	Description     string     `json:"description"`
	Scope           string     `json:"scope"`
	DiscountAmount  float64    `json:"discount_amount"`
	ThresholdAmount float64    `json:"threshold_amount"`
	ValidDays       *int       `json:"valid_days,omitempty"`
	ValidFrom       *time.Time `json:"valid_from,omitempty"`
	ValidUntil      *time.Time `json:"valid_until,omitempty"`
	Status          string     `json:"status"`
	Notes           string     `json:"notes"`
}

type UpdateCouponTemplateInput struct {
	Name            *string    `json:"name,omitempty"`
	Description     *string    `json:"description,omitempty"`
	Scope           *string    `json:"scope,omitempty"`
	DiscountAmount  *float64   `json:"discount_amount,omitempty"`
	ThresholdAmount *float64   `json:"threshold_amount,omitempty"`
	ValidDays       *int       `json:"valid_days,omitempty"`
	ValidFrom       *time.Time `json:"valid_from,omitempty"`
	ValidUntil      *time.Time `json:"valid_until,omitempty"`
	Status          *string    `json:"status,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
}

type CouponTemplateListFilter struct {
	Status string
	Scope  string
	Search string
}

type UserCouponListFilter struct {
	Status string
	Scope  string
}

type ApplyPaymentCouponInput struct {
	UserID       int64
	OrderType    string
	OrderAmount  float64
	UserCouponID int64
}

type ApplyPaymentCouponResult struct {
	UserCoupon       *UserCoupon `json:"user_coupon,omitempty"`
	OriginalAmount   float64     `json:"original_amount"`
	DiscountAmount   float64     `json:"discount_amount"`
	DiscountedAmount float64     `json:"discounted_amount"`
}

type UserCouponOrderView struct {
	ID              int64      `json:"id"`
	CouponCode      string     `json:"coupon_code"`
	Scope           string     `json:"scope"`
	DiscountAmount  float64    `json:"discount_amount"`
	ThresholdAmount float64    `json:"threshold_amount"`
	Status          string     `json:"status"`
	ValidFrom       *time.Time `json:"valid_from,omitempty"`
	ValidUntil      *time.Time `json:"valid_until,omitempty"`
	TemplateName    string     `json:"template_name,omitempty"`
}
