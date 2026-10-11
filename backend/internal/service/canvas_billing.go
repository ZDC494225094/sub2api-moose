package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"net/url"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/google/uuid"
)

const CanvasBillingHeader = "X-Sub2api-Canvas-Charge"

var ErrCanvasChargeConflict = errors.New("画布任务编号已被使用，请使用新的任务编号")
var ErrCanvasQuotaUnavailable = errors.New("订阅剩余额度不足或计费窗口已变化，请稍后重试")
var ErrCanvasChargeUnavailable = errors.New("画布预扣服务暂不可用，请稍后重试")

// CanvasCharge never contains the bearer capability. It is safe to persist, but
// must not be accepted from client JSON or used without API-key authentication.
type CanvasCharge struct {
	ID                                                      string
	UserID, APIKeyID, GroupID                               int64
	SubscriptionID                                          *int64
	RunID, PayloadHash, Mode, Model, Status                 string
	ReservedCost, ActualCost, ChargedCost                   float64
	UsageRecorded                                           bool
	DailyWindowStart, WeeklyWindowStart, MonthlyWindowStart *time.Time
	CreatedAt                                               time.Time
}
type CanvasChargeReservation struct {
	CanvasCharge
	TokenHash string
}
type CanvasBillingRepository interface {
	ReserveCanvasCharge(context.Context, *CanvasChargeReservation) (*CanvasCharge, bool, error)
	AuthenticateCanvasCharge(context.Context, string, int64) (*CanvasCharge, error)
	GetCanvasCharge(context.Context, int64, string) (*CanvasCharge, error)
	FinishCanvasCharge(context.Context, string, bool) (*CanvasCharge, error)
	RenewCanvasCharge(context.Context, string) (bool, error)
	ExpiredCanvasCharges(context.Context) ([]CanvasCharge, error)
	FailExpiredCanvasCharge(context.Context, string) (*CanvasCharge, error)
}

type canvasChargeContextKey struct{}

func WithCanvasCharge(ctx context.Context, charge *CanvasCharge) context.Context {
	if charge == nil {
		return ctx
	}
	return context.WithValue(ctx, canvasChargeContextKey{}, charge)
}
func CanvasChargeFromContext(ctx context.Context) *CanvasCharge {
	if ctx == nil {
		return nil
	}
	c, _ := ctx.Value(canvasChargeContextKey{}).(*CanvasCharge)
	return c
}
func canvasTokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// A random per-run capability is generated only on the server. Knowing a run ID
// or supplying a billing header is NOT sufficient to bypass affordability checks.
func (s *APIKeyService) AuthenticateCanvasCharge(ctx context.Context, key *APIKey, token string) (context.Context, error) {
	if token == "" {
		return ctx, nil
	}
	if s.canvasBillingRepo == nil || !IsCanvasManagedAPIKey(key) || len(token) > 128 {
		return ctx, ErrCanvasChargeUnavailable
	}
	charge, err := s.canvasBillingRepo.AuthenticateCanvasCharge(ctx, canvasTokenHash(token), key.ID)
	if err != nil || charge == nil || charge.UserID != key.UserID || charge.Status != "reserved" {
		return ctx, ErrCanvasChargeUnavailable
	}
	return WithCanvasCharge(ctx, charge), nil
}

type CanvasBillingService struct {
	repo          CanvasBillingRepository
	keys          *APIKeyService
	subscriptions *SubscriptionService
	gateway       *GatewayService
	cache         *BillingCacheService
	cfg           *config.Config
}

func NewCanvasBillingService(repo CanvasBillingRepository, keys *APIKeyService, subscriptions *SubscriptionService, gateway *GatewayService, cache *BillingCacheService, cfg *config.Config) *CanvasBillingService {
	return &CanvasBillingService{repo: repo, keys: keys, subscriptions: subscriptions, gateway: gateway, cache: cache, cfg: cfg}
}

