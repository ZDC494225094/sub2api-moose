package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type CouponTemplateRepository interface {
	Create(ctx context.Context, input *CouponTemplate) error
	Update(ctx context.Context, coupon *CouponTemplate) error
	GetByID(ctx context.Context, id int64) (*CouponTemplate, error)
	List(ctx context.Context, params pagination.PaginationParams, filter CouponTemplateListFilter) ([]CouponTemplate, *pagination.PaginationResult, error)
}

type UserCouponRepository interface {
	Create(ctx context.Context, coupon *UserCoupon) error
	GetByID(ctx context.Context, id int64) (*UserCoupon, error)
	GetByIDForUpdate(ctx context.Context, id int64) (*UserCoupon, error)
	GetByCode(ctx context.Context, code string) (*UserCoupon, error)
	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams, filter UserCouponListFilter) ([]UserCoupon, *pagination.PaginationResult, error)
	ReserveForOrder(ctx context.Context, couponID int64, orderID int64, reservedAt time.Time) error
	ReleaseReservationByOrderID(ctx context.Context, orderID int64, releasedAt time.Time) error
	MarkUsedByOrderID(ctx context.Context, orderID int64, usedAt time.Time) error
}

type PaymentOrderDiscountRepository interface {
	Create(ctx context.Context, item *PaymentOrderDiscount) error
	GetByOrderID(ctx context.Context, orderID int64) (*PaymentOrderDiscount, error)
	MarkReleasedByOrderID(ctx context.Context, orderID int64, releasedAt time.Time) error
	MarkUsedByOrderID(ctx context.Context, orderID int64, usedAt time.Time) error
}
