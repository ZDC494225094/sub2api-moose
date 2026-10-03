//go:build unit

package handler

import (
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/customize/modules/mediagateway"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func enabledMediaSettings() *service.SettingService {
	return service.NewSettingService(&contentModerationHandlerSettingRepo{values: map[string]string{customize.Key(mediagateway.ModuleID): "true"}}, nil)
}

type failingMediaSettings struct {
	contentModerationHandlerSettingRepo
}

func (*failingMediaSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, errors.New("store down")
}
func TestMediaGatewayRejectsVeoBeforeUpstream(t *testing.T) {
	for _, value := range []string{"false", "", "malformed", "missing", "outage", "nil"} {
		t.Run(value, func(t *testing.T) {
			settings := service.NewSettingService(&contentModerationHandlerSettingRepo{values: map[string]string{customize.Key(mediagateway.ModuleID): value}}, nil)
			want := 403
			if value == "missing" {
				settings = service.NewSettingService(&contentModerationHandlerSettingRepo{values: map[string]string{}}, nil)
			}
			if value == "outage" {
				settings = service.NewSettingService(&failingMediaSettings{}, nil)
				want = 503
			}
			if value == "nil" {
				settings = nil
				want = 503
			}
			if value == "malformed" || value == "" {
				want = 503
			}
			h := &GatewayHandler{settingService: settings}
			c, w := grokMediaSlotContext(context.Background(), true)
			key, _ := middleware.GetAPIKeyFromContext(c)
			key.Group.Platform = service.PlatformGemini
			c.Params = append(c.Params, struct {
				Key   string
				Value string
			}{"modelAction", "veo-3:predictLongRunning"})
			h.GeminiV1BetaModels(c)
			require.Equal(t, want, w.Code, w.Body.String())
		})
	}
}
func TestMediaGatewayDisabledStillChecksHistoricalOwnership(t *testing.T) {
	for _, settings := range []*service.SettingService{nil, service.NewSettingService(&failingMediaSettings{}, nil), service.NewSettingService(&contentModerationHandlerSettingRepo{values: map[string]string{}}, nil)} {
		h, _, _, _ := newGrokMediaSlotHandlerWithSettings(t, false, false, settings)
		c, w := grokMediaSlotContext(context.Background(), false)
		h.GrokVideoStatus(c)
		require.Equal(t, 200, w.Code, w.Body.String())
		c, w = grokMediaSlotContext(context.Background(), false)
		key, _ := middleware.GetAPIKeyFromContext(c)
		key.ID = 999
		h.GrokVideoStatus(c)
		require.NotEqual(t, 200, w.Code)
		require.NotEqual(t, 503, w.Code)
	}
}

