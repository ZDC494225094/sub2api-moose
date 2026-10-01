package marketing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type httpReward struct {
	NativeField string `json:"native_field"`
}
type lotteryHTTPStub struct {
	LotteryHTTPService[httpReward]
	input LotteryDrawInput
	calls int
	err   error
}

func (s *lotteryHTTPStub) Draw(ctx context.Context, in LotteryDrawInput) (*LotteryDrawResult[httpReward], error) {
	s.calls++
	s.input = in
	if s.err != nil {
		return nil, s.err
	}
	return &LotteryDrawResult[httpReward]{RedeemCode: &httpReward{NativeField: "preserved"}}, nil
}
func TestMarketingHTTPDrawUsesAuthenticatedIdentityAndNativePayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, scenario := range []string{"ok", "unauthenticated", "invalid", "conflict"} {
		t.Run(scenario, func(t *testing.T) {
			stub := &lotteryHTTPStub{}
			if scenario == "conflict" {
				stub.err = infraerrors.Conflict("LOTTERY_NO_AVAILABLE_CHANCES", "no chances")
			}
			h := NewHandler[httpReward](nil, stub, func(*gin.Context) (int64, bool) { return 42, scenario != "unauthenticated" })
			r := gin.New()
			h.RegisterUserRoutes(r.Group("/payment"))
			body := `{"activity_id":9,"use_wallet":true,"user_id":999}`
			if scenario == "invalid" {
				body = `{"activity_id":0}`
			}
			req := httptest.NewRequest(http.MethodPost, "/payment/lottery/draw?user_id=888", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			switch scenario {
			case "unauthenticated":
				require.Equal(t, 401, w.Code)
				require.Zero(t, stub.calls)
			case "invalid":
				require.Equal(t, 400, w.Code)
				require.Zero(t, stub.calls)
			case "conflict":
				require.Equal(t, 409, w.Code)
				require.Contains(t, w.Body.String(), "LOTTERY_NO_AVAILABLE_CHANCES")
			default:
				require.Equal(t, 200, w.Code)
				require.JSONEq(t, `{"code":0,"message":"success","data":{"redeem_code":{"native_field":"preserved"}}}`, w.Body.String())
			}
			if stub.calls > 0 {
				require.Equal(t, LotteryDrawInput{UserID: 42, ActivityID: 9, UseWallet: true}, stub.input)
			}
		})
	}
}

type couponHTTPStub struct {
	CouponHTTPService
	userID int64
	filter UserCouponListFilter
	params pagination.PaginationParams
}

func (s *couponHTTPStub) ListUserCoupons(ctx context.Context, id int64, p pagination.PaginationParams, f UserCouponListFilter) ([]UserCoupon, *pagination.PaginationResult, error) {
	s.userID = id
	s.params = p
	s.filter = f
	return []UserCoupon{{ID: 7, UserID: id}}, &pagination.PaginationResult{Total: 31, Page: p.Page, PageSize: p.PageSize}, nil
}
func TestMarketingHTTPCouponFiltersAndEnvelopePreserved(t *testing.T) {
	stub := &couponHTTPStub{}
	h := NewHandler[httpReward](stub, nil, func(*gin.Context) (int64, bool) { return 42, true })
	r := gin.New()
	h.RegisterUserRoutes(r.Group("/payment"))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/payment/coupons?page=2&page_size=10&status=%20unused%20&scope=balance&user_id=999", nil))
	require.Equal(t, 200, w.Code)
	require.Equal(t, int64(42), stub.userID)
	require.Equal(t, UserCouponListFilter{Status: "unused", Scope: "balance"}, stub.filter)
	require.Equal(t, pagination.PaginationParams{Page: 2, PageSize: 10}, stub.params)
	var payload struct {
		Code int
		Data struct {
			Items    []UserCoupon
			Total    int
			Page     int
			PageSize int `json:"page_size"`
			Pages    int
		}
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
	require.Equal(t, 31, payload.Data.Total)
	require.Equal(t, 2, payload.Data.Page)
	require.Equal(t, 10, payload.Data.PageSize)
	require.Equal(t, 4, payload.Data.Pages)
	require.Equal(t, int64(42), payload.Data.Items[0].UserID)
}
