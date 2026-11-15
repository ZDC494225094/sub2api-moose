package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const homepageVerifyPath = "/_homepage/verify"
const homepageChallengeCookie = "sub2api_home_challenge"
const homepageClearanceCookie = "sub2api_home_clearance"

type HomepageGateSettings interface {
	HomepageCaptchaEnabled(context.Context) (bool, error)
}

// Only homepage documents enter this middleware. Static files, gateway/API
// requests, login, OAuth callbacks and admin recovery keep their native routes.
func HomepageGate(settings HomepageGateSettings, client redis.UniversalClient, secret string) gin.HandlerFunc {
	gate, gateErr := accesspolicy.NewHomepageGate(secret)
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !accesspolicy.IsHomepagePath(path) && path != homepageVerifyPath {
			c.Next()
			return
		}
		if path == homepageVerifyPath && c.Request.Method != http.MethodPost {
			c.Header("Allow", "POST")
			c.AbortWithStatus(http.StatusMethodNotAllowed)
			return
		}
		if path != homepageVerifyPath && c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Next()
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		enabled, err := settings.HomepageCaptchaEnabled(ctx)
		if err != nil {
			homepageGateUnavailable(c)
			return
		}
		if !enabled {
			if path == homepageVerifyPath {
				c.Header("Cache-Control", "no-store")
				c.Redirect(http.StatusSeeOther, "/")
				c.Abort()
				return
			}
			if homepageDevCheck(c, "disabled") {
				return
			}
			c.Next()
			return
		}
		// A response selected using clearance must never be shared through an HTML cache.
		c.Header("Cache-Control", "private, no-store")
		c.Header("Vary", "Cookie")
		c.Header("X-Homepage-Verification", "required")
		if gateErr != nil {
			homepageGateUnavailable(c)
			return
		}
		clearance, _ := c.Cookie(homepageClearanceCookie)
		if path != homepageVerifyPath && gate.HasClearance(clearance, c.Request.UserAgent()) {
			c.Header("X-Homepage-Verification", "passed")
			if homepageDevCheck(c, "passed") {
				return
			}
			c.Next()
			return
		}
		if client == nil {
			homepageGateUnavailable(c)
			return
		}
		bucket := "issue"
		limit := int64(30)
		if path == homepageVerifyPath {
			bucket = "submit"
			limit = 12
		}
		address := sha256.Sum256([]byte(ip.GetTrustedClientIP(c)))
		key := "homepage-gate:rate:" + bucket + ":" + hex.EncodeToString(address[:])
		count, err := client.Eval(ctx, `local n=redis.call('INCR',KEYS[1]); if n==1 then redis.call('EXPIRE',KEYS[1],60) end; return n`, []string{key}).Int64()
		if err != nil {
			homepageGateUnavailable(c)
			return
		}
		if count > limit {
			c.Header("Retry-After", "60")
			c.String(http.StatusTooManyRequests, "验证请求过于频繁，请稍后再试。")
			c.Abort()
			return
		}
		returnPath := c.Request.URL.RequestURI()
		message := ""
		status := http.StatusOK
		if path == homepageVerifyPath {
			if !homepageSameOrigin(c.Request) {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
			if err := c.Request.ParseForm(); err != nil {
				c.AbortWithStatus(http.StatusBadRequest)
				return
			}
			returnPath = accesspolicy.HomepageReturnPath(c.Request.PostForm.Get("return"))
			browserID, _ := c.Cookie(homepageChallengeCookie)
			token := c.Request.PostForm.Get("challenge")
			id, err := gate.ChallengeID(token, browserID, c.Request.UserAgent())
			if err == nil {
				// Shared atomic consumption also protects multiple backend replicas.
				fresh, storeErr := client.SetNX(ctx, "homepage-gate:used:"+id, "1", accesspolicy.HomepageChallengeTTL).Result()
				if storeErr != nil {
					homepageGateUnavailable(c)
					return
				}
				if fresh && gate.CheckAnswer(token, c.Request.PostForm.Get("answer")) {
					clearance, err := gate.Clearance(c.Request.UserAgent())
					if err != nil {
						homepageGateUnavailable(c)
						return
					}
					setHomepageCookie(c, homepageClearanceCookie, clearance, int(accesspolicy.HomepageClearanceTTL/time.Second), http.SameSiteLaxMode)
					setHomepageCookie(c, homepageChallengeCookie, "", -1, http.SameSiteStrictMode)
					c.Redirect(http.StatusSeeOther, returnPath)
					c.Abort()
					return
				}
			}
			message = "验证码错误或已过期，请输入下方的新验证码。"
			status = http.StatusBadRequest
		}
		token, browserID, answer, err := gate.NewChallenge(c.Request.UserAgent())
		if err != nil {
			homepageGateUnavailable(c)
			return
		}
		nonce := GetNonceFromContext(c)
		if nonce == "" {
			nonce, err = GenerateNonce()
			if err != nil {
				homepageGateUnavailable(c)
				return
			}
		}
		body, err := accesspolicy.RenderHomepageGate(token, answer, returnPath, nonce, message)
		if err != nil {
			homepageGateUnavailable(c)
			return
		}
		setHomepageCookie(c, homepageChallengeCookie, browserID, int(accesspolicy.HomepageChallengeTTL/time.Second), http.SameSiteStrictMode)
		c.Header("Content-Security-Policy", "default-src 'none'; img-src data:; style-src 'nonce-"+nonce+"'; script-src 'nonce-"+nonce+"'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		// Preserve the Origin of the same-origin form POST for CSRF validation.
		// no-referrer makes Chromium submit Origin: null on HTTP deployments.
		c.Header("Referrer-Policy", "same-origin")
		c.Header("X-Robots-Tag", "noindex, nofollow")
		if c.Request.Method == http.MethodHead {
			c.Status(status)
		} else {
			c.Data(status, "text/html; charset=utf-8", body)
		}
		c.Abort()
	}
}

// Vite checks the gate before serving its own HTML. A probe can only suppress
// the document; it never skips verification or returns application resources.
func homepageDevCheck(c *gin.Context, decision string) bool {
	if c.GetHeader("X-Homepage-Verification-Check") != "1" {
		return false
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Homepage-Verification", decision)
	c.AbortWithStatus(http.StatusNoContent)
	return true
}

func homepageGateUnavailable(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Retry-After", "5")
	c.String(http.StatusServiceUnavailable, "访问验证暂时不可用，请稍后刷新。")
	c.Abort()
}

func setHomepageCookie(c *gin.Context, name, value string, maxAge int, sameSite http.SameSite) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: sameSite})
}

func homepageSameOrigin(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && strings.EqualFold(u.Host, r.Host)
}
