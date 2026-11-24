package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	testOriginSecretNew = "0123456789abcdef0123456789abcdef"
	testOriginSecretOld = "fedcba9876543210fedcba9876543210"
)

func newOriginSecretTestRouter(t *testing.T, cfg OriginSecretConfig) (*gin.Engine, *http.Header) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(OriginSecret(cfg))
	seen := &http.Header{}
	handler := func(c *gin.Context) {
		*seen = c.Request.Header.Clone()
		c.String(http.StatusOK, "ok")
	}
	r.GET("/health", handler)
	r.HEAD("/health", handler)
	r.POST("/health", handler)
	r.GET("/admin/", handler)
	r.POST("/v1/messages", handler)
	r.NoRoute(handler)
	return r, seen
}

func doOriginSecretRequest(r http.Handler, method, path string, headers map[string][]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for name, values := range headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func mustParseOriginSecret(t *testing.T, header, secrets string) OriginSecretConfig {
	t.Helper()
	cfg, err := ParseOriginSecretConfig(header, secrets)
	require.NoError(t, err)
	return cfg
}

func TestParseOriginSecretConfig(t *testing.T) {
	cfg, err := ParseOriginSecretConfig("", "")
	require.NoError(t, err)
	require.False(t, cfg.Enabled())

	cfg, err = ParseOriginSecretConfig("x-origin-secret", " , ")
	require.NoError(t, err)
	require.False(t, cfg.Enabled(), "only separators means disabled")

	cfg = mustParseOriginSecret(t, "", testOriginSecretNew)
	require.Equal(t, DefaultOriginSecretHeader, cfg.Header)
	require.Equal(t, 1, cfg.SecretCount())

	cfg = mustParseOriginSecret(t, " x-my-secret ", testOriginSecretNew+" , "+testOriginSecretOld+",")
	require.Equal(t, "X-My-Secret", cfg.Header)
	require.Equal(t, 2, cfg.SecretCount())

	cfg = mustParseOriginSecret(t, `"X-Origin-Secret"`, `"`+testOriginSecretNew+`"`)
	require.Equal(t, "X-Origin-Secret", cfg.Header)
	require.True(t, cfg.matches([]string{testOriginSecretNew}), "surrounding quotes from compose list syntax are stripped")

	_, err = ParseOriginSecretConfig("", "short")
	require.ErrorContains(t, err, "at least")

	_, err = ParseOriginSecretConfig("bad header", testOriginSecretNew)
	require.ErrorContains(t, err, "invalid HTTP header name")
}

func TestOriginSecretDisabledPassesThrough(t *testing.T) {
	r, _ := newOriginSecretTestRouter(t, OriginSecretConfig{})
	w := doOriginSecretRequest(r, http.MethodGet, "/admin/", nil)
	require.Equal(t, http.StatusOK, w.Code)
}

func TestOriginSecretRejectsMissingOrWrongSecret(t *testing.T) {
	r, _ := newOriginSecretTestRouter(t, mustParseOriginSecret(t, "", testOriginSecretNew))

	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string][]string
	}{
		{name: "missing", method: http.MethodGet, path: "/admin/"},
		{name: "wrong", method: http.MethodGet, path: "/admin/", headers: map[string][]string{"X-Origin-Secret": {testOriginSecretOld}}},
		{name: "prefix", method: http.MethodPost, path: "/v1/messages", headers: map[string][]string{"X-Origin-Secret": {testOriginSecretNew[:20]}}},
		{name: "other header name", method: http.MethodGet, path: "/admin/", headers: map[string][]string{"X-Other-Secret": {testOriginSecretNew}}},
		{name: "unknown route", method: http.MethodGet, path: "/nope"},
		{name: "health post", method: http.MethodPost, path: "/health"},
		{name: "health subpath", method: http.MethodGet, path: "/health/x"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := doOriginSecretRequest(r, test.method, test.path, test.headers)
			require.Equal(t, http.StatusForbidden, w.Code)
			require.Empty(t, w.Body.String())
			require.Equal(t, "close", w.Header().Get("Connection"))
		})
	}
}

func TestOriginSecretAcceptsCurrentAndRotatedSecrets(t *testing.T) {
	r, seen := newOriginSecretTestRouter(t, mustParseOriginSecret(t, "X-Origin-Secret", testOriginSecretNew+","+testOriginSecretOld))

	for _, secret := range []string{testOriginSecretNew, testOriginSecretOld, " " + testOriginSecretNew + " "} {
		w := doOriginSecretRequest(r, http.MethodPost, "/v1/messages", map[string][]string{"x-origin-secret": {secret}})
		require.Equal(t, http.StatusOK, w.Code)
		require.Empty(t, seen.Values("X-Origin-Secret"), "secret must not reach handlers or upstream forwarding")
	}
}

func TestOriginSecretAcceptsAppendedHeaderValue(t *testing.T) {
	r, _ := newOriginSecretTestRouter(t, mustParseOriginSecret(t, "", testOriginSecretNew))
	w := doOriginSecretRequest(r, http.MethodGet, "/admin/", map[string][]string{
		"X-Origin-Secret": {"forged-by-visitor", testOriginSecretNew},
	})
	require.Equal(t, http.StatusOK, w.Code)
}

func TestOriginSecretHealthExempt(t *testing.T) {
	r, seen := newOriginSecretTestRouter(t, mustParseOriginSecret(t, "", testOriginSecretNew))
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := doOriginSecretRequest(r, method, "/health", map[string][]string{"X-Origin-Secret": {"wrong-but-exempt-path"}})
		require.Equal(t, http.StatusOK, w.Code)
		require.Empty(t, seen.Values("X-Origin-Secret"))
	}
}

func TestLoadOriginSecretConfigFromEnv(t *testing.T) {
	t.Setenv(OriginSecretHeaderEnv, "X-Edge-Key")
	t.Setenv(OriginSecretEnv, testOriginSecretNew)
	cfg, err := LoadOriginSecretConfigFromEnv()
	require.NoError(t, err)
	require.Equal(t, "X-Edge-Key", cfg.Header)
	require.Equal(t, 1, cfg.SecretCount())
}

func TestOriginSecretLoopbackHeaderUsesPrimarySecret(t *testing.T) {
	t.Setenv(OriginSecretEnv, "")
	_, _, ok := OriginSecretLoopbackHeader()
	require.False(t, ok, "disabled check needs no header")

	t.Setenv(OriginSecretHeaderEnv, "x-cdn-key")
	t.Setenv(OriginSecretEnv, testOriginSecretNew+","+testOriginSecretOld)
	name, value, ok := OriginSecretLoopbackHeader()
	require.True(t, ok)
	require.Equal(t, "X-Cdn-Key", name)
	require.Equal(t, testOriginSecretNew, value, "loopback calls use the new secret while rotating")

	cfg, err := LoadOriginSecretConfigFromEnv()
	require.NoError(t, err)
	r, _ := newOriginSecretTestRouter(t, cfg)
	w := doOriginSecretRequest(r, http.MethodPost, "/v1/messages", map[string][]string{name: {value}})
	require.Equal(t, http.StatusOK, w.Code)
}
