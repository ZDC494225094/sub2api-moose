package wiring

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/marketing"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMarketingManagementRoutesEnforceServiceAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, state := range []string{"disabled", "unavailable"} {
		t.Run(state, func(t *testing.T) {
			reader := &marketingStateProbe{}
			wantStatus, wantReason := 403, "CUSTOM_EXTENSION_DISABLED"
			if state == "unavailable" {
				reader.err = infraerrors.ServiceUnavailable("CUSTOM_EXTENSION_STATE_UNAVAILABLE", "state unavailable")
				wantStatus, wantReason = 503, "CUSTOM_EXTENSION_STATE_UNAVAILABLE"
			}
			gate := &marketingAdmission{states: reader, lookup: marketingManifest(true)}
			coupons := ProvideCouponService(nil, nil, nil, gate)
			lottery := ProvideLotteryService(nil, nil, nil, nil, nil, nil, nil, nil, nil, coupons, gate)
			handler := ProvideMarketingHandler(coupons, lottery)
			router := gin.New()
			// Actual registration/provider/services, with no repositories: any accidental
			// business access fails the test. Host auth is tested separately, not replaced.
			handler.RegisterAdminRoutes(router.Group("/api/v1/admin/payment"))
			for _, route := range []struct{ method, path string }{
				{"POST", "/coupon-templates"}, {"PUT", "/coupon-templates/1"},
				{"POST", "/lottery/activities"}, {"PUT", "/lottery/activities/1"},
				{"DELETE", "/lottery/activities/1"}, {"POST", "/lottery/prizes"}, {"PUT", "/lottery/prizes/1"},
			} {
				req := httptest.NewRequest(route.method, "/api/v1/admin/payment"+route.path, strings.NewReader(`{}`))
				req.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, req)
				require.Equal(t, wantStatus, response.Code, route.method+route.path+response.Body.String())
				require.Contains(t, response.Body.String(), wantReason)
			}
			require.Equal(t, 7, reader.calls)
		})
	}
}

type readTemplates struct {
	service.CouponTemplateRepository
	calls int
}

func (r *readTemplates) List(context.Context, pagination.PaginationParams, marketing.CouponTemplateListFilter) ([]marketing.CouponTemplate, *pagination.PaginationResult, error) {
	r.calls++
	return []marketing.CouponTemplate{}, &pagination.PaginationResult{}, nil
}

type readCoupons struct {
	service.UserCouponRepository
	users []int64
}

func (r *readCoupons) ListByUser(_ context.Context, user int64, _ pagination.PaginationParams, _ marketing.UserCouponListFilter) ([]marketing.UserCoupon, *pagination.PaginationResult, error) {
	r.users = append(r.users, user)
	return []marketing.UserCoupon{{ID: 71, UserID: user, Status: marketing.UserCouponStatusUnused}}, &pagination.PaginationResult{Total: 1}, nil
}

type readActivities struct {
	service.LotteryActivityRepository
	calls int
}

func (r *readActivities) List(context.Context, pagination.PaginationParams, marketing.LotteryActivityListFilter) ([]marketing.LotteryActivity, *pagination.PaginationResult, error) {
	r.calls++
	return []marketing.LotteryActivity{}, &pagination.PaginationResult{}, nil
}

type readDraws struct {
	service.LotteryDrawRecordRepository
	users, activities []int64
}

func (r *readDraws) ListByUser(_ context.Context, user, activity int64, _ pagination.PaginationParams) ([]marketing.LotteryDrawRecord, *pagination.PaginationResult, error) {
	r.users = append(r.users, user)
	r.activities = append(r.activities, activity)
	return []marketing.LotteryDrawRecord{}, &pagination.PaginationResult{}, nil
}

func TestMarketingHistoricalReadRoutesRemainAvailableWithoutAdmission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, state := range []string{"disabled", "unavailable"} {
		t.Run(state, func(t *testing.T) {
			reader := &marketingStateProbe{}
			if state == "unavailable" {
				reader.err = infraerrors.ServiceUnavailable("CUSTOM_EXTENSION_STATE_UNAVAILABLE", "state unavailable")
			}
			gate := &marketingAdmission{states: reader, lookup: marketingManifest(true)}
			templates, coupons, activities, records := &readTemplates{}, &readCoupons{}, &readActivities{}, &readDraws{}
			couponService := ProvideCouponService(templates, coupons, nil, gate)
			lottery := ProvideLotteryService(nil, activities, nil, nil, nil, records, nil, nil, nil, couponService, gate)
			handler := ProvideMarketingHandler(couponService, lottery)
			router := gin.New()
			userGroup := router.Group("/api/v1/payment", func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				c.Next()
			})
			handler.RegisterUserRoutes(userGroup)
			handler.RegisterAdminRoutes(router.Group("/api/v1/admin/payment"))
			for _, path := range []string{
				"/api/v1/payment/coupons?user_id=99",
				"/api/v1/payment/lottery/active",
				"/api/v1/payment/lottery/my-records?activity_id=5&user_id=99",
				"/api/v1/admin/payment/coupon-templates",
				"/api/v1/admin/payment/lottery/activities",
				"/api/v1/admin/payment/lottery/activities/5/draw-records",
			} {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest("GET", path, nil))
				require.Equal(t, 200, response.Code, path+response.Body.String())
				if strings.Contains(path, "/coupons?") {
					require.Contains(t, response.Body.String(), `"id":71`)
					require.Contains(t, response.Body.String(), `"status":"unused"`)
				}
			}
			require.Equal(t, []int64{42}, coupons.users, "history must keep native subject ownership")
			require.Equal(t, []int64{42, 0}, records.users)
			require.Equal(t, []int64{5, 5}, records.activities)
			require.Equal(t, 1, templates.calls)
			require.Equal(t, 2, activities.calls)
			require.Zero(t, reader.calls, "historical reads must not depend on switch availability")
		})
	}
}