// Native image tools reach the host scheduler regardless of the extension flag.
// This fixture has no available upstream and intentionally returns 502.
func TestMediaGatewayNativeResponsesPreserved(t *testing.T) {
	for _, value := range []string{"false", "missing", "malformed", "outage"} {
		t.Run(value, func(t *testing.T) {
			values := map[string]string{}
			if value != "missing" {
				values[customize.Key(mediagateway.ModuleID)] = value
			}
			settings := service.NewSettingService(&contentModerationHandlerSettingRepo{values: values}, nil)
			want := http.StatusBadGateway
			if value == "malformed" {
				want = http.StatusBadGateway
			}
			if value == "outage" {
				settings = service.NewSettingService(&failingMediaSettings{}, nil)
				want = http.StatusBadGateway
			}
			h, slots, bindings, upstream := newGrokMediaSlotHandlerWithSettings(t, false, false, settings)
			upstream.call = func(*http.Request, int64) (*http.Response, error) {
				t.Fatal("disabled media reached upstream")
				return nil, nil
			}
			c, w := grokMediaSlotContext(context.Background(), true)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"grok-3","input":"draw","tools":[{"type":"image_generation"}]}`))
			c.Request.Header.Set("Content-Type", "application/json")
			h.Responses(c)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, slots.acquired)
			require.Equal(t, 1, slots.userAcquired)
			require.Zero(t, bindings.writes)
		})
	}
}

func TestMediaGatewayDisabledPreservesDownloadOwnership(t *testing.T) {
	h, _, _, upstream := newGrokMediaSlotHandlerWithSettings(t, false, false, service.NewSettingService(&failingMediaSettings{}, nil))
	calls := 0
	upstream.call = func(*http.Request, int64) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(strings.NewReader("retained-video"))}, nil
	}
	c, w := grokMediaSlotContext(context.Background(), false)
	h.GrokVideoContent(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "retained-video", w.Body.String())
	require.Positive(t, calls)
	admittedCalls := calls // host may read task status before fetching content
	c, w = grokMediaSlotContext(context.Background(), false)
	key, _ := middleware.GetAPIKeyFromContext(c)
	key.ID = 999
	h.GrokVideoContent(c)
	require.NotEqual(t, http.StatusOK, w.Code)
	require.Equal(t, admittedCalls, calls, "foreign owner must not download")
}

func TestMediaGatewayNativeResponsesWebSocketPreserved(t *testing.T) {
	for _, value := range []string{"false", "malformed", "missing"} {
		t.Run(value, func(t *testing.T) {
			settings := service.NewSettingService(&contentModerationHandlerSettingRepo{values: map[string]string{customize.Key(mediagateway.ModuleID): value}}, nil)
			if value == "missing" {
				settings = service.NewSettingService(&contentModerationHandlerSettingRepo{values: map[string]string{}}, nil)
			}
			h, slots, _, _ := newGrokMediaSlotHandlerWithSettings(t, false, false, settings)
			source, _ := grokMediaSlotContext(context.Background(), true)
			key, _ := middleware.GetAPIKeyFromContext(source)
			subject, _ := middleware.GetAuthSubjectFromContext(source)
			done := make(chan struct{})
			router := gin.New()
			router.GET("/v1/responses", func(c *gin.Context) {
				defer close(done)
				c.Set(string(middleware.ContextKeyAPIKey), key)
				c.Set(string(middleware.ContextKeyUser), subject)
				h.ResponsesWebSocket(c)
			})
			server := httptest.NewServer(router)
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			conn, _, err := coderws.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/v1/responses", nil)
			require.NoError(t, err)
			defer conn.CloseNow()
			require.NoError(t, conn.Write(ctx, coderws.MessageText, []byte(`{"type":"response.create","model":"grok-3","input":"draw","tools":[{"type":"image_generation"}]}`)))
			_, _, err = conn.Read(ctx)
			require.Equal(t, coderws.StatusTryAgainLater, coderws.CloseStatus(err))
			require.NotContains(t, err.Error(), "media generation is unavailable")
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("handler did not finish")
			}
			require.Zero(t, slots.acquired)
			require.Equal(t, 1, slots.userAcquired)
		})
	}
}

func TestMediaGatewayNativeProtocolsBypassExtensionState(t *testing.T) {
	for _, settings := range []*service.SettingService{nil, service.NewSettingService(&failingMediaSettings{}, nil)} {
		h, _, _, _ := newGrokMediaSlotHandlerWithSettings(t, false, false, settings)
		for _, ep := range []mediagateway.Endpoint{mediagateway.ImagesGenerations, mediagateway.ImagesEdits, mediagateway.VideosGenerations, mediagateway.VideosEdits, mediagateway.VideosExtensions, mediagateway.SeedanceCreate} {
			c, _ := grokMediaSlotContext(context.Background(), true)
			require.True(t, h.admitMediaGateway(c, ep.Operation()))
		}
		c, _ := grokMediaSlotContext(context.Background(), true)
		require.True(t, h.admitMediaGateway(c, mediagateway.VoiceOperation(http.MethodPost)))
	}
}
