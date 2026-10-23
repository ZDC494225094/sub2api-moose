// Package wechat implements WeChat Official Account integration for the
// in-site H5 lottery page: webpage authorization (snsapi_base) to obtain the
// visitor's openid, openid <-> sub2api user binding, and pushing the lottery
// result back to the OA conversation via the customer-service message API.
//
// Configuration comes from environment variables (never hardcode secrets):
//   WECHAT_OA_APPID     - 公众号 AppID
//   WECHAT_OA_APPSECRET - 公众号 AppSecret
//
// The server's outbound IP must be added to the OA backend IP whitelist,
// otherwise access_token calls fail.
package wechat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	wechatAuthorizeURL   = "https://open.weixin.qq.com/connect/oauth2/authorize"
	wechatSnsTokenURL    = "https://api.weixin.qq.com/sns/oauth2/access_token"
	wechatCGITokenURL    = "https://api.weixin.qq.com/cgi-bin/token"
	wechatCustomSendURL  = "https://api.weixin.qq.com/cgi-bin/message/custom/send"
	httpTimeout          = 15 * time.Second
	accessTokenSkew      = 5 * time.Minute
	maxMessageRunes      = 1800
)

// Config carries the Official Account credentials.
type Config struct {
	AppID     string
	AppSecret string
}

// LoadConfigFromEnv reads WECHAT_OA_APPID / WECHAT_OA_APPSECRET.
func LoadConfigFromEnv() Config {
	return Config{
		AppID:     strings.TrimSpace(os.Getenv("WECHAT_OA_APPID")),
		AppSecret: strings.TrimSpace(os.Getenv("WECHAT_OA_APPSECRET")),
	}
}

// Enabled reports whether the integration is configured.
func (c Config) Enabled() bool {
	return c.AppID != "" && c.AppSecret != ""
}

// Service talks to the WeChat API.
type Service struct {
	cfg    Config
	client *http.Client

	mu               sync.Mutex
	cachedToken      string
	cachedTokenUntil time.Time
}

// NewService builds the service; it works (reporting disabled) even when the
// config is empty so callers don't need nil checks.
func NewService(cfg Config) *Service {
	return &Service{cfg: cfg, client: &http.Client{Timeout: httpTimeout}}
}

// Enabled reports whether WeChat calls can be made.
func (s *Service) Enabled() bool {
	return s != nil && s.cfg.Enabled()
}

// BuildAuthorizeURL returns the WeChat webpage-authorization URL (snsapi_base,
// silent) for the given backend callback URL.
func (s *Service) BuildAuthorizeURL(callbackURL, state string) (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("wechat oa is not configured")
	}
	q := url.Values{}
	q.Set("appid", s.cfg.AppID)
	q.Set("redirect_uri", callbackURL)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_base")
	if state != "" {
		q.Set("state", state)
	}
	return wechatAuthorizeURL + "?" + q.Encode() + "#wechat_redirect", nil
}

type snsTokenResponse struct {
	AccessToken string `json:"access_token"`
	OpenID      string `json:"openid"`
	Scope       string `json:"scope"`
	ErrCode     int64  `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// ExchangeCode trades the authorization code for the visitor's openid.
func (s *Service) ExchangeCode(ctx context.Context, code string) (string, error) {
	if !s.Enabled() {
		return "", fmt.Errorf("wechat oa is not configured")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return "", fmt.Errorf("empty wechat oauth code")
	}
	q := url.Values{}
	q.Set("appid", s.cfg.AppID)
	q.Set("secret", s.cfg.AppSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")
	var resp snsTokenResponse
	if err := s.getJSON(ctx, wechatSnsTokenURL+"?"+q.Encode(), &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("wechat sns token failed: %d %s", resp.ErrCode, resp.ErrMsg)
	}
	if resp.OpenID == "" {
		return "", fmt.Errorf("wechat sns token returned empty openid")
	}
	return resp.OpenID, nil
}

type cgiTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	ErrCode     int64  `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

// getAccessToken returns a cached cgi-bin access_token.
func (s *Service) getAccessToken(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.cachedToken != "" && time.Now().Before(s.cachedTokenUntil) {
		token := s.cachedToken
		s.mu.Unlock()
		return token, nil
	}
	s.mu.Unlock()

	q := url.Values{}
	q.Set("grant_type", "client_credential")
	q.Set("appid", s.cfg.AppID)
	q.Set("secret", s.cfg.AppSecret)
	var resp cgiTokenResponse
	if err := s.getJSON(ctx, wechatCGITokenURL+"?"+q.Encode(), &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("wechat cgi token failed: %d %s (check IP whitelist)", resp.ErrCode, resp.ErrMsg)
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("wechat cgi token returned empty token")
	}
	ttl := time.Duration(resp.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	ttl -= accessTokenSkew
	if ttl <= 0 {
		ttl = time.Minute
	}
	s.mu.Lock()
	s.cachedToken = resp.AccessToken
	s.cachedTokenUntil = time.Now().Add(ttl)
	s.mu.Unlock()
	return resp.AccessToken, nil
}

type customSendResponse struct {
	ErrCode int64  `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// SendTextMessage pushes a customer-service text message to the OA
// conversation. The user must have interacted with the OA within 48 hours
// (tapping the custom menu counts).
func (s *Service) SendTextMessage(ctx context.Context, openid, text string) error {
	if !s.Enabled() {
		return fmt.Errorf("wechat oa is not configured")
	}
	openid = strings.TrimSpace(openid)
	if openid == "" {
		return fmt.Errorf("empty openid")
	}
	runes := []rune(text)
	if len(runes) > maxMessageRunes {
		text = string(runes[:maxMessageRunes])
	}
	token, err := s.getAccessToken(ctx)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{
		"touser":  openid,
		"msgtype": "text",
		"text":    map[string]string{"content": text},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wechatCustomSendURL+"?access_token="+url.QueryEscape(token), strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	httpResp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = httpResp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return err
	}
	var resp customSendResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("wechat custom send bad response: %s", truncate(string(body), 200))
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("wechat custom send failed: %d %s", resp.ErrCode, resp.ErrMsg)
	}
	return nil
}

func (s *Service) getJSON(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	httpResp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = httpResp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("wechat api bad response: %s", truncate(string(body), 200))
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