func canvasChargeEstimate(r PlaygroundRunRequest) InflightEstimateRequest {
	// Only textual input contributes to a token estimate, not API keys/base64 assets.
	input, _ := json.Marshal(struct {
		Prompt   string
		Messages []PlaygroundRunChatMessage
	}{r.Prompt, r.Messages})
	e := InflightEstimateRequest{Model: r.Model, BodyBytes: len(input), Units: 1}
	if r.MaxTokens != nil {
		e.MaxTokens = *r.MaxTokens
	}
	switch r.Mode {
	case "image":
		e.Kind = InflightEstimateImage
		e.Units = min(4, max(1, r.N))
	case "video":
		e.Kind = InflightEstimateVideo
		e.VideoResolution = r.Resolution
		e.VideoDurationSeconds = r.Duration
		if r.Platform == PlatformGemini {
			e.Units = max(1, r.N)
		}
	case "audio":
		e.Kind = InflightEstimateAudio
		e.AudioMode = "tts"
		e.AudioUnits = float64(utf8.RuneCountInString(r.Prompt)) / 1_000_000
	}
	return e
}

func (s *CanvasBillingService) reserve(ctx context.Context, key *APIKey, request PlaygroundRunRequest) (*CanvasCharge, string, bool, error) {
	if s == nil || s.repo == nil || s.keys == nil || key == nil || !IsCanvasManagedAPIKey(key) {
		return nil, "", false, ErrCanvasChargeUnavailable
	}
	// Repeat submissions are resolved before affordability checks: a task that
	// consumed the last available dollar must still be retrievable/idempotent.
	hashRequest := request
	hashRequest.APIKey = ""
	hashRequest.ID = ""
	payload, err := json.Marshal(hashRequest)
	if err != nil {
		return nil, "", false, err
	}
	hash := HashUsageRequestPayload(payload)
	existing, err := s.repo.GetCanvasCharge(ctx, key.UserID, request.ID)
	if err != nil {
		return nil, "", false, err
	}
	if existing != nil {
		if existing.PayloadHash != hash || existing.APIKeyID != key.ID {
			return nil, "", false, ErrCanvasChargeConflict
		}
		return existing, "", false, nil
	}
	selection, err := s.keys.SelectUsableGroupForAPIKey(ctx, key, s.subscriptions)
	if err != nil {
		return nil, "", false, err
	}
	var sub *UserSubscription
	if selection != nil {
		sub = selection.Subscription
	}
	if key.Group == nil || key.User == nil || !key.User.IsActive() || !key.Group.IsActive() || !key.IsActive() || key.IsExpired() || key.IsQuotaExhausted() {
		return nil, "", false, ErrCanvasChargeUnavailable
	}
	simple := s.cfg != nil && s.cfg.RunMode == config.RunModeSimple
	if simple {
		sub = nil
	}
	if !simple && key.Group.IsSubscriptionType() {
		if s.subscriptions == nil {
			return nil, "", false, ErrCanvasChargeUnavailable
		}
		if sub == nil {
			sub, err = s.subscriptions.GetActiveSubscription(ctx, key.UserID, key.Group.ID)
		}
		if err != nil {
			return nil, "", false, err
		}
		sub, err = s.subscriptions.EnsureWindowMaintenance(ctx, sub)
		if err != nil {
			return nil, "", false, err
		}
		if _, err = s.subscriptions.ValidateAndCheckLimits(sub, key.Group); err != nil {
			return nil, "", false, err
		}
	}
	amount, priced := 0.0, true
	if !simple {
		amount, priced = s.gateway.EstimateInflightReservation(ctx, key, canvasChargeEstimate(request))
	}
	if !priced || amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return nil, "", false, errors.New("无法确认该模型的预扣价格，请先配置模型价格")
	}
	amount = QuantizeUsageBillingAmount(amount)
	token := uuid.NewString() + uuid.NewString()
	c := &CanvasChargeReservation{CanvasCharge: CanvasCharge{ID: uuid.NewString(), UserID: key.UserID, APIKeyID: key.ID, GroupID: key.Group.ID, RunID: request.ID, PayloadHash: hash, Mode: request.Mode, Model: request.Model, ReservedCost: amount}, TokenHash: canvasTokenHash(token)}
	if sub != nil {
		c.SubscriptionID = &sub.ID
		c.DailyWindowStart, c.WeeklyWindowStart, c.MonthlyWindowStart = sub.DailyWindowStart, sub.WeeklyWindowStart, sub.MonthlyWindowStart
	}
	charge, created, err := s.repo.ReserveCanvasCharge(ctx, c)
	if err != nil {
		return nil, "", false, err
	}
	s.invalidate(ctx, charge)
	return charge, token, created, nil
}

