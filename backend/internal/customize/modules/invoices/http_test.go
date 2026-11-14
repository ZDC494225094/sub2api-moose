package invoices

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestUserOverviewAndApplyUseIndependentThreshold(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	allowAll(t, s, "100")
	require.NoError(t, s.AddAccess(ctx, []int64{7}, ""))
	minimum := decimal.RequireFromString("60.50")
	require.NoError(t, s.UpdateAccess(ctx, 7, "", &minimum))
	addOrder(t, s, orderFixture{id: 1, user: 7, amount: "60.50"})
	title := addTitle(t, s, 7, TitleInput{TitleType: TitlePersonal, Name: "张三"})
	h := &Handler{Store: s}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 7})
	})
	r.GET("/overview", h.overview)
	r.POST("/apply", h.requireAccess, h.apply)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/overview", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data struct {
			Settings struct {
				MinAmount string `json:"min_amount"`
			} `json:"settings"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "60.50", body.Data.Settings.MinAmount)
	input, err := json.Marshal(ApplyInput{OrderIDs: []int64{1}, TitleID: title.ID, InvoiceType: InvoiceNormal})
	require.NoError(t, err)
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/apply", strings.NewReader(string(input)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
}

func TestPrepareMakeoutReservesPendingAndRejectsInvalidApplications(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	settings := allowAll(t, s, "0")
	addOrder(t, s, orderFixture{id: 1, user: 7, amount: "60.50"})
	title := addTitle(t, s, 7, TitleInput{TitleType: TitlePersonal, Name: "张三"})
	app, err := s.CreateApplication(ctx, 7, ApplyInput{OrderIDs: []int64{1}, TitleID: title.ID, InvoiceType: InvoiceNormal}, title, settings)
	require.NoError(t, err)
	h := &Handler{Store: s}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1})
	})
	r.POST("/applications/:id/prepare-makeout", h.prepareMakeout)
	prepare := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/applications/1/prepare-makeout", nil))
		return w
	}
	require.Equal(t, http.StatusOK, prepare().Code)
	updated, err := s.Application(ctx, app.ID, 0, true)
	require.NoError(t, err)
	require.Equal(t, StatusProcessing, updated.Status)
	requireReason(t, s.Cancel(ctx, 7, app.ID), "INVOICE_STATE_CONFLICT")
	require.Equal(t, http.StatusOK, prepare().Code, "an in-progress application can be reopened")
	_, err = s.DB.Exec(`UPDATE payment_orders SET refund_amount = 1 WHERE id = 1`)
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, prepare().Code)
	_, err = s.DB.Exec(`UPDATE payment_orders SET refund_amount = 0 WHERE id = 1`)
	require.NoError(t, err)
	require.NoError(t, s.Issue(ctx, 1, app.ID, "NO-1", "", nil))
	require.Equal(t, http.StatusConflict, prepare().Code)
}
