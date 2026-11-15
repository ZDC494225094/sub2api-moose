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
const homepageChallengeCookie = accesspolicy.HomepageChallengeCookie
const homepageClearanceCookie = accesspolicy.HomepageClearanceCookie

type HomepageGateSettings interface {
	HomepageCaptchaEnabled(context.Context) (bool, error)
}

type homepageTurnstileSettings interface {
	HomepageTurnstileConfig(context.Context) (accesspolicy.HomepageTurnstileConfig, error)
}

type HomepageTurnstileVerifier interface {
	VerifyHomepageToken(context.Context, accesspolicy.HomepageTurnstileConfig, string, string, string) error
}

// Only homepage documents enter this middleware. Static files, gateway/API
// requests, login, OAuth callbacks and admin recovery keep their native routes.
func HomepageGate(settings HomepageGateSettings, client redis.UniversalClient, secret string, verifiers ...HomepageTurnstileVerifier) gin.HandlerFunc {
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
		var turnstile accesspolicy.HomepageTurnstileConfig
		if provider, ok := settings.(homepageTurnstileSettings); ok {
			turnstile, err = provider.HomepageTurnstileConfig(ctx)
			if err != nil {
				homepageGateUnavailable(c)
				return
			}
		}
		browserID, _ := c.Cookie(homepageChallengeCookie)
		usesTurnstile := turnstile.SiteKey != ""
		passed := gate.HasClearance(clearance, c.Request.UserAgent())
		if usesTurnstile {
			passed = gate.TurnstileClearanceExpires(clearance, browserID, c.Request.UserAgent(), turnstile) > 0
		}
		if path != homepageVerifyPath && passed {
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
				c.Header("X-Homepage-Verification-Error", "origin_mismatch")
				c.String(http.StatusForbidden, "验证请求来源不匹配，请从本站首页刷新后重新验证。")
				c.Abort()
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
				verified := fresh && !usesTurnstile && gate.CheckAnswer(token, c.Request.PostForm.Get("answer"))
				if fresh && usesTurnstile && len(verifiers) > 0 && verifiers[0] != nil {
					// Provider I/O has its own deadline; the short settings/Redis
					// deadline must not cause a valid Cloudflare check to time out.
					verifyCtx, verifyCancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
					verified = verifiers[0].VerifyHomepageToken(verifyCtx, turnstile, c.Request.PostForm.Get("cf-turnstile-response"), id, ip.GetTrustedClientIP(c)) == nil
					verifyCancel()
				}
				if verified {
					var clearance string
					var err error
					if usesTurnstile {
						clearance, err = gate.TurnstileClearance(browserID, c.Request.UserAgent(), turnstile)
					} else {
						clearance, err = gate.Clearance(c.Request.UserAgent())
					}
					if err != nil {
						homepageGateUnavailable(c)
						return
					}
					setHomepageCookie(c, homepageClearanceCookie, clearance, int(accesspolicy.HomepageClearanceTTL/time.Second), http.SameSiteLaxMode)
					if usesTurnstile {
						setHomepageCookie(c, homepageChallengeCookie, browserID, int(accesspolicy.HomepageClearanceTTL/time.Second), http.SameSiteLaxMode)
					} else {
						setHomepageCookie(c, homepageChallengeCookie, "", -1, http.SameSiteStrictMode)
					}
					c.Redirect(http.StatusSeeOther, returnPath)
					c.Abort()
					return
				}
			}
			message = "验证码错误或已过期，请输入下方的新验证码。"
			if usesTurnstile {
				message = "验证未通过或已过期，请重新完成人机验证。"
			}
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
		body, err := accesspolicy.RenderHomepageGateWithTurnstile(token, answer, returnPath, nonce, message, turnstile.SiteKey, browserID)
		if err != nil {
			homepageGateUnavailable(c)
			return
		}
		setHomepageCookie(c, homepageChallengeCookie, browserID, int(accesspolicy.HomepageChallengeTTL/time.Second), http.SameSiteStrictMode)
		csp := "default-src 'none'; img-src data:; style-src 'nonce-" + nonce + "'; script-src 'nonce-" + nonce + "'"
		if usesTurnstile {
			csp += " https://challenges.cloudflare.com; frame-src https://challenges.cloudflare.com; connect-src 'self' https://challenges.cloudflare.com"
		}
		c.Header("Content-Security-Policy", csp+"; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
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
	// Browsers compute this forbidden request header against the public URL,
	// before a reverse proxy can replace Host with an internal upstream address.
	// A cross-origin page cannot spoof same-origin through browser JavaScript.
	// Challenge, browser cookie, answer and single-use checks still run below.
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))) {
	case "same-origin":
		return true
	case "", "none":
		// Older clients lack Fetch Metadata; retain the Origin/Host fallback.
	default:
		// Same-site sibling domains are also cross-origin for this form.
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && strings.EqualFold(u.Host, r.Host)
}
