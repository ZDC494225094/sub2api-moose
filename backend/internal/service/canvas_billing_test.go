package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type canvasRepoStub struct {
	CanvasBillingRepository
	mu                          sync.Mutex
	charge                      *CanvasCharge
	tokenHash                   string
	reserveErr                  error
	finishErr                   error
	reserves, refunds, captures int
}

func (r *canvasRepoStub) GetCanvasCharge(_ context.Context, u int64, id string) (*CanvasCharge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.charge == nil || r.charge.UserID != u || r.charge.RunID != id {
		return nil, nil
	}
	c := *r.charge
	return &c, nil
}
func (r *canvasRepoStub) ReserveCanvasCharge(_ context.Context, in *CanvasChargeReservation) (*CanvasCharge, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.reserveErr != nil {
		return nil, false, r.reserveErr
	}
	if r.charge != nil {
		c := *r.charge
		return &c, false, nil
	}
	c := in.CanvasCharge
	c.Status = "reserved"
	c.CreatedAt = time.Now()
	r.charge = &c
	r.tokenHash = in.TokenHash
	r.reserves++
	out := c
	return &out, true, nil
}
func (r *canvasRepoStub) FinishCanvasCharge(_ context.Context, id string, success bool) (*CanvasCharge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.finishErr != nil {
		return nil, r.finishErr
	}
	if r.charge == nil || r.charge.ID != id {
		return nil, ErrCanvasChargeUnavailable
	}
	if r.charge.Status == "reserved" {
		if success {
			r.captures++
			r.charge.Status = "succeeded"
			r.charge.ChargedCost = r.charge.ReservedCost
		} else {
			r.refunds++
			r.charge.Status = "failed"
			r.charge.ChargedCost = 0
		}
	}
	c := *r.charge
	return &c, nil
}
func (r *canvasRepoStub) AuthenticateCanvasCharge(_ context.Context, hash string, id int64) (*CanvasCharge, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.charge == nil || r.tokenHash != hash || r.charge.APIKeyID != id {
		return nil, nil
	}
	c := *r.charge
	return &c, nil
}
func (*canvasRepoStub) ExpiredCanvasCharges(context.Context) ([]CanvasCharge, error) { return nil, nil }
func (*canvasRepoStub) RenewCanvasCharge(context.Context, string) (bool, error)      { return true, nil }

type canvasKeyRepoStub struct{ APIKeyRepository }

