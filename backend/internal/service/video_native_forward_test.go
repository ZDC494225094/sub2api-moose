//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type nativeVideoTestUpstream struct {
	HTTPUpstream
	call func(*http.Request) (*http.Response, error)
}

func (upstream *nativeVideoTestUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	return upstream.call(req)
}

func TestNativeVideoForwardCreatePollAndDownload(t *testing.T) {
	for _, tt := range []struct{ protocol, createPath, pollPath, created, completed string }{
		{"minimax-h3", "/v2/video_generation", "/v2/query/video_generation/42", `{"task_id":"42","base_resp":{"status_code":0}}`, `{"task":{"task_id":"42","status":"succeeded","duration":12,"content":{"url":"https://cdn.test/h3.mp4"}}}`},
		{"minimax", "/v1/video_generation", "/v1/query/video_generation?task_id=42", `{"task_id":"42","base_resp":{"status_code":0}}`, `{"task_id":"42","status":"Success","file_id":"file-42"}`},
		{"kling-v3", "/image-to-video/kling-3.0", "/tasks?task_ids=42", `{"code":0,"data":{"id":"42","status":"submitted"}}`, `{"code":0,"data":[{"id":"42","status":"succeeded","outputs":[{"type":"video","url":"https://cdn.test/kling.mp4","duration":"12"}]}]}`},
		{"omni", "/v1beta/interactions", "/v1beta/interactions/42", `{"id":"42","status":"in_progress"}`, `{"id":"42","status":"completed","steps":[{"type":"model_output","content":[{"type":"video","uri":"https://generativelanguage.googleapis.com/v1beta/files/42:download?alt=media"}]}]}`},
		{"ark", "/api/v3/contents/generations/tasks", "/api/v3/contents/generations/tasks/42", `{"id":"42","status":"queued"}`, `{"id":"42","status":"succeeded","duration":12,"content":{"video_url":"https://cdn.test/ark.mp4"}}`},
	} {
		t.Run(tt.protocol, func(t *testing.T) {
			step := 0
			upstream := &nativeVideoTestUpstream{}
			upstream.call = func(req *http.Request) (*http.Response, error) {
				if req.URL.Hostname() == "generativelanguage.googleapis.com" && req.URL.Path == "/v1beta/files/42" {
					require.Equal(t, "server-only-key", req.Header.Get("x-goog-api-key"))
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"state":"ACTIVE"}`))}, nil
				}
				if tt.protocol == "omni" {
					require.Equal(t, "server-only-key", req.Header.Get("x-goog-api-key"))
					require.Empty(t, req.Header.Get("Authorization"))
				} else {
					require.Equal(t, "Bearer server-only-key", req.Header.Get("Authorization"))
				}
				response := tt.completed
				switch step {
				case 0:
					require.Equal(t, http.MethodPost, req.Method)
					require.Equal(t, tt.createPath, req.URL.RequestURI())
					body, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					if tt.protocol != "kling-v3" {
						require.Equal(t, "mapped", gjson.GetBytes(body, "model").String())
					}
					require.False(t, gjson.GetBytes(body, "_video_protocol").Exists())
					response = tt.created
				case 1:
					require.Equal(t, tt.pollPath, req.URL.RequestURI())
				case 2:
					if tt.protocol == "minimax" {
						require.Equal(t, "/v1/files/retrieve?file_id=file-42", req.URL.RequestURI())
						response = `{"file":{"download_url":"https://cdn.test/hailuo.mp4"},"base_resp":{"status_code":0}}`
					} else {
						require.Equal(t, "omni", tt.protocol)
						require.Equal(t, tt.pollPath, req.URL.RequestURI())
					}
				case 3:
					require.Equal(t, "generativelanguage.googleapis.com", req.URL.Hostname())
					require.Equal(t, "/v1beta/files/42:download?alt=media", req.URL.RequestURI())
					step++
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"video/mp4"}}, Body: io.NopCloser(strings.NewReader("video-bytes"))}, nil
				default:
					t.Fatal("unexpected upstream request")
				}
				step++
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(response))}, nil
			}
			settings := &SettingService{settingRepo: &upstreamBillingProbeSettingRepo{values: map[string]string{"custom_extensions.media-gateway.enabled": "true"}}}
			gateway := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, settingService: settings}
			account := &Account{ID: 9, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://relay.example.test/v1", "api_key": "server-only-key", "model_mapping": map[string]any{"original": "mapped"}}}
			makeContext := func() (*gin.Context, *httptest.ResponseRecorder) {
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
				return c, recorder
			}
			c, created := makeContext()
			result, err := gateway.ForwardOpenAICompatibleVideo(context.Background(), c, account, GrokMediaEndpointVideosGenerations, "", []byte(`{"model":"original","prompt":"waves","duration":6,"resolution":"720p","aspect_ratio":"16:9","video_mode":"frames","image":"https://cdn.test/first.png","_video_protocol":"`+tt.protocol+`"}`), "application/json")
			require.NoError(t, err)
			require.Equal(t, "original", result.BillingModel)
			require.Equal(t, "mapped", result.UpstreamModel)
			taskID := gjson.Get(created.Body.String(), "id").String()
			require.Equal(t, taskID, result.ResponseID)
			require.NotEmpty(t, taskID)
			c, polled := makeContext()
			result, err = gateway.ForwardOpenAICompatibleVideo(context.Background(), c, account, GrokMediaEndpointVideoStatus, taskID, nil, "")
			require.NoError(t, err)
			require.Equal(t, taskID, gjson.Get(polled.Body.String(), "id").String())
			require.Equal(t, "completed", gjson.Get(polled.Body.String(), "status").String())
			require.Equal(t, 1, result.VideoCount)
			billing := ExtractOpenAICompatibleVideoBillingFromStatusBody(polled.Body.Bytes(), nil, taskID)
			require.NotNil(t, billing)
			require.Equal(t, "original", billing.BillingModel)
			if tt.protocol == "omni" {
				require.Equal(t, "/v1/videos/"+taskID+"/content", gjson.Get(polled.Body.String(), "video.url").String())
				c, downloaded := makeContext()
				_, err = gateway.ForwardOpenAICompatibleVideo(context.Background(), c, account, GrokMediaEndpointVideoContent, taskID, nil, "")
				require.NoError(t, err)
				require.Equal(t, "video-bytes", downloaded.Body.String())
				require.NotContains(t, polled.Body.String(), "server-only-key")
			}
		})
	}
}
