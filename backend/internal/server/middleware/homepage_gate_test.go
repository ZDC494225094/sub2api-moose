package middleware

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const homepageTestSecret = "homepage-test-secret-with-sufficient-length"

func TestHomepageDevProbeChecksClearanceBeforeAllowingVite(t *testing.T) {
	settings := &homepageSettingsStub{enabled: true}
	router, _, served := homepageFixture(t, settings)
	headers := map[string]string{"X-Homepage-Verification-Check": "1"}
	first := homepageRequest(router, "GET", "/", "", nil, headers)
	require.Equal(t, 200, first.Code)
	require.Equal(t, "required", first.Header().Get("X-Homepage-Verification"))
	require.Contains(t, first.Body.String(), "name=\"challenge\"")
	require.Zero(t, served.Load())
	gate, err := accesspolicy.NewHomepageGate(homepageTestSecret)
	require.NoError(t, err)
	token, err := gate.Clearance("test-browser")
	require.NoError(t, err)
	passed := homepageRequest(router, "GET", "/", "", []*http.Cookie{{Name: homepageClearanceCookie, Value: token}}, headers)
	require.Equal(t, 204, passed.Code)
	require.Equal(t, "passed", passed.Header().Get("X-Homepage-Verification"))
	require.Empty(t, passed.Body.String())
	require.Zero(t, served.Load())
	settings.enabled = false
	disabled := homepageRequest(router, "GET", "/", "", nil, headers)
	require.Equal(t, 204, disabled.Code)
	require.Equal(t, "disabled", disabled.Header().Get("X-Homepage-Verification"))
	require.Zero(t, served.Load())
	// Normal clients still receive the page, never a blank probe response.
	normal := homepageRequest(router, "GET", "/", "", nil, nil)
	require.Equal(t, 200, normal.Code)
	require.Contains(t, normal.Body.String(), "/assets/main.js")
}

type homepageSettingsStub struct {
	enabled bool
	err     error
	calls   int
}

func (s *homepageSettingsStub) HomepageCaptchaEnabled(context.Context) (bool, error) {
	s.calls++
	return s.enabled, s.err
}

func homepageFixture(t *testing.T, settings *homepageSettingsStub) (*gin.Engine, *redis.Client, *atomic.Int32) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	router := gin.New()
	router.Use(HomepageGate(settings, rdb, homepageTestSecret))
	var served atomic.Int32
	router.NoRoute(func(c *gin.Context) {
		served.Add(1)
		c.Data(200, "text/html", []byte(`<script src="/assets/main.js"></script>`))
	})
	return router, rdb, &served
}

