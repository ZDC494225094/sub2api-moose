package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	RegistrationProofMinDifficulty     = 16
	RegistrationProofMaxDifficulty     = 24
	DefaultRegistrationProofDifficulty = 18
	registrationProofTTL               = 5 * time.Minute
)

var (
	ErrRegistrationProofFailed = infraerrors.BadRequest(
		"REGISTRATION_PROOF_FAILED",
		"registration security verification failed",
	)
	ErrRegistrationProofNotConfigured = infraerrors.ServiceUnavailable(
		"REGISTRATION_PROOF_NOT_CONFIGURED",
		"registration security verification is not configured",
	)
)

type RegistrationProofChallenge struct {
	Enabled    bool
	Challenge  string
	Difficulty int
	ExpiresAt  int64
}

type registrationProofPayload struct {
	Version    int    `json:"v"`
	IssuedAt   int64  `json:"iat"`
	ExpiresAt  int64  `json:"exp"`
	Difficulty int    `json:"d"`
	Nonce      string `json:"n"`
	EmailHash  string `json:"e"`
	IPHash     string `json:"i"`
}

func normalizeRegistrationProofDifficulty(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return DefaultRegistrationProofDifficulty
	}
	return clampRegistrationProofDifficulty(parsed)
}

func clampRegistrationProofDifficulty(value int) int {
	if value == 0 {
		return DefaultRegistrationProofDifficulty
	}
	if value < RegistrationProofMinDifficulty {
		return RegistrationProofMinDifficulty
	}
	if value > RegistrationProofMaxDifficulty {
		return RegistrationProofMaxDifficulty
	}
	return value
}

func (s *SettingService) IsRegistrationProofEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationProofEnabled)
	return err == nil && value == "true"
}

func (s *SettingService) GetRegistrationProofDifficulty(ctx context.Context) int {
	if s == nil || s.settingRepo == nil {
		return DefaultRegistrationProofDifficulty
	}
	value, err := s.settingRepo.GetValue(ctx, SettingKeyRegistrationProofDifficulty)
	if err != nil {
		return DefaultRegistrationProofDifficulty
	}
	return normalizeRegistrationProofDifficulty(value)
}

func (s *AuthService) CreateRegistrationProofChallenge(ctx context.Context, email, remoteIP string) (*RegistrationProofChallenge, error) {
	if s == nil || s.settingService == nil || !s.settingService.IsRegistrationProofEnabled(ctx) {
		return &RegistrationProofChallenge{Enabled: false}, nil
	}
	secret := s.registrationProofSecret()
	if len(secret) == 0 {
		return nil, ErrRegistrationProofNotConfigured
	}

	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return nil, ErrRegistrationProofNotConfigured.WithCause(err)
	}
	now := time.Now()
	difficulty := s.settingService.GetRegistrationProofDifficulty(ctx)
	payload := registrationProofPayload{
		Version:    1,
		IssuedAt:   now.Unix(),
		ExpiresAt:  now.Add(registrationProofTTL).Unix(),
		Difficulty: difficulty,
		Nonce:      base64.RawURLEncoding.EncodeToString(nonce),
		EmailHash:  registrationProofBindingHash(secret, "email", normalizeRegistrationProofEmail(email)),
		IPHash:     registrationProofBindingHash(secret, "ip", strings.TrimSpace(remoteIP)),
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrRegistrationProofNotConfigured.WithCause(err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := registrationProofSignature(secret, encodedPayload)
	challenge := encodedPayload + "." + base64.RawURLEncoding.EncodeToString(signature)

	return &RegistrationProofChallenge{
		Enabled:    true,
		Challenge:  challenge,
		Difficulty: difficulty,
		ExpiresAt:  payload.ExpiresAt,
	}, nil
}

func (s *AuthService) VerifyRegistrationProof(ctx context.Context, email, remoteIP, challenge, solution string) error {
	if s == nil || s.settingService == nil || !s.settingService.IsRegistrationProofEnabled(ctx) {
		return nil
	}
	secret := s.registrationProofSecret()
	if len(secret) == 0 {
		return ErrRegistrationProofNotConfigured
	}
	if len(challenge) == 0 || len(challenge) > 2048 || len(solution) == 0 || len(solution) > 20 {
		return ErrRegistrationProofFailed
	}
	if _, err := strconv.ParseUint(solution, 10, 64); err != nil {
		return ErrRegistrationProofFailed
	}

	parts := strings.Split(challenge, ".")
	if len(parts) != 2 {
		return ErrRegistrationProofFailed
	}
	providedSignature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ErrRegistrationProofFailed
	}
	expectedSignature := registrationProofSignature(secret, parts[0])
	if len(providedSignature) != len(expectedSignature) || subtle.ConstantTimeCompare(providedSignature, expectedSignature) != 1 {
		return ErrRegistrationProofFailed
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return ErrRegistrationProofFailed
	}
	var payload registrationProofPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return ErrRegistrationProofFailed
	}

	now := time.Now().Unix()
	if payload.Version != 1 || payload.IssuedAt > now+30 || payload.ExpiresAt < now || payload.ExpiresAt < payload.IssuedAt || payload.ExpiresAt-payload.IssuedAt > int64(registrationProofTTL/time.Second) {
		return ErrRegistrationProofFailed
	}
	if payload.Difficulty < RegistrationProofMinDifficulty || payload.Difficulty > RegistrationProofMaxDifficulty {
		return ErrRegistrationProofFailed
	}
	if !hmac.Equal(
		[]byte(payload.EmailHash),
		[]byte(registrationProofBindingHash(secret, "email", normalizeRegistrationProofEmail(email))),
	) || !hmac.Equal(
		[]byte(payload.IPHash),
		[]byte(registrationProofBindingHash(secret, "ip", strings.TrimSpace(remoteIP))),
	) {
		return ErrRegistrationProofFailed
	}

	digest := sha256.Sum256([]byte(challenge + ":" + solution))
	if !hasLeadingZeroBits(digest[:], payload.Difficulty) {
		return ErrRegistrationProofFailed
	}
	return nil
}

func (s *AuthService) registrationProofSecret() []byte {
	if s == nil || s.cfg == nil {
		return nil
	}
	secret := strings.TrimSpace(s.cfg.JWT.Secret)
	if secret == "" {
		return nil
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("sub2api-registration-proof-v1"))
	return mac.Sum(nil)
}

func registrationProofSignature(secret []byte, payload string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func registrationProofBindingHash(secret []byte, field, value string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(field))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizeRegistrationProofEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
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
