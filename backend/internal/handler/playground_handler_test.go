package handler

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

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