func homepageRequest(router http.Handler, method, path, body string, cookies []*http.Cookie, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("User-Agent", "test-browser")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// Only tests know the signing secret. Decode a real issued challenge so the HTTP
// flow verifies a correct answer without a production bypass or fixed captcha.
func homepageAnswer(t *testing.T, page string) (string, string) {
	t.Helper()
	match := regexp.MustCompile(`name="challenge" value="([^"]+)"`).FindStringSubmatch(page)
	require.Len(t, match, 2)
	data, err := base64.RawURLEncoding.DecodeString(match[1])
	require.NoError(t, err)
	key := sha256.Sum256([]byte("sub2api/homepage-gate/v1/" + homepageTestSecret))
	block, err := aes.NewCipher(key[:])
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	n := aead.NonceSize()
	raw, err := aead.Open(nil, data[:n], data[n:], []byte("homepage/v1"))
	require.NoError(t, err)
	var payload struct {
		Answer string `json:"a"`
	}
	require.NoError(t, json.Unmarshal(raw, &payload))
	return match[1], payload.Answer
}

func TestHomepageGateStopsAllHomepageWorkUntilVerified(t *testing.T) {
	settings := &homepageSettingsStub{enabled: true}
	router, _, served := homepageFixture(t, settings)
	first := homepageRequest(router, "GET", "https://example.test/?aff=keep", "", nil, nil)
	require.Equal(t, 200, first.Code)
	require.Zero(t, served.Load())
	require.NotContains(t, first.Body.String(), "/assets/")
	require.Contains(t, first.Header().Get("Cache-Control"), "no-store")
	require.Contains(t, first.Header().Get("Content-Security-Policy"), "form-action 'self'")
	cookies := first.Result().Cookies()
	require.Len(t, cookies, 1)
	require.True(t, cookies[0].HttpOnly)
	require.True(t, cookies[0].Secure)
	token, answer := homepageAnswer(t, first.Body.String())
	body := url.Values{"challenge": {token}, "answer": {answer}, "return": {"/?aff=keep#plans"}}.Encode()
	verified := homepageRequest(router, "POST", "https://example.test/_homepage/verify", body, cookies, map[string]string{"Origin": "https://example.test"})
	require.Equal(t, 303, verified.Code)
	require.Equal(t, "/?aff=keep#plans", verified.Header().Get("Location"))
	var clearance *http.Cookie
	for _, cookie := range verified.Result().Cookies() {
		if cookie.Name == homepageClearanceCookie {
			clearance = cookie
		}
	}
	require.NotNil(t, clearance)
	require.Equal(t, 86400, clearance.MaxAge)
	require.Equal(t, http.SameSiteLaxMode, clearance.SameSite)
	require.True(t, clearance.Secure)
	passed := homepageRequest(router, "GET", "https://example.test/", "", []*http.Cookie{clearance}, nil)
	require.Equal(t, "passed", passed.Header().Get("X-Homepage-Verification"))
	require.Contains(t, passed.Body.String(), "/assets/main.js")
	require.EqualValues(t, 1, served.Load())
	replayed := homepageRequest(router, "POST", "https://example.test/_homepage/verify", body, cookies, nil)
	require.Equal(t, 400, replayed.Code)
	require.NotContains(t, replayed.Header().Get("Set-Cookie"), homepageClearanceCookie)
}

func TestHomepageWrongAnswerConsumesChallengeAndRejectsCrossSiteAndOversize(t *testing.T) {
	router, _, _ := homepageFixture(t, &homepageSettingsStub{enabled: true})
	first := homepageRequest(router, "GET", "/", "", nil, nil)
	token, answer := homepageAnswer(t, first.Body.String())
	cookies := first.Result().Cookies()
	body := url.Values{"challenge": {token}, "answer": {"wrong"}}.Encode()
	require.Equal(t, 400, homepageRequest(router, "POST", homepageVerifyPath, body, cookies, nil).Code)
	body = url.Values{"challenge": {token}, "answer": {answer}}.Encode()
	require.Equal(t, 400, homepageRequest(router, "POST", homepageVerifyPath, body, cookies, nil).Code)
	require.Equal(t, 403, homepageRequest(router, "POST", homepageVerifyPath, body, cookies, map[string]string{"Origin": "https://evil.test"}).Code)
	require.Equal(t, 400, homepageRequest(router, "POST", homepageVerifyPath, strings.Repeat("a", 9000), cookies, nil).Code)
}

func TestHomepageGateDoesNotGateAPIsRecoveryOrStaticFiles(t *testing.T) {
	settings := &homepageSettingsStub{enabled: true, err: errors.New("offline")}
	router, _, served := homepageFixture(t, settings)
	for _, path := range []string{"/login", "/admin/settings", "/auth/callback", "/setup", "/health", "/api/v1/auth/login", "/v1/responses", "/canvas/", "/assets/main.js", "/logo.svg"} {
		require.Equal(t, 200, homepageRequest(router, "GET", path, "", nil, nil).Code, path)
	}
	require.Zero(t, settings.calls)
	require.EqualValues(t, 10, served.Load())
	for _, path := range []string{"/", "/home", "/home/", "/index.html"} {
		require.Equal(t, 503, homepageRequest(router, "GET", path, "", nil, nil).Code)
	}
	settings.err = nil
	settings.enabled = false
	require.Contains(t, homepageRequest(router, "GET", "/", "", nil, nil).Body.String(), "/assets/main.js")
}

func TestHomepageRateLimitAndUnavailableStoreFailClosed(t *testing.T) {
	router, rdb, served := homepageFixture(t, &homepageSettingsStub{enabled: true})
	for range 12 {
		homepageRequest(router, "POST", homepageVerifyPath, "", nil, nil)
	}
	require.Equal(t, 429, homepageRequest(router, "POST", homepageVerifyPath, "", nil, nil).Code)
	require.Zero(t, served.Load())
	require.NoError(t, rdb.Close())
	require.Equal(t, 503, homepageRequest(router, "GET", "/", "", nil, nil).Code)
}

func TestHomepageConcurrentReplayIssuesOnlyOneClearance(t *testing.T) {
	// Use separate settings per engine to avoid test-stub call counter races.
	router, rdb, _ := homepageFixture(t, &homepageSettingsStub{enabled: true})
	first := homepageRequest(router, "GET", "/", "", nil, nil)
	token, answer := homepageAnswer(t, first.Body.String())
	body := url.Values{"challenge": {token}, "answer": {answer}}.Encode()
	cookies := first.Result().Cookies()
	var wg sync.WaitGroup
	var successes atomic.Int32
	for range 8 {
		engine := gin.New()
		engine.Use(HomepageGate(&homepageSettingsStub{enabled: true}, rdb, homepageTestSecret))
		wg.Add(1)
		go func() {
			defer wg.Done()
			if homepageRequest(engine, "POST", homepageVerifyPath, body, cookies, nil).Code == 303 {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, successes.Load())
}
