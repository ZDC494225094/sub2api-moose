package accesspolicy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"
)

const (
	HomepageCaptchaEnabledKey = "homepage_captcha_enabled"
	HomepageChallengeTTL      = 5 * time.Minute
	HomepageClearanceTTL      = 24 * time.Hour
	HomepageChallengeCookie   = "sub2api_home_challenge"
	HomepageClearanceCookie   = "sub2api_home_clearance"
)

var ErrHomepageChallenge = errors.New("invalid or expired homepage verification")

// HomepageGate uses a separate encryption key/domain from authentication tokens.
// No captcha answer or guessable answer digest is exposed to the browser.
type HomepageGate struct {
	aead cipher.AEAD
	key  []byte
	now  func() time.Time
}

type homepageTicket struct {
	Kind    string `json:"k"`
	Expires int64  `json:"e"`
	ID      string `json:"n,omitempty"`
	Answer  string `json:"a,omitempty"`
	Binding string `json:"b"`
}

func NewHomepageGate(secret string) (*HomepageGate, error) {
	if len(secret) < 16 {
		return nil, ErrHomepageChallenge
	}
	key := sha256.Sum256([]byte("sub2api/homepage-gate/v1/" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &HomepageGate{aead: aead, key: key[:], now: time.Now}, nil
}

func (g *HomepageGate) binding(value string) string {
	h := hmac.New(sha256.New, g.key)
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}

func (g *HomepageGate) seal(ticket homepageTicket) (string, error) {
	raw, err := json.Marshal(ticket)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, g.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := g.aead.Seal(nonce, nonce, raw, []byte("homepage/v1"))
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (g *HomepageGate) open(token, kind string, ttl time.Duration) (homepageTicket, error) {
	var ticket homepageTicket
	if len(token) > 2048 {
		return ticket, ErrHomepageChallenge
	}
	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(data) < g.aead.NonceSize() {
		return ticket, ErrHomepageChallenge
	}
	n := g.aead.NonceSize()
	raw, err := g.aead.Open(nil, data[:n], data[n:], []byte("homepage/v1"))
	if err != nil || json.Unmarshal(raw, &ticket) != nil {
		return ticket, ErrHomepageChallenge
	}
	now := g.now().Unix()
	if ticket.Kind != kind || ticket.Expires <= now || ticket.Expires > now+int64(ttl/time.Second) {
		return ticket, ErrHomepageChallenge
	}
	return ticket, nil
}

func (g *HomepageGate) NewChallenge(userAgent string) (token, browserID, answer string, err error) {
	id := make([]byte, 24)
	if _, err = io.ReadFull(rand.Reader, id); err != nil {
		return
	}
	browserID = base64.RawURLEncoding.EncodeToString(id)
	// Rejection sampling avoids modulo bias in the six decimal digits.
	for len(answer) < 6 {
		var b [1]byte
		if _, err = io.ReadFull(rand.Reader, b[:]); err != nil {
			return
		}
		if b[0] < 250 {
			answer += string(rune('0' + b[0]%10))
		}
	}
	token, err = g.seal(homepageTicket{Kind: "challenge", Expires: g.now().Add(HomepageChallengeTTL).Unix(), ID: browserID, Answer: answer, Binding: g.binding(browserID + "\n" + userAgent)})
	return
}

// ChallengeID validates browser binding before the caller atomically consumes the
// challenge in shared storage. Every submission, including a wrong guess, consumes it.
func (g *HomepageGate) ChallengeID(token, browserID, userAgent string) (string, error) {
	ticket, err := g.open(token, "challenge", HomepageChallengeTTL)
	if err != nil || browserID == "" || ticket.ID != browserID || !hmac.Equal([]byte(ticket.Binding), []byte(g.binding(browserID+"\n"+userAgent))) {
		return "", ErrHomepageChallenge
	}
	return ticket.ID, nil
}

func (g *HomepageGate) CheckAnswer(token, answer string) bool {
	ticket, err := g.open(token, "challenge", HomepageChallengeTTL)
	return err == nil && hmac.Equal([]byte(ticket.Answer), []byte(strings.TrimSpace(answer)))
}

func (g *HomepageGate) Clearance(userAgent string) (string, error) {
	return g.seal(homepageTicket{Kind: "clearance", Expires: g.now().Add(HomepageClearanceTTL).Unix(), Binding: g.binding(userAgent)})
}

func (g *HomepageGate) HasClearance(token, userAgent string) bool {
	ticket, err := g.open(token, "clearance", HomepageClearanceTTL)
	return err == nil && hmac.Equal([]byte(ticket.Binding), []byte(g.binding(userAgent)))
}

// Turnstile receipts are distinct from image clearance and tied to both the
// browser's HttpOnly binding cookie and the active provider credentials.
type HomepageTurnstileConfig struct {
	SiteKey, SecretKey string
}

func (g *HomepageGate) TurnstileClearance(browserID, userAgent string, config HomepageTurnstileConfig) (string, error) {
	return g.seal(homepageTicket{Kind: "turnstile-clearance", Expires: g.now().Add(HomepageClearanceTTL).Unix(), Binding: g.binding(browserID + "\n" + userAgent + "\n" + config.SiteKey + "\n" + config.SecretKey)})
}

func (g *HomepageGate) TurnstileClearanceExpires(token, browserID, userAgent string, config HomepageTurnstileConfig) int64 {
	ticket, err := g.open(token, "turnstile-clearance", HomepageClearanceTTL)
	if err != nil || browserID == "" || config.SiteKey == "" || config.SecretKey == "" || !hmac.Equal([]byte(ticket.Binding), []byte(g.binding(browserID+"\n"+userAgent+"\n"+config.SiteKey+"\n"+config.SecretKey))) {
		return 0
	}
	return ticket.Expires
}

func IsHomepagePath(path string) bool {
	return path == "/" || path == "/home" || path == "/home/" || path == "/index.html"
}

func HomepageReturnPath(raw string) string {
	if len(raw) > 4096 || strings.ContainsAny(raw, "\r\n\\") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.IsAbs() || u.Host != "" || u.Opaque != "" || !IsHomepagePath(u.Path) {
		return "/"
	}
	// Re-serialize the decoded, allowlisted path; do not retain an escaped //host.
	u.RawPath = ""
	return u.String()
}
