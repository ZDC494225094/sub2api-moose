package service

import (
	"bytes"
	"context"
	"errors"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMediaExtensionServiceRejectsBeforeNetwork(t *testing.T) {
	t.Setenv(xai.EnvAllowUnsafeURLOverrides, "true")
	gin.SetMode(gin.TestMode)
	for _, flag := range []string{"", "false", "malformed", "unavailable"} {
		for _, adapter := range []string{"compatible", "grok-video-r", "string-image"} {
			t.Run(flag+"/"+adapter, func(t *testing.T) {
				values := map[string]string{}
				if flag != "" {
					values["custom_extensions.media-gateway.enabled"] = flag
				}
				repo := &upstreamBillingProbeSettingRepo{values: values}
				if flag == "unavailable" {
					repo.readErr = errors.New("offline")
				}
				settings := &SettingService{settingRepo: repo}
				svc := &OpenAIGatewayService{settingService: settings} // nil HTTP client: a network attempt would panic
				account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "base_url": "https://api.x.ai/v1"}}
				body := []byte(`{"model":"grok-video-r","prompt":"test"}`)
				if adapter == "string-image" {
					body = []byte(`{"model":"grok-imagine-video","prompt":"test","image":"https://example.org/image.png"}`)
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(body))
				var err error
				if adapter == "compatible" {
					_, err = svc.ForwardOpenAICompatibleVideo(context.Background(), c, account, GrokMediaEndpointVideosGenerations, "", body, "application/json")
				} else {
					account.Platform = PlatformGrok
					_, err = svc.ForwardGrokMedia(context.Background(), c, account, GrokMediaEndpointVideosGenerations, "", body, "application/json")
				}
				require.Error(t, err)
				want := http.StatusForbidden
				if flag == "malformed" || flag == "unavailable" {
					want = http.StatusServiceUnavailable
				}
				require.Equal(t, want, infraerrors.Code(err))
			})
		}
	}
}

func TestCompatibleVideoAdmissionOnAndHistoricalDrain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []GrokMediaEndpoint{GrokMediaEndpointVideosGenerations, GrokMediaEndpointVideoStatus, GrokMediaEndpointVideoContent} {
		t.Run(string(endpoint), func(t *testing.T) {
			repo := &upstreamBillingProbeSettingRepo{values: map[string]string{"custom_extensions.media-gateway.enabled": "true"}}
			if endpoint != GrokMediaEndpointVideosGenerations {
				repo.readErr = errors.New("state database offline")
			}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"id":"task-1","status":"completed","video":{"url":"https://video.example/result.mp4"}}`))}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, settingService: &SettingService{settingRepo: repo}, httpUpstream: upstream}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "base_url": "https://relay.example.test/v1"}}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task-1", nil)
			result, err := svc.ForwardOpenAICompatibleVideo(context.Background(), c, account, endpoint, "task-1", []byte(`{"model":"custom-video","prompt":"test"}`), "application/json")
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, upstream.lastReq)
			if endpoint == GrokMediaEndpointVideosGenerations {
				require.Equal(t, http.MethodPost, upstream.lastReq.Method)
			} else {
				require.Equal(t, http.MethodGet, upstream.lastReq.Method)
			}
		})
	}
}
