package handler

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestCanvasGroupConfigEmptyModelsMarshalsAsArray(t *testing.T) {
	payload, err := json.Marshal(canvasGroupConfig{ID: 1, Name: "No models", RateMultiplier: 1.25, Models: make([]canvasModelConfig, 0)})
	if err != nil {
		t.Fatalf("marshal canvas group config: %v", err)
	}
	if got, want := string(payload), `{"id":1,"name":"No models","platform":"","rateMultiplier":1.25,"models":[]}`; got != want {
		t.Fatalf("canvas group config = %s, want %s", got, want)
	}
}

func TestCanvasGroupRateMultiplierUsesCurrentPeakRate(t *testing.T) {
	h := &PlaygroundHandler{}
	group := service.Group{
		ID:                 1,
		RateMultiplier:     1.5,
		PeakRateEnabled:    true,
		PeakStart:          "10:00",
		PeakEnd:            "12:00",
		PeakRateMultiplier: 0.8,
	}

	if got, want := h.canvasGroupRateMultiplier(context.Background(), 1, group, time.Date(2026, time.August, 11, 10, 30, 0, 0, timezone.Location())), 0.8; got != want {
		t.Fatalf("canvasGroupRateMultiplier() = %v, want %v", got, want)
	}
	if got, want := h.canvasGroupRateMultiplier(context.Background(), 1, group, time.Date(2026, time.August, 11, 9, 30, 0, 0, timezone.Location())), 1.5; got != want {
		t.Fatalf("canvasGroupRateMultiplier() = %v, want %v", got, want)
	}
}

func TestPlaygroundRequestBaseURLUsesLocalAddress(t *testing.T) {
	c := newPlaygroundRequestContext(t, "172.19.0.2:8090")
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "moosecloud.cc")

	if got, want := playgroundRequestBaseURL(c), "http://172.19.0.2:8090"; got != want {
		t.Fatalf("playgroundRequestBaseURL() = %q, want %q", got, want)
	}
}

func TestPlaygroundRequestBaseURLFallsBackToForwardedAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "http://internal/api/v1/playground/runs", nil)
	c.Request.Header.Set("X-Forwarded-Proto", "https")
	c.Request.Header.Set("X-Forwarded-Host", "moosecloud.cc")

	if got, want := playgroundRequestBaseURL(c), "https://moosecloud.cc"; got != want {
		t.Fatalf("playgroundRequestBaseURL() = %q, want %q", got, want)
	}
}

func newPlaygroundRequestContext(t *testing.T, localAddress string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "http://public.example/api/v1/playground/runs", nil)
	addr, err := net.ResolveTCPAddr("tcp", localAddress)
	if err != nil {
		t.Fatalf("resolve local address: %v", err)
	}
	req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, net.Addr(addr)))
	c.Request = req
	return c
}
