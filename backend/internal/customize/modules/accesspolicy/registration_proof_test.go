package accesspolicy

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strconv"
	"strings"
	"testing"
	"time"
)

func solveProof(t *testing.T, challenge string, difficulty int) string {
	t.Helper()
	for nonce := uint64(0); ; nonce++ {
		solution := strconv.FormatUint(nonce, 10)
		digest := sha256.Sum256([]byte(challenge + ":" + solution))
		if hasLeadingZeroBits(digest[:], difficulty) {
			return solution
		}
	}
}
func TestProofProtocolRoundTripAndBindings(t *testing.T) {
	secret := RegistrationProofSecret("  legacy-jwt-secret  ")
	require.Equal(t, RegistrationProofSecret("legacy-jwt-secret"), secret)
	require.Nil(t, RegistrationProofSecret("  "))
	challenge, err := CreateRegistrationProofChallenge(secret, 16, " User@Example.com ", " 127.0.0.1 ")
	require.NoError(t, err)
	require.True(t, challenge.Enabled)
	require.InDelta(t, time.Now().Add(5*time.Minute).Unix(), challenge.ExpiresAt, 2)
	solution := solveProof(t, challenge.Challenge, challenge.Difficulty)
	require.NoError(t, VerifyRegistrationProof(secret, "user@example.com", "127.0.0.1", challenge.Challenge, solution))
	for _, tc := range []struct {
		name, email, ip, token, solution string
		secret                           []byte
	}{
		{"email", "other@example.com", "127.0.0.1", challenge.Challenge, solution, secret},
		{"ip", "user@example.com", "127.0.0.2", challenge.Challenge, solution, secret},
		{"signature", "user@example.com", "127.0.0.1", challenge.Challenge + "x", solution, secret},
		{"key rotation", "user@example.com", "127.0.0.1", challenge.Challenge, solution, RegistrationProofSecret("another")},
		{"empty token", "user@example.com", "127.0.0.1", "", solution, secret},
		{"oversized token", "user@example.com", "127.0.0.1", strings.Repeat("a", 2049), solution, secret},
		{"negative nonce", "user@example.com", "127.0.0.1", challenge.Challenge, "-1", secret},
		{"overflow nonce", "user@example.com", "127.0.0.1", challenge.Challenge, "18446744073709551616", secret},
		{"oversized nonce", "user@example.com", "127.0.0.1", challenge.Challenge, strings.Repeat("1", 21), secret},
		{"empty nonce", "user@example.com", "127.0.0.1", challenge.Challenge, "", secret},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.ErrorIs(t, VerifyRegistrationProof(tc.secret, tc.email, tc.ip, tc.token, tc.solution), ErrRegistrationProofFailed)
		})
	}
	_, err = CreateRegistrationProofChallenge(nil, 16, "user@example.com", "127.0.0.1")
	require.ErrorIs(t, err, ErrRegistrationProofNotConfigured)
	require.ErrorIs(t, VerifyRegistrationProof(nil, "", "", "", ""), ErrRegistrationProofNotConfigured)
}
func TestProofRejectsInvalidSignedPayloads(t *testing.T) {
	secret := RegistrationProofSecret("test-secret")
	now := time.Now().Unix()
	valid := registrationProofPayload{Version: 1, IssuedAt: now, ExpiresAt: now + 300, Difficulty: 16, Nonce: "legacy-nonce", EmailHash: registrationProofBindingHash(secret, "email", "user@example.com"), IPHash: registrationProofBindingHash(secret, "ip", "127.0.0.1")}
	cases := []struct {
		name   string
		mutate func(*registrationProofPayload)
	}{
		{"version", func(p *registrationProofPayload) { p.Version = 2 }},
		{"future", func(p *registrationProofPayload) { p.IssuedAt = now + 60 }},
		{"expired", func(p *registrationProofPayload) { p.IssuedAt = now - 600; p.ExpiresAt = now - 300 }},
		{"inverted", func(p *registrationProofPayload) { p.ExpiresAt = p.IssuedAt - 1 }},
		{"ttl", func(p *registrationProofPayload) { p.ExpiresAt = now + 301 }},
		{"low difficulty", func(p *registrationProofPayload) { p.Difficulty = 15 }},
		{"high difficulty", func(p *registrationProofPayload) { p.Difficulty = 25 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := valid
			tc.mutate(&payload)
			raw, err := json.Marshal(payload)
			require.NoError(t, err)
			encoded := base64.RawURLEncoding.EncodeToString(raw)
			token := encoded + "." + base64.RawURLEncoding.EncodeToString(registrationProofSignature(secret, encoded))
			require.ErrorIs(t, VerifyRegistrationProof(secret, "user@example.com", "127.0.0.1", token, "0"), ErrRegistrationProofFailed)
		})
	}
}
func TestProofDifficultyCompatibility(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want int
	}{{"", 18}, {"bad", 18}, {"0", 18}, {"-1", 16}, {"15", 16}, {" 20 ", 20}, {"25", 24}} {
		require.Equal(t, tc.want, NormalizeRegistrationProofDifficulty(tc.raw))
	}
}
