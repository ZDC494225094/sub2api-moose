//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type videoReferenceForwardUpstream struct {
	HTTPUpstream
	body []byte
	url  string
}

func (u *videoReferenceForwardUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	var err error
	u.body, err = io.ReadAll(req.Body)
	u.url = req.URL.String()
	if err != nil {
		return nil, err
	}
	return grokMediaContentStatusResponse(`{}`), nil
}

func TestCanvasVideoReferencesReachFinalUpstreamRequest(t *testing.T) {
	for _, route := range []string{"relay", "relay_explicit", "relay_fast_auto", "relay_manual_alias", "ark", "ark_legacy_fields", "native_seedance"} {
		t.Run(route, func(t *testing.T) {
			storage := &fakeImageStorage{}
			runs := NewPlaygroundRunService()
			runs.imageStorageResolver = func() (*ImageResultUploader, bool) {
				return NewImageResultUploader(storage, "images/", 0, nil), true
			}
			upstream := &videoReferenceForwardUpstream{}
			settings := &SettingService{settingRepo: &upstreamBillingProbeSettingRepo{values: map[string]string{"custom_extensions.media-gateway.enabled": "true"}}}
			gateway := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, settingService: settings}
			base := "https://relay.example.test/v1"
			if !strings.HasPrefix(route, "relay") {
				base = "https://ark.cn-beijing.volces.com/api/v3"
			}
			model := "doubao-seedance-2-0-260128"
			if route == "relay_fast_auto" {
				model = "seedance2.0-fast"
			} else if route == "relay_manual_alias" {
				model = "private-model"
			}
			account := &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
				"base_url": base, "api_key": "test", "openai_capabilities": []string{"seedance"},
				"model_mapping": map[string]any{model: "doubao-seedance-2-0-fast-260128"},
			}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				body, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				if route == "ark_legacy_fields" {
					body, err = sjson.DeleteBytes(body, "content")
					require.NoError(t, err)
				}
				c, _ := gin.CreateTestContext(w)
				c.Request = req
				if route == "native_seedance" {
					_, err = gateway.ForwardSeedance(req.Context(), c, account, SeedanceEndpointCreate, "", body)
					// The fake response intentionally has no task id.
					require.ErrorContains(t, err, "missing task ID")
					c.Data(http.StatusOK, "application/json", []byte(`{}`))
				} else {
					_, err = gateway.ForwardOpenAICompatibleVideo(req.Context(), c, account, GrokMediaEndpointVideosGenerations, "", body, "application/json")
					require.NoError(t, err)
				}
			}))
			defer server.Close()
			request := PlaygroundRunRequest{
				ID: "run-four-references", Model: model, APIKey: "test", Prompt: "replace the actor", Duration: 15,
				AspectRatio: "16:9", Resolution: "720p",
				Images:          []PlaygroundRunImageInput{{DataURL: videoInputDataURL()}},
				ReferenceVideos: []PlaygroundRunVideoInput{{DataURL: "data:video/mp4;base64,QUJD"}, {DataURL: "data:video/webm;base64,REVG"}},
				ReferenceAudios: []PlaygroundRunVideoInput{{DataURL: "data:audio/mpeg;base64,R0hJ"}},
			}
			if strings.HasPrefix(route, "relay_") {
				request.VideoStandard, request.VideoProtocol, request.VideoMode = "seedance-2-fast", "auto", "reference"
				if route == "relay_explicit" {
					request.VideoProtocol = "relay"
				}
			}
			_, err := runs.executeVideo(context.Background(), "test", request, server.URL, time.Now())
			require.ErrorContains(t, err, "neither a video nor a task id")
			require.Len(t, storage.saved, 4)
			require.Equal(t, "doubao-seedance-2-0-fast-260128", gjson.GetBytes(upstream.body, "model").String())
			parts := gjson.GetBytes(upstream.body, "content").Array()
			require.Len(t, parts, 5)
			for i, kind := range []string{"image", "video", "video", "audio"} {
				part := parts[i+1]
				require.Equal(t, kind+"_url", part.Get("type").String())
				require.Equal(t, "https://cdn.test/"+storage.saved[i].key, part.Get(kind+"_url.url").String())
				require.Equal(t, "reference_"+kind, part.Get("role").String())
			}
			if strings.HasPrefix(route, "relay") {
				require.Equal(t, base+"/videos/generations", upstream.url, "payload conversion must not change the relay endpoint")
			}
			if route != "relay_explicit" && route != "native_seedance" {
				for _, field := range []string{"reference_videos", "reference_audios", "video_standard", "video_mode", "_video_protocol", "image", "video", "aspect_ratio"} {
					require.False(t, gjson.GetBytes(upstream.body, field).Exists(), field)
				}
				require.Equal(t, "16:9", gjson.GetBytes(upstream.body, "ratio").String())
			} else if route == "relay_explicit" {
				require.Len(t, gjson.GetBytes(upstream.body, "reference_videos").Array(), 2)
				require.Len(t, gjson.GetBytes(upstream.body, "reference_audios").Array(), 1)
				require.False(t, gjson.GetBytes(upstream.body, "_video_protocol").Exists())
			}
		})
	}
}
