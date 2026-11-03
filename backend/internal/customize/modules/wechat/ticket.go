package wechat

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
)

// bindTicketTTL bounds how long an OAuth result can be redeemed by /wechat/bind.
// It covers a login detour while keeping a leaked page URL short-lived.
const bindTicketTTL = 15 * time.Minute

var errInvalidBindTicket = errors.New("invalid or expired wechat bind ticket")

// IssueBindTicket signs the openid returned by this server's code exchange, so
// that /wechat/bind never trusts an openid typed by the client.
func (s *Service) IssueBindTicket(openid string, now time.Time) string {
	payload := openid + "." + strconv.FormatInt(now.Add(bindTicketTTL).Unix(), 10)
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(s.bindTicketMAC(payload))
}

// VerifyBindTicket returns the signed openid when the ticket is authentic and unexpired.
func (s *Service) VerifyBindTicket(ticket string, now time.Time) (string, error) {
	if !s.Enabled() {
		return "", errInvalidBindTicket
	}
	encodedPayload, encodedMAC, ok := strings.Cut(strings.TrimSpace(ticket), ".")
	if !ok {
		return "", errInvalidBindTicket
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return "", errInvalidBindTicket
	}
	mac, err := base64.RawURLEncoding.DecodeString(encodedMAC)
	if err != nil || !hmac.Equal(mac, s.bindTicketMAC(string(payload))) {
		return "", errInvalidBindTicket
	}
	openid, expires, ok := strings.Cut(string(payload), ".")
	if !ok || !validOpenID(openid) {
		return "", errInvalidBindTicket
	}
	expiresAt, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || now.Unix() > expiresAt {
		return "", errInvalidBindTicket
	}
	return openid, nil
}

// The AppSecret never leaves the server, so a key derived from it cannot be
// reproduced by a client. Rotating the secret invalidates outstanding tickets.
func (s *Service) bindTicketMAC(payload string) []byte {
	mac := hmac.New(sha256.New, []byte("sub2api-wechat-oa-bind-ticket:"+s.cfg.AppSecret))
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
