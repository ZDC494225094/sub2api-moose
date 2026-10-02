package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

// Frozen pre-extraction payload/signature helpers. Production must use the module.
type registrationProofPayload struct {
	Version    int    `json:"v"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
	Difficulty int    `json:"d"`
	Nonce      string `json:"n"`
	EmailHash  string `json:"e"`
	IPHash     string `json:"i"`
}

func registrationProofSignature(secret []byte, payload string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func hasLeadingZeroBits(value []byte, bits int) bool {
	fullBytes := bits / 8
	remainingBits := bits % 8
	for i := 0; i < fullBytes; i++ {
		if value[i] != 0 {
			return false
		}
	}
	return remainingBits == 0 || value[fullBytes]>>(8-remainingBits) == 0
}

func TestRegistrationProofAcceptsLegacyTokenRegardlessOfBusinessSwitch(t *testing.T) {
	// Use the old v1 implementation to sign, not the extracted module's constructor.
	mac := hmac.New(sha256.New, []byte(strings.Repeat("a", 32)))
	_, _ = mac.Write([]byte("sub2api-registration-proof-v1"))
	secret := mac.Sum(nil)
	binding := func(field, value string) string {
		m := hmac.New(sha256.New, secret)
		_, _ = m.Write([]byte(field))
		_, _ = m.Write([]byte{0})
		_, _ = m.Write([]byte(value))
		return hex.EncodeToString(m.Sum(nil))
	}
	now := time.Now().Unix()
	payload := registrationProofPayload{Version: 1, IssuedAt: now, ExpiresAt: now + 300, Difficulty: 16, Nonce: "pre-extraction-v1", EmailHash: binding("email", "user@example.com"), IPHash: binding("ip", "127.0.0.1")}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	token := encoded + "." + base64.RawURLEncoding.EncodeToString(registrationProofSignature(secret, encoded))
	solution := solveRegistrationProofForTest(t, token, 16)
	for _, state := range []string{"true", "false", "invalid"} {
		t.Run(state, func(t *testing.T) {
			svc := newRegistrationProofTestService(map[string]string{SettingKeyRegistrationProofEnabled: "true", "custom_extensions.access-policy.enabled": state})
			require.NoError(t, svc.VerifyRegistrationProof(context.Background(), "user@example.com", "127.0.0.1", token, solution))
			require.ErrorIs(t, svc.VerifyRegistrationProof(context.Background(), "attacker@example.com", "127.0.0.1", token, solution), ErrRegistrationProofFailed)
			require.ErrorIs(t, svc.VerifyRegistrationProof(context.Background(), "user@example.com", "127.0.0.1", "", ""), ErrRegistrationProofFailed)
		})
	}
}
