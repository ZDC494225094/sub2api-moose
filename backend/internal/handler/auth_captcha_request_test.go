//go:build unit

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/accesspolicy"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type rejectingHomepageAuthVerifier struct{ calls int }

func (v *rejectingHomepageAuthVerifier) VerifyToken(context.Context, string, string, string) (*service.TurnstileVerifyResponse, error) {
	v.calls++
	return &service.TurnstileVerifyResponse{Success: false}, nil
}

func TestHomepageClearanceNeverBypassesLoginOrRegistrationCaptcha(t *testing.T) {
	cfg := &config.Config{}
	cfg.JWT.Secret = "homepage-test-secret-with-sufficient-length"
	settings := service.NewSettingService(&settingHandlerPublicRepoStub{values: map[string]string{
		service.SettingKeyHomepageCaptchaEnabled: "true", service.SettingKeyTurnstileEnabled: "true",
		service.SettingKeyTurnstileSiteKey: "site", service.SettingKeyTurnstileSecretKey: "secret", service.SettingKeyEmailVerifyEnabled: "true",
	}}, cfg)
	verifier := &rejectingHomepageAuthVerifier{}
	auth := service.NewAuthService(nil, nil, nil, nil, cfg, settings, nil, service.NewTurnstileService(settings, verifier), nil, nil, nil, nil, nil)
	handler := &AuthHandler{cfg: cfg, authService: auth, settingSvc: settings}
	router := gin.New()
	router.Use(middleware.HomepageGate(settings, nil, cfg.JWT.Secret))
	router.POST("/api/v1/auth/login", handler.Login)
	router.POST("/api/v1/auth/register", handler.Register)
	router.POST("/api/v1/auth/send-verify-code", handler.SendVerifyCode)
	gate, err := accesspolicy.NewHomepageGate(cfg.JWT.Secret)
	require.NoError(t, err)
	receipt, err := gate.TurnstileClearance("browser", "agent", accesspolicy.HomepageTurnstileConfig{SiteKey: "site", SecretKey: "secret"})
	require.NoError(t, err)
	for _, path := range []string{"login", "register", "send-verify-code"} {
		for _, proof := range []string{"", "already-used-token"} {
			body, err := json.Marshal(map[string]string{"email": "test@example.com", "password": "secret-123", "verify_code": "123456", "turnstile_token": proof})
			require.NoError(t, err)
			req := httptest.NewRequest("POST", "/api/v1/auth/"+path, strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "agent")
			req.AddCookie(&http.Cookie{Name: accesspolicy.HomepageClearanceCookie, Value: receipt})
			req.AddCookie(&http.Cookie{Name: accesspolicy.HomepageChallengeCookie, Value: "browser"})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code, path)
			require.Contains(t, w.Body.String(), "TURNSTILE_VERIFICATION_FAILED", path)
		}
	}
	require.Equal(t, 3, verifier.calls)
}

func TestAuthRequestsBindTencentCaptchaProof(t *testing.T) {
	const payload = `{"email":"user@example.com","password":"secret-123","tencent_captcha_ticket":"ticket-value","tencent_captcha_randstr":"@rand-value"}`

	tests := []struct {
		name   string
		decode func([]byte) service.CaptchaProof
	}{
		{
			name: "登录",
			decode: func(raw []byte) service.CaptchaProof {
				var req LoginRequest
				require.NoError(t, json.Unmarshal(raw, &req))
				return captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
			},
		},
		{
			name: "注册",
			decode: func(raw []byte) service.CaptchaProof {
				var req RegisterRequest
				require.NoError(t, json.Unmarshal(raw, &req))
				return captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
			},
		},
		{
			name: "发送邮箱验证码",
			decode: func(raw []byte) service.CaptchaProof {
				var req SendVerifyCodeRequest
				require.NoError(t, json.Unmarshal(raw, &req))
				return captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
			},
		},
		{
			name: "忘记密码",
			decode: func(raw []byte) service.CaptchaProof {
				var req ForgotPasswordRequest
				require.NoError(t, json.Unmarshal(raw, &req))
				return captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
			},
		},
		{
			name: "OAuth启动",
			decode: func(raw []byte) service.CaptchaProof {
				var req oauthStartCaptchaRequest
				require.NoError(t, json.Unmarshal(raw, &req))
				return captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
			},
		},
		{
			name: "Passkey登录",
			decode: func(raw []byte) service.CaptchaProof {
				var req passkeyBeginLoginRequest
				require.NoError(t, json.Unmarshal(raw, &req))
				return captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
			},
		},
		{
			name: "OAuth待处理账号发送邮箱验证码",
			decode: func(raw []byte) service.CaptchaProof {
				var req sendPendingOAuthVerifyCodeRequest
				require.NoError(t, json.Unmarshal(raw, &req))
				return captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
			},
		},
		{
			name: "OAuth待处理账号创建",
			decode: func(raw []byte) service.CaptchaProof {
				var req createPendingOAuthAccountRequest
				require.NoError(t, json.Unmarshal(raw, &req))
				return captchaProof(req.TurnstileToken, req.TencentCaptchaTicket, req.TencentCaptchaRandstr)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			proof := test.decode([]byte(payload))
			require.Equal(t, "ticket-value", proof.TencentTicket)
			require.Equal(t, "@rand-value", proof.TencentRandstr)
		})
	}
}
