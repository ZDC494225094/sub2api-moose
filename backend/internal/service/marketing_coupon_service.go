package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrCouponTemplateNotFound = infraerrors.NotFound("COUPON_TEMPLATE_NOT_FOUND", "coupon template not found")
	ErrUserCouponNotFound     = infraerrors.NotFound("USER_COUPON_NOT_FOUND", "user coupon not found")
	ErrUserCouponInvalid      = infraerrors.BadRequest("USER_COUPON_INVALID", "user coupon is invalid")
	ErrUserCouponUnavailable  = infraerrors.Conflict("USER_COUPON_UNAVAILABLE", "user coupon is unavailable")
)

type CouponService struct {
	templateRepo      CouponTemplateRepository
	userCouponRepo    UserCouponRepository
	orderDiscountRepo PaymentOrderDiscountRepository
}

func NewCouponService(templateRepo CouponTemplateRepository, userCouponRepo UserCouponRepository, orderDiscountRepo PaymentOrderDiscountRepository) *CouponService {
	return &CouponService{
		templateRepo:      templateRepo,
		userCouponRepo:    userCouponRepo,
		orderDiscountRepo: orderDiscountRepo,
	}
}

func (s *CouponService) CreateTemplate(ctx context.Context, input *CreateCouponTemplateInput) (*CouponTemplate, error) {
	template := &CouponTemplate{
		Name:            strings.TrimSpace(input.Name),
		Description:     strings.TrimSpace(input.Description),
		Scope:           normalizeCouponScope(input.Scope),
		DiscountAmount:  input.DiscountAmount,
		ThresholdAmount: input.ThresholdAmount,
		ValidDays:       input.ValidDays,
		ValidFrom:       normalizeOptionalTime(input.ValidFrom),
		ValidUntil:      normalizeOptionalTime(input.ValidUntil),
		Status:          normalizeCouponTemplateStatus(input.Status),
		Notes:           strings.TrimSpace(input.Notes),
	}
	if err := validateCouponTemplate(template); err != nil {
		return nil, err
	}
	if err := s.templateRepo.Create(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *CouponService) UpdateTemplate(ctx context.Context, id int64, input *UpdateCouponTemplateInput) (*CouponTemplate, error) {
	template, err := s.templateRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, ErrCouponTemplateNotFound
	}
	if input.Name != nil {
		template.Name = strings.TrimSpace(*input.Name)
	}
	if input.Description != nil {
		template.Description = strings.TrimSpace(*input.Description)
	}
	if input.Scope != nil {
		template.Scope = normalizeCouponScope(*input.Scope)
	}
	if input.DiscountAmount != nil {
		template.DiscountAmount = *input.DiscountAmount
	}
	if input.ThresholdAmount != nil {
		template.ThresholdAmount = *input.ThresholdAmount
	}
	if input.ValidDays != nil {
		template.ValidDays = input.ValidDays
	}
	if input.ValidFrom != nil {
		template.ValidFrom = normalizeOptionalTime(input.ValidFrom)
	}
	if input.ValidUntil != nil {
		template.ValidUntil = normalizeOptionalTime(input.ValidUntil)
	}
	if input.Status != nil {
		template.Status = normalizeCouponTemplateStatus(*input.Status)
	}
	if input.Notes != nil {
		template.Notes = strings.TrimSpace(*input.Notes)
	}
	if err := validateCouponTemplate(template); err != nil {
		return nil, err
	}
	if err := s.templateRepo.Update(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *CouponService) ListTemplates(ctx context.Context, params pagination.PaginationParams, filter CouponTemplateListFilter) ([]CouponTemplate, *pagination.PaginationResult, error) {
	filter.Scope = normalizeCouponScope(filter.Scope)
	return s.templateRepo.List(ctx, params, filter)
}

func (s *CouponService) ListUserCoupons(ctx context.Context, userID int64, params pagination.PaginationParams, filter UserCouponListFilter) ([]UserCoupon, *pagination.PaginationResult, error) {
	filter.Scope = normalizeCouponScope(filter.Scope)
	return s.userCouponRepo.ListByUser(ctx, userID, params, filter)
}

func (s *CouponService) IssueCouponFromTemplate(ctx context.Context, templateID, userID int64, sourceType string, sourceRefID *int64) (*UserCoupon, error) {
	template, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if template == nil {
		return nil, ErrCouponTemplateNotFound
	}
	if template.Status != CouponTemplateStatusActive {
		return nil, infraerrors.BadRequest("COUPON_TEMPLATE_DISABLED", "coupon template is disabled")
	}
	now := time.Now().UTC()
	validFrom := normalizeOptionalTime(template.ValidFrom)
	validUntil := normalizeOptionalTime(template.ValidUntil)
	if validFrom == nil {
		validFrom = &now
	}
	if validUntil == nil && template.ValidDays != nil {
		expires := validFrom.AddDate(0, 0, *template.ValidDays)
		validUntil = &expires
	}
	couponCode, err := generateUserCouponCode()
	if err != nil {
		return nil, err
	}
	coupon := &UserCoupon{
		TemplateID:      template.ID,
		UserID:          userID,
		CouponCode:      couponCode,
		SourceType:      strings.TrimSpace(sourceType),
		SourceRefID:     sourceRefID,
		Scope:           template.Scope,
		DiscountAmount:  template.DiscountAmount,
		ThresholdAmount: template.ThresholdAmount,
		ValidFrom:       validFrom,
		ValidUntil:      validUntil,
		Status:          UserCouponStatusUnused,
		Template:        template,
	}
	if err := s.userCouponRepo.Create(ctx, coupon); err != nil {
		return nil, err
	}
	return coupon, nil
}

func (s *CouponService) PreviewCouponForOrder(ctx context.Context, input ApplyPaymentCouponInput) (*ApplyPaymentCouponResult, error) {
	coupon, err := s.userCouponRepo.GetByID(ctx, input.UserCouponID)
	if err != nil {
		return nil, err
	}
	return s.evaluateCouponForOrder(coupon, input)
}

func (s *CouponService) ReserveCouponForOrder(ctx context.Context, orderID int64, input ApplyPaymentCouponInput) (*ApplyPaymentCouponResult, error) {
	coupon, err := s.userCouponRepo.GetByIDForUpdate(ctx, input.UserCouponID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	result, err := s.evaluateCouponForOrder(coupon, input)
	if err != nil {
		return nil, err
	}
	if err := s.userCouponRepo.ReserveForOrder(ctx, coupon.ID, orderID, now); err != nil {
		return nil, err
	}
	discount := &PaymentOrderDiscount{
		OrderID:          orderID,
		UserCouponID:     &coupon.ID,
		CouponTemplateID: &coupon.TemplateID,
		CouponCode:       coupon.CouponCode,
		Scope:            coupon.Scope,
		DiscountAmount:   coupon.DiscountAmount,
		ThresholdAmount:  coupon.ThresholdAmount,
		OriginalAmount:   input.OrderAmount,
		DiscountedAmount: result.DiscountedAmount,
		Status:           OrderDiscountStatusReserved,
		ReservedAt:       now,
	}
	if err := s.orderDiscountRepo.Create(ctx, discount); err != nil {
		return nil, err
	}
	result.UserCoupon = coupon
	return result, nil
}

func (s *CouponService) ReleaseCouponReservationByOrderID(ctx context.Context, orderID int64) error {
	now := time.Now().UTC()
	if err := s.orderDiscountRepo.MarkReleasedByOrderID(ctx, orderID, now); err != nil {
		return err
	}
	return s.userCouponRepo.ReleaseReservationByOrderID(ctx, orderID, now)
}

func (s *CouponService) ConsumeReservedCouponByOrderID(ctx context.Context, orderID int64) error {
	now := time.Now().UTC()
	if err := s.orderDiscountRepo.MarkUsedByOrderID(ctx, orderID, now); err != nil {
		return err
	}
	return s.userCouponRepo.MarkUsedByOrderID(ctx, orderID, now)
}

func (s *CouponService) evaluateCouponForOrder(coupon *UserCoupon, input ApplyPaymentCouponInput) (*ApplyPaymentCouponResult, error) {
	if coupon == nil {
		return nil, ErrUserCouponNotFound
	}
	now := time.Now().UTC()
	if coupon.UserID != input.UserID {
		return nil, infraerrors.Forbidden("USER_COUPON_FORBIDDEN", "coupon does not belong to user")
	}
	if coupon.Status != UserCouponStatusUnused {
		return nil, ErrUserCouponUnavailable
	}
	if coupon.IsExpired(now) {
		return nil, infraerrors.Conflict("USER_COUPON_EXPIRED", "user coupon has expired")
	}
	if !coupon.SupportsOrderType(input.OrderType) {
		return nil, infraerrors.BadRequest("USER_COUPON_SCOPE_MISMATCH", "coupon does not support this order type")
	}
	if input.OrderAmount < coupon.ThresholdAmount {
		return nil, infraerrors.BadRequest("USER_COUPON_THRESHOLD_UNMET", "coupon threshold not met")
	}
	discountedAmount := input.OrderAmount - coupon.DiscountAmount
	if discountedAmount < 0.01 {
		discountedAmount = 0.01
	}
	return &ApplyPaymentCouponResult{
		UserCoupon:       coupon,
		OriginalAmount:   input.OrderAmount,
		DiscountAmount:   coupon.DiscountAmount,
		DiscountedAmount: discountedAmount,
	}, nil
}

func validateCouponTemplate(template *CouponTemplate) error {
	if template == nil {
		return infraerrors.BadRequest("COUPON_TEMPLATE_INVALID", "coupon template is required")
	}
	if strings.TrimSpace(template.Name) == "" {
		return infraerrors.BadRequest("COUPON_TEMPLATE_NAME_REQUIRED", "coupon template name is required")
	}
	switch template.Scope {
	case CouponScopeBalance, CouponScopeSubscription, CouponScopeUniversal:
	default:
		return infraerrors.BadRequest("COUPON_TEMPLATE_SCOPE_INVALID", "coupon template scope is invalid")
	}
	if template.DiscountAmount <= 0 {
		return infraerrors.BadRequest("COUPON_TEMPLATE_DISCOUNT_INVALID", "discount amount must be greater than zero")
	}
	if template.ThresholdAmount < 0 {
		return infraerrors.BadRequest("COUPON_TEMPLATE_THRESHOLD_INVALID", "threshold amount must be greater than or equal to zero")
	}
	if template.ValidDays != nil && *template.ValidDays <= 0 {
		return infraerrors.BadRequest("COUPON_TEMPLATE_VALID_DAYS_INVALID", "valid days must be greater than zero")
	}
	if template.ValidFrom != nil && template.ValidUntil != nil && template.ValidUntil.Before(template.ValidFrom.UTC()) {
		return infraerrors.BadRequest("COUPON_TEMPLATE_VALID_RANGE_INVALID", "valid until must be after valid from")
	}
	switch template.Status {
	case CouponTemplateStatusActive, CouponTemplateStatusDisabled:
	default:
		return infraerrors.BadRequest("COUPON_TEMPLATE_STATUS_INVALID", "coupon template status is invalid")
	}
	return nil
}

func normalizeCouponScope(scope string) string {
	switch strings.TrimSpace(scope) {
	case CouponScopeBalance:
		return CouponScopeBalance
	case CouponScopeSubscription:
		return CouponScopeSubscription
	case CouponScopeUniversal:
		return CouponScopeUniversal
	default:
		return strings.TrimSpace(scope)
	}
}

func normalizeCouponTemplateStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "", CouponTemplateStatusActive:
		return CouponTemplateStatusActive
	case CouponTemplateStatusDisabled:
		return CouponTemplateStatusDisabled
	default:
		return strings.TrimSpace(status)
	}
}

func normalizeOptionalTime(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	t := v.UTC()
	return &t
}

func generateUserCouponCode() (string, error) {
	raw := make([]byte, 8)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate user coupon code: %w", err)
	}
	return strings.ToUpper(hex.EncodeToString(raw)), nil
}