func (canvasKeyRepoStub) ListKeysByUserID(context.Context, int64) ([]string, error) { return nil, nil }
func canvasTestBilling(r *canvasRepoStub) (*CanvasBillingService, *APIKey) {
	cfg := &config.Config{}
	cfg.Default.RateMultiplier = 1
	price := 3.0
	group := &Group{ID: 11, Status: StatusActive, Platform: PlatformOpenAI, RateMultiplier: 1, ImageRateMultiplier: 1, ImagePrice1K: &price, ImagePrice2K: &price, ImagePrice4K: &price}
	key := &APIKey{ID: 9, UserID: 7, Key: "sk-internal", Name: CanvasManagedAPIKeyName(11), Status: StatusAPIKeyActive, GroupID: &group.ID, Group: group, User: &User{ID: 7, Status: StatusActive, Balance: 100}}
	keys := &APIKeyService{apiKeyRepo: canvasKeyRepoStub{}, canvasBillingRepo: r}
	billing := &CanvasBillingService{repo: r, keys: keys, cfg: cfg, gateway: &GatewayService{cfg: cfg, billingService: NewBillingService(cfg, nil)}}
	return billing, key
}
func TestCanvasStartPrechargesBeforeUpstreamAndRefundsFailure(t *testing.T) {
	for _, fail := range []bool{true, false} {
		t.Run(map[bool]string{true: "upstream_failure", false: "upstream_success"}[fail], func(t *testing.T) {
			r := &canvasRepoStub{}
			billing, key := canvasTestBilling(r)
			s := ProvidePlaygroundRunService(nil, nil, nil, billing)
			defer s.Stop()
			reached := make(chan struct{})
			release := make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			defer unblock()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				r.mu.Lock()
				reserved := r.reserves == 1 && r.charge.Status == "reserved" && r.charge.ReservedCost > 0
				r.mu.Unlock()
				if !reserved {
					t.Error("upstream dispatched before durable precharge")
				}
				if req.Header.Get(CanvasBillingHeader) == "" {
					t.Error("internal gateway capability missing")
				}
				close(reached)
				<-release
				if fail {
					w.WriteHeader(400)
					_, _ = io.WriteString(w, `{"error":{"message":"upstream rejected"}}`)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[{"b64_json":"aW1hZ2U="}]}`)
			}))
			defer func() { unblock(); server.Close() }()
			req := PlaygroundRunRequest{CanvasKey: key, ID: "canvas-run", Mode: "image", APIKey: key.Key, Model: "gpt-image-2", Prompt: "test", N: 1}
			_, err := s.Start(7, req, server.URL)
			require.NoError(t, err)
			select {
			case <-reached:
			case <-time.After(2 * time.Second):
				t.Fatal("upstream not reached")
			}
			_, err = s.Start(7, req, server.URL)
			require.NoError(t, err)
			unblock()
			require.Eventually(t, func() bool { run, ok := s.Get(7, req.ID); return ok && isTerminalPlaygroundRunStatus(run.Status) }, 3*time.Second, 10*time.Millisecond)
			r.mu.Lock()
			defer r.mu.Unlock()
			require.Equal(t, 1, r.reserves)
			if fail {
				require.Equal(t, 1, r.refunds)
				require.Zero(t, r.captures)
			} else {
				require.Equal(t, 1, r.captures)
				require.Zero(t, r.refunds)
			}
		})
	}
}
func TestCanvasInsufficientFundsDoNotDispatch(t *testing.T) {
	r := &canvasRepoStub{reserveErr: ErrInsufficientBalance}
	billing, key := canvasTestBilling(r)
	s := ProvidePlaygroundRunService(nil, nil, nil, billing)
	defer s.Stop()
	_, err := s.Start(7, PlaygroundRunRequest{CanvasKey: key, ID: "run", Mode: "image", APIKey: key.Key, Model: "gpt-image-2"}, "http://127.0.0.1:1")
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Empty(t, s.runs)
	require.Zero(t, r.reserves)
}
func TestCanvasAuthRejectsForgeryAndWrongIdentity(t *testing.T) {
	r := &canvasRepoStub{charge: &CanvasCharge{ID: "charge", UserID: 7, APIKeyID: 9, Status: "reserved"}, tokenHash: canvasTokenHash("secret")}
	billing, key := canvasTestBilling(r)
	ctx, err := billing.keys.AuthenticateCanvasCharge(context.Background(), key, "secret")
	require.NoError(t, err)
	require.Equal(t, "charge", CanvasChargeFromContext(ctx).ID)
	for _, token := range []string{"wrong", strings.Repeat("x", 129)} {
		_, err = billing.keys.AuthenticateCanvasCharge(context.Background(), key, token)
		require.ErrorIs(t, err, ErrCanvasChargeUnavailable)
	}
	key.ID = 10
	_, err = billing.keys.AuthenticateCanvasCharge(context.Background(), key, "secret")
	require.ErrorIs(t, err, ErrCanvasChargeUnavailable)
	key.ID = 9
	r.charge.Status = "failed"
	_, err = billing.keys.AuthenticateCanvasCharge(context.Background(), key, "secret")
	require.ErrorIs(t, err, ErrCanvasChargeUnavailable)
	ctx, err = billing.keys.AuthenticateCanvasCharge(context.Background(), key, "")
	require.NoError(t, err)
	require.Nil(t, CanvasChargeFromContext(ctx))
	var request PlaygroundRunRequest
	require.NoError(t, json.Unmarshal([]byte(`{"CanvasKey":{"ID":9},"canvasCharge":{"ID":"forged"},"canvasToken":"forged"}`), &request))
	require.Nil(t, request.CanvasKey)
	require.Nil(t, request.canvasCharge)
	require.Empty(t, request.canvasToken)
}

type canvasRoundTripper func(*http.Request) (*http.Response, error)

func (f canvasRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCanvasCapabilityNeverLeaksToAssetsOrRedirects(t *testing.T) {
	for _, tt := range []struct {
		name, url, auth, key string
		want                 bool
	}{
		{"gateway", "https://gateway.test/v1/images", "Bearer sk", "sk", true},
		{"cdn", "https://cdn.test/result.png", "Bearer sk", "sk", false},
		{"other_port", "https://gateway.test:9443/v1/images", "Bearer sk", "sk", false},
		{"wrong_key", "https://gateway.test/v1/images", "Bearer wrong", "sk", false},
		{"no_key", "https://gateway.test/v1/images", "", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), canvasTransportContextKey{}, canvasTransportCredentials{origin: "https://gateway.test", token: "secret", key: tt.key})
			req, err := http.NewRequestWithContext(ctx, "GET", tt.url, nil)
			require.NoError(t, err)
			req.Header.Set("Authorization", tt.auth)
			req.Header.Set(CanvasBillingHeader, "forged")
			transport := canvasBillingTransport{base: canvasRoundTripper(func(got *http.Request) (*http.Response, error) {
				if tt.want {
					require.Equal(t, "secret", got.Header.Get(CanvasBillingHeader))
				} else {
					require.Empty(t, got.Header.Get(CanvasBillingHeader))
				}
				return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
			})}
			_, err = transport.RoundTrip(req)
			require.NoError(t, err)
			require.Equal(t, "forged", req.Header.Get(CanvasBillingHeader), "do not mutate caller's header")
		})
	}
}
func TestCanvasEstimateUsesExecutionCounts(t *testing.T) {
	require.Equal(t, 4, canvasChargeEstimate(PlaygroundRunRequest{Mode: "image", N: 20}).Units)
	require.Equal(t, 1, canvasChargeEstimate(PlaygroundRunRequest{Mode: "image", N: -1}).Units)
	require.Equal(t, 3, canvasChargeEstimate(PlaygroundRunRequest{Mode: "video", Platform: PlatformGemini, N: 3}).Units)
	require.Equal(t, 1, canvasChargeEstimate(PlaygroundRunRequest{Mode: "video", Platform: PlatformOpenAI, N: 3}).Units)
	require.Equal(t, 0.000002, canvasChargeEstimate(PlaygroundRunRequest{Mode: "audio", Prompt: "你好"}).AudioUnits)
}
func TestCanvasBillingCannotFallBackToSecondDebit(t *testing.T) {
	ctx := WithCanvasCharge(context.Background(), &CanvasCharge{ID: "charge"})
	p := &postUsageBillingParams{Cost: &CostBreakdown{ActualCost: 2}, User: &User{ID: 7}, APIKey: &APIKey{ID: 9}, Account: &Account{ID: 11}}
	_, err := applyUsageBilling(ctx, "request", nil, p, &billingDeps{}, nil)
	require.ErrorIs(t, err, ErrCanvasChargeUnavailable)
	repo := &simpleModeUsageBillingRepoStub{}
	_, err = applyUsageBilling(ctx, "request", nil, p, &billingDeps{deferredService: &DeferredService{}}, repo)
	require.NoError(t, err)
	require.Equal(t, "charge", repo.cmds[0].CanvasChargeID)
	require.NotEqual(t, buildUsageBillingFingerprint(&UsageBillingCommand{RequestID: "request"}), repo.cmds[0].RequestFingerprint)
}
func TestCanvasCancelSuccessRaceAndRefundError(t *testing.T) {
	for _, status := range []string{"succeeded", "reserved"} {
		t.Run(status, func(t *testing.T) {
			r := &canvasRepoStub{charge: &CanvasCharge{ID: "charge", UserID: 7, APIKeyID: 9, RunID: "run", Status: status}}
			if status == "reserved" {
				r.finishErr = errors.New("database down")
			}
			billing, _ := canvasTestBilling(r)
			s := NewPlaygroundRunService()
			s.canvasBilling = billing
			s.runs[playgroundRunKey(7, "run")] = &PlaygroundRun{ID: "run", UserID: 7, Status: PlaygroundRunRunning}
			run, ok := s.Cancel(7, "run")
			require.True(t, ok)
			require.NotEqual(t, PlaygroundRunCanceled, run.Status)
			require.Zero(t, r.refunds)
		})
	}
}

type canvasSubscriptionRepoStub struct {
	UserSubscriptionRepository
	sub      *UserSubscription
	selected int64
}

func (r *canvasSubscriptionRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	r.selected = id
	return r.sub, nil
}
func TestCanvasPinnedSubscriptionSurvivesPrechargeLimit(t *testing.T) {
	r := &canvasRepoStub{}
	billing, key := canvasTestBilling(r)
	id := int64(77)
	subRepo := &canvasSubscriptionRepoStub{sub: &UserSubscription{ID: id, UserID: 7, GroupID: 11, DailyUsageUSD: 100}}
	subService := &SubscriptionService{userSubRepo: subRepo}
	ctx := WithCanvasCharge(context.Background(), &CanvasCharge{ID: "charge", UserID: 7, APIKeyID: 9, GroupID: 11, SubscriptionID: &id})
	selection, err := billing.keys.SelectUsableGroupForAPIKey(ctx, key, subService)
	require.NoError(t, err)
	require.Equal(t, id, selection.Subscription.ID)
	require.Equal(t, id, subRepo.selected)
	subRepo.sub = nil
	_, err = billing.keys.SelectUsableGroupForAPIKey(ctx, key, subService)
	require.ErrorIs(t, err, ErrCanvasChargeUnavailable)
}
