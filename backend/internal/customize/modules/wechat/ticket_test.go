package wechat

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBindTicketRejectsForgedExpiredAndForeignTickets(t *testing.T) {
	svc := NewService(Config{AppID: "wx-app", AppSecret: "secret-a"})
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	ticket := svc.IssueBindTicket("o-openid-12345", now)

	openid, err := svc.VerifyBindTicket(ticket, now.Add(bindTicketTTL))
	require.NoError(t, err)
	require.Equal(t, "o-openid-12345", openid)

	_, err = svc.VerifyBindTicket(ticket, now.Add(bindTicketTTL+time.Second))
	require.Error(t, err, "expired")
	_, err = NewService(Config{AppID: "wx-app", AppSecret: "secret-b"}).VerifyBindTicket(ticket, now)
	require.Error(t, err, "signed with another secret")
	_, err = NewService(Config{}).VerifyBindTicket(ticket, now)
	require.Error(t, err, "integration disabled")

	payload, mac, _ := strings.Cut(ticket, ".")
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	require.NoError(t, err)
	forged := base64.RawURLEncoding.EncodeToString([]byte(strings.Replace(string(raw), "o-openid-12345", "o-victim-1234", 1))) + "." + mac
	_, err = svc.VerifyBindTicket(forged, now)
	require.Error(t, err, "openid swapped under the original signature")

	for _, bad := range []string{"", "o-openid-12345", "a.b", "...", payload} {
		_, err = svc.VerifyBindTicket(bad, now)
		require.Error(t, err, "ticket %q", bad)
	}
}

type bindingStoreStub struct {
	bound map[int64]string
}

func (s *bindingStoreStub) GetByUserID(_ context.Context, userID int64) (*Binding, error) {
	if openid, ok := s.bound[userID]; ok {
		return &Binding{UserID: userID, OpenID: openid}, nil
	}
	return nil, nil
}

func (s *bindingStoreStub) GetByOpenID(context.Context, string) (*Binding, error) { return nil, nil }

func (s *bindingStoreStub) Upsert(_ context.Context, userID int64, openid string) (*Binding, error) {
	s.bound[userID] = openid
	return &Binding{UserID: userID, OpenID: openid}, nil
}

func TestBindOpenIDAcceptsOnlySignedTickets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(Config{AppID: "wx-app", AppSecret: "secret"})
	store := &bindingStoreStub{bound: map[int64]string{}}
	h := NewHandler(svc, store, nil, func(*gin.Context) (int64, bool) { return 42, true })
	r := gin.New()
	h.RegisterUserRoutes(r.Group(""))
	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/wechat/bind", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w := post(`{"openid":"o-victim-openid-1"}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "WECHAT_BIND_TICKET_INVALID")
	require.Empty(t, store.bound)

	w = post(`{"openid":"o-victim-openid-1","ticket":"` + svc.IssueBindTicket("o-own-openid-1", time.Now()) + `"}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "o-own-openid-1", store.bound[42], "the signed openid wins over the body")
}
