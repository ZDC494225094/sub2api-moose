package service

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func newRegistrationProofTestService(settings map[string]string) *AuthService {
	cfg := &config.Config{JWT: config.JWTConfig{Secret: strings.Repeat("a", 32)}}
	return &AuthService{
		cfg:            cfg,
		settingService: NewSettingService(&registrationProofSettingRepo{values: settings}, cfg),
	}
}

type registrationProofSettingRepo struct {
	values map[string]string
}

func (r *registrationProofSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *registrationProofSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *registrationProofSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *registrationProofSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *registrationProofSettingRepo) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		r.values[key] = value
	}
	return nil
}

func (r *registrationProofSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return r.values, nil
}

func (r *registrationProofSettingRepo) Delete(_ context.Context, key string) error {
	delete(r.values, key)
	return nil
}

func solveRegistrationProofForTest(t *testing.T, challenge string, difficulty int) string {
	t.Helper()
	for nonce := uint64(0); ; nonce++ {
		solution := strconv.FormatUint(nonce, 10)
		digest := sha256.Sum256([]byte(challenge + ":" + solution))
		if hasLeadingZeroBits(digest[:], difficulty) {
			return solution
		}
	}
}

func TestRegistrationProofDisabledSkipsChallengeAndVerification(t *testing.T) {
	svc := newRegistrationProofTestService(map[string]string{
		SettingKeyRegistrationProofEnabled: "false",
	})

	challenge, err := svc.CreateRegistrationProofChallenge(context.Background(), "user@example.com", "127.0.0.1")
	require.NoError(t, err)
	require.False(t, challenge.Enabled)
	require.NoError(t, svc.VerifyRegistrationProof(context.Background(), "user@example.com", "127.0.0.1", "", ""))
}

func TestRegistrationProofValidatesSignedEmailAndIPBoundSolution(t *testing.T) {
	svc := newRegistrationProofTestService(map[string]string{
		SettingKeyRegistrationProofEnabled:    "true",
		SettingKeyRegistrationProofDifficulty: "16",
	})
	ctx := context.Background()
	challenge, err := svc.CreateRegistrationProofChallenge(ctx, "User@Example.com", "127.0.0.1")
	require.NoError(t, err)
	require.True(t, challenge.Enabled)
	solution := solveRegistrationProofForTest(t, challenge.Challenge, challenge.Difficulty)

	require.NoError(t, svc.VerifyRegistrationProof(ctx, "user@example.com", "127.0.0.1", challenge.Challenge, solution))
	require.ErrorIs(t, svc.VerifyRegistrationProof(ctx, "other@example.com", "127.0.0.1", challenge.Challenge, solution), ErrRegistrationProofFailed)
	require.ErrorIs(t, svc.VerifyRegistrationProof(ctx, "user@example.com", "127.0.0.2", challenge.Challenge, solution), ErrRegistrationProofFailed)
	require.ErrorIs(t, svc.VerifyRegistrationProof(ctx, "user@example.com", "127.0.0.1", challenge.Challenge+"x", solution), ErrRegistrationProofFailed)
}

func TestRegistrationProofRejectsExpiredChallenge(t *testing.T) {
	svc := newRegistrationProofTestService(map[string]string{
		SettingKeyRegistrationProofEnabled:    "true",
		SettingKeyRegistrationProofDifficulty: "16",
	})
	ctx := context.Background()
	challenge, err := svc.CreateRegistrationProofChallenge(ctx, "user@example.com", "127.0.0.1")
	require.NoError(t, err)

	parts := strings.Split(challenge.Challenge, ".")
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	require.NoError(t, err)
	var payload registrationProofPayload
	require.NoError(t, json.Unmarshal(payloadBytes, &payload))
	payload.IssuedAt = time.Now().Add(-10 * time.Minute).Unix()
	payload.ExpiresAt = time.Now().Add(-5 * time.Minute).Unix()
	payloadBytes, err = json.Marshal(payload)
	require.NoError(t, err)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := registrationProofSignature(svc.registrationProofSecret(), encodedPayload)
	expiredChallenge := encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature)
	solution := solveRegistrationProofForTest(t, expiredChallenge, payload.Difficulty)

	require.ErrorIs(t, svc.VerifyRegistrationProof(ctx, "user@example.com", "127.0.0.1", expiredChallenge, solution), ErrRegistrationProofFailed)
}
