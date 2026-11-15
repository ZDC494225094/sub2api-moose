package accesspolicy

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHomepageTicketsAreBoundEncryptedAndExpire(t *testing.T) {
	g, err := NewHomepageGate("homepage-test-secret-with-sufficient-length")
	require.NoError(t, err)
	now := time.Now()
	g.now = func() time.Time { return now }
	token, browserID, answer, err := g.NewChallenge("browser-a")
	require.NoError(t, err)
	require.Len(t, answer, 6)
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err)
	require.NotContains(t, string(decoded), answer)
	_, err = g.ChallengeID(token, browserID, "browser-a")
	require.NoError(t, err)
	_, err = g.ChallengeID(token, "other-browser", "browser-a")
	require.Error(t, err)
	_, err = g.ChallengeID(token, browserID, "browser-b")
	require.Error(t, err)
	require.True(t, g.CheckAnswer(token, answer))
	require.False(t, g.CheckAnswer(token, "wrong"))
	decoded[len(decoded)/2] ^= 1
	_, err = g.ChallengeID(base64.RawURLEncoding.EncodeToString(decoded), browserID, "browser-a")
	require.Error(t, err)
	clearance, err := g.Clearance("browser-a")
	require.NoError(t, err)
	require.True(t, g.HasClearance(clearance, "browser-a"))
	require.False(t, g.HasClearance(clearance, "browser-b"))
	require.False(t, g.HasClearance(token, "browser-a"))
	_, err = g.ChallengeID(clearance, browserID, "browser-a")
	require.Error(t, err)
	now = now.Add(HomepageChallengeTTL)
	require.False(t, g.CheckAnswer(token, answer))
	require.True(t, g.HasClearance(clearance, "browser-a"))
	now = now.Add(HomepageClearanceTTL)
	require.False(t, g.HasClearance(clearance, "browser-a"))
}

func TestHomepagePageHasNoRemoteAssetsOrReadableAnswer(t *testing.T) {
	body, err := RenderHomepageGate("encrypted-challenge", "482957", "/?aff=example#plans", "test-nonce", "")
	require.NoError(t, err)
	html := string(body)
	require.Less(t, len(body), 16000)
	for _, forbidden := range []string{"/assets/", "modulepreload", "https://", "482957", "<svg", "type=\"module\""} {
		require.NotContains(t, html, forbidden)
	}
	require.Contains(t, html, "data:image/png;base64,")
	require.Contains(t, html, `nonce="test-nonce"`)
	require.Contains(t, html, `action="/_homepage/verify"`)
}

func TestHomepageReturnPathCannotRedirectOffsite(t *testing.T) {
	for _, value := range []string{"https://evil.test", "//evil.test", "/%2fevil.test", "/\\evil.test", "/login", "/\r\nX-Test:x", strings.Repeat("a", 5000)} {
		require.Equal(t, "/", HomepageReturnPath(value), value)
	}
	for _, value := range []string{"/", "/home/", "/?aff=abc#plans", "/home?x=1&y=2"} {
		require.Equal(t, value, HomepageReturnPath(value))
	}
}

func TestHomepageTurnstileReceiptRejectsImageTamperingAndConfigurationChanges(t *testing.T) {
	g, err := NewHomepageGate("homepage-test-secret-with-sufficient-length")
	require.NoError(t, err)
	now := time.Now()
	g.now = func() time.Time { return now }
	config := HomepageTurnstileConfig{SiteKey: "site", SecretKey: "secret"}
	token, err := g.TurnstileClearance("browser-id", "agent", config)
	require.NoError(t, err)
	require.Positive(t, g.TurnstileClearanceExpires(token, "browser-id", "agent", config))
	for _, tc := range []struct {
		token, browser, agent string
		config                HomepageTurnstileConfig
	}{
		{token, "", "agent", config},
		{token, "other-browser", "agent", config},
		{token, "browser-id", "other-agent", config},
		{token, "browser-id", "agent", HomepageTurnstileConfig{SiteKey: "site", SecretKey: "rotated"}},
		{token + "invalid", "browser-id", "agent", config},
	} {
		require.Zero(t, g.TurnstileClearanceExpires(tc.token, tc.browser, tc.agent, tc.config))
	}
	image, err := g.Clearance("agent")
	require.NoError(t, err)
	require.Zero(t, g.TurnstileClearanceExpires(image, "browser-id", "agent", config))
	require.False(t, g.HasClearance(token, "agent"))
	now = now.Add(HomepageClearanceTTL)
	require.Zero(t, g.TurnstileClearanceExpires(token, "browser-id", "agent", config))
}

func TestHomepageTurnstilePageOnlyLoadsProviderAndNeverImageAnswer(t *testing.T) {
	body, err := RenderHomepageGateWithTurnstile("challenge", "123456", "/?aff=keep#plans", "nonce", "", "site-key", "browser-id")
	require.NoError(t, err)
	html := string(body)
	for _, forbidden := range []string{`name="answer"`, "data:image/png", "/assets/", "123456"} {
		require.NotContains(t, html, forbidden)
	}
	require.Contains(t, html, "https://challenges.cloudflare.com/turnstile/v0/api.js")
	require.Contains(t, html, `action:"homepage"`)
	require.Contains(t, html, `data-binding="browser-id"`)
	require.Contains(t, html, `requestSubmit()`)
}