func (s *CanvasBillingService) invalidate(ctx context.Context, c *CanvasCharge) {
	if c == nil {
		return
	}
	invalidateCanvasChargeCaches(ctx, s.cache, c)
	if s.keys != nil {
		s.keys.InvalidateAuthCacheByUserID(ctx, c.UserID)
	}
	if c.SubscriptionID != nil && s.subscriptions != nil {
		s.subscriptions.InvalidateSubCacheSync(c.UserID, c.GroupID)
	}
}
func invalidateCanvasChargeCaches(ctx context.Context, cache *BillingCacheService, c *CanvasCharge) {
	if cache == nil || c == nil {
		return
	}
	if c.SubscriptionID != nil {
		_ = cache.InvalidateSubscriptionByID(ctx, *c.SubscriptionID)
		_ = cache.InvalidateSubscription(ctx, c.UserID, c.GroupID)
	} else {
		_ = cache.InvalidateUserBalance(ctx, c.UserID)
	}
}
func (s *CanvasBillingService) finish(id string, success bool) (*CanvasCharge, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	c, err := s.repo.FinishCanvasCharge(ctx, id, success)
	if err == nil {
		s.invalidate(ctx, c)
	}
	return c, err
}
func (s *CanvasBillingService) recoverExpired(ctx context.Context) {
	charges, err := s.repo.ExpiredCanvasCharges(ctx)
	if err != nil {
		log.Printf("canvas billing recovery lookup failed: %v", err)
		return
	}
	for _, c := range charges {
		result, err := s.repo.FailExpiredCanvasCharge(ctx, c.ID)
		if err != nil {
			log.Printf("canvas billing recovery failed for charge %s: %v", c.ID, err)
			continue
		}
		s.invalidate(ctx, result)
	}
}

// Attach capabilities only to authenticated calls to our own gateway. Asset CDN
// downloads and redirects must never receive this header.
type canvasTransportContextKey struct{}
type canvasTransportCredentials struct{ token, origin, key string }
type canvasBillingTransport struct{ base http.RoundTripper }

func (t canvasBillingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	credential, _ := r.Context().Value(canvasTransportContextKey{}).(canvasTransportCredentials)
	copy := r.Clone(r.Context())
	copy.Header = r.Header.Clone()
	copy.Header.Del(CanvasBillingHeader)
	origin, err := url.Parse(credential.origin)
	if err == nil && origin != nil && r.URL.Scheme == origin.Scheme && r.URL.Host == origin.Host && credential.token != "" && credential.key != "" && (r.Header.Get("Authorization") == "Bearer "+credential.key || r.Header.Get("x-goog-api-key") == credential.key || r.Header.Get("x-api-key") == credential.key) {
		copy.Header.Set(CanvasBillingHeader, credential.token)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(copy)
}
func canvasRecoveredRun(c *CanvasCharge) *PlaygroundRun {
	status := PlaygroundRunRunning
	message := ""
	if c.Status == "failed" {
		status = PlaygroundRunFailed
		message = "任务已中止，预扣费用已退回"
	}
	if c.Status == "succeeded" {
		status = PlaygroundRunSucceeded
	}
	return &PlaygroundRun{ID: c.RunID, UserID: c.UserID, Mode: c.Mode, Model: c.Model, Status: status, CreatedAt: c.CreatedAt, UpdatedAt: time.Now(), Error: message}
}
