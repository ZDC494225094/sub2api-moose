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
