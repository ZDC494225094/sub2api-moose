package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
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

func TestDetectOpenAIImageResultSize(t *testing.T) {
	pngEncoded := encodeOpenAIImageTestPNG(t, 1672, 941)
	jpegEncoded := encodeOpenAIImageTestJPEG(t, 640, 360)
	webpVP8XEncoded := encodeOpenAIImageTestWebPVP8X(1920, 1080)
	webpVP8Encoded := encodeOpenAIImageTestWebPVP8(1280, 720)
	webpVP8LEncoded := encodeOpenAIImageTestWebPVP8L(640, 480)

	require.Equal(t, "1672x941", detectOpenAIImageResultSize(pngEncoded))
	require.Equal(t, "1672x941", detectOpenAIImageResultSize(strings.TrimRight(pngEncoded, "=")))
	require.Equal(t, "1672x941", detectOpenAIImageResultSize("data:image/png;base64,"+pngEncoded))
	require.Equal(t, "640x360", detectOpenAIImageResultSize(jpegEncoded))
	require.Equal(t, "1920x1080", detectOpenAIImageResultSize(webpVP8XEncoded))
	require.Equal(t, "1280x720", detectOpenAIImageResultSize(webpVP8Encoded))
	require.Equal(t, "640x480", detectOpenAIImageResultSize(webpVP8LEncoded))
	require.Empty(t, detectOpenAIImageResultSize("data:image/png;base64"))
	require.Empty(t, detectOpenAIImageResultSize("not-image-data"))
}

func TestOpenAIGatewayServiceForwardImages_OAuthUsesDecodedOutputDimensions(t *testing.T) {
	run := runOpenAIOAuthImageActualSizeTest(t, false, "auto")

	require.NoError(t, run.err)
	require.NotNil(t, run.result)
	require.Equal(t, "auto", gjson.GetBytes(run.upstream.lastBody, "tools.0.size").String())
	require.Equal(t, "low", gjson.GetBytes(run.upstream.lastBody, "tools.0.quality").String())
	require.Equal(t, "1672x941", gjson.Get(run.recorder.Body.String(), "size").String())
	require.Equal(t, "auto", gjson.Get(run.recorder.Body.String(), "quality").String())
	require.Equal(t, []string{"1672x941"}, run.result.ImageOutputSizes)

	ApplyOpenAIImageBillingResolution(run.result)
	require.Equal(t, ImageBillingSize2K, run.result.ImageSize)
	require.Equal(t, "1672x941", run.result.ImageOutputSize)
	require.Equal(t, ImageSizeSourceOutput, run.result.ImageSizeSource)
}

func TestOpenAIGatewayServiceForwardImages_OAuthStreamingUsesDecodedOutputDimensions(t *testing.T) {
	run := runOpenAIOAuthImageActualSizeTest(t, true, "auto")

	require.NoError(t, run.err)
	require.NotNil(t, run.result)
	events := parseOpenAIImageTestSSEEvents(run.recorder.Body.String())
	completed, ok := findOpenAIImageTestSSEEvent(events, "image_generation.completed")
	require.True(t, ok)
	require.Equal(t, "1672x941", gjson.Get(completed.Data, "size").String())
	require.Equal(t, "auto", gjson.Get(completed.Data, "quality").String())
	require.Equal(t, []string{"1672x941"}, run.result.ImageOutputSizes)
}

func TestOpenAIGatewayServiceForwardImages_OAuthReturnsActualSizeForExplicitSizeMismatch(t *testing.T) {
	run := runOpenAIOAuthImageActualSizeTest(t, false, "2048x1152")

	require.NoError(t, run.err)
	require.NotNil(t, run.result)
	require.Equal(t, 1, run.result.ImageCount)
	require.Equal(t, []string{"1672x941"}, run.result.ImageOutputSizes)
	require.Equal(t, "gpt-5.6", gjson.GetBytes(run.upstream.lastBody, "model").String())
	require.Equal(t, "2048x1152", gjson.GetBytes(run.upstream.lastBody, "tools.0.size").String())
	require.Equal(t, http.StatusOK, run.recorder.Code)
	require.Equal(t, "1672x941", gjson.Get(run.recorder.Body.String(), "size").String())
}

func TestOpenAIGatewayServiceForwardImages_OAuthStreamingReturnsActualSizeForExplicitSizeMismatch(t *testing.T) {
	run := runOpenAIOAuthImageActualSizeTest(t, true, "2048x1152")

	require.NoError(t, run.err)
	require.NotNil(t, run.result)
	require.Equal(t, 1, run.result.ImageCount)
	require.Equal(t, []string{"1672x941"}, run.result.ImageOutputSizes)
	events := parseOpenAIImageTestSSEEvents(run.recorder.Body.String())
	completedEvent, ok := findOpenAIImageTestSSEEvent(events, "image_generation.completed")
	require.True(t, ok)
	require.Equal(t, "1672x941", gjson.Get(completedEvent.Data, "size").String())
	_, hasError := findOpenAIImageTestSSEEvent(events, "error")
	require.False(t, hasError)
}

func TestOpenAIGatewayServiceForwardImages_APIKeyReturnsDecodedB64ActualSize(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-image-2","prompt":"draw a landscape","size":"2048x1152","response_format":"b64_json"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req

	encoded := encodeOpenAIImageTestPNG(t, 1089, 1448)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"X-Request-Id": []string{"req_img_apikey_size_mismatch"},
		},
		Body: io.NopCloser(strings.NewReader(fmt.Sprintf(
			`{"created":1710000007,"usage":{"input_tokens":12,"output_tokens":21,"output_tokens_details":{"image_tokens":9}},"data":[{"b64_json":%q,"revised_prompt":"draw a landscape"}]}`,
			encoded,
		))),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)
	account := &Account{
		ID:       6,
		Name:     "openai-apikey",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "test-api-key",
			"base_url": "https://image-upstream.example/v1",
		},
	}

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, OpenAIUsage{InputTokens: 12, OutputTokens: 21, ImageOutputTokens: 9}, result.Usage)
	require.Equal(t, []string{"1089x1448"}, result.ImageOutputSizes)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "1089x1448", gjson.Get(rec.Body.String(), "data.0.size").String())
	require.True(t, gjson.Get(rec.Body.String(), "data.0.b64_json").Exists(), rec.Body.String())
	require.Equal(t, "2048x1152", gjson.GetBytes(upstream.lastBody, "size").String())
}

func TestOpenAIImagesAPIResponseResultsWithActualSizes_PreservesUnverifiableURLResult(t *testing.T) {
	body := []byte(`{"data":[{"url":"https://example.test/image.png"}]}`)
	results := openAIImagesAPIResponseResultsWithActualSizes(body)
	require.Len(t, results, 1)
	require.Empty(t, results[0].Size)
	require.JSONEq(t, string(body), string(annotateOpenAIImagesAPIResponseActualSizes(body, results)))
}

type openAIOAuthImageActualSizeTestRun struct {
	result   *OpenAIForwardResult
	recorder *httptest.ResponseRecorder
	upstream *httpUpstreamRecorder
	err      error
}

func runOpenAIOAuthImageActualSizeTest(t *testing.T, stream bool, requestSize string) openAIOAuthImageActualSizeTestRun {
	t.Helper()
	gin.SetMode(gin.TestMode)
	body := []byte(fmt.Sprintf(`{"model":"gpt-image-2","prompt":"draw a test chart","size":%q,"quality":"low","output_format":"png","stream":%t}`, requestSize, stream))
	req := httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = req
	c.Set("api_key", &APIKey{ID: 42})

	encoded := encodeOpenAIImageTestPNG(t, 1672, 941)
	upstreamBody := fmt.Sprintf(
		"data: {\"type\":\"response.created\",\"response\":{\"created_at\":1710000000,\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2\",\"size\":\"auto\",\"quality\":\"auto\",\"output_format\":\"png\"}]}}\n\n"+
			"data: {\"type\":\"response.completed\",\"response\":{\"created_at\":1710000000,\"tools\":[{\"type\":\"image_generation\",\"model\":\"gpt-image-2\",\"size\":\"auto\",\"quality\":\"auto\",\"output_format\":\"png\"}],\"output\":[{\"id\":\"ig_actual_size\",\"type\":\"image_generation_call\",\"result\":%q}]}}\n\n"+
			"data: [DONE]\n\n",
		encoded,
	)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
			"X-Request-Id": []string{"req_img_actual_size"},
		},
		Body: io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	parsed, err := svc.ParseOpenAIImagesRequest(c, body)
	require.NoError(t, err)

	account := &Account{
		ID:       1,
		Name:     "openai-oauth",
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "token-123",
			"chatgpt_account_id": "acct-123",
		},
	}
	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")
	return openAIOAuthImageActualSizeTestRun{result: result, recorder: rec, upstream: upstream, err: err}
}

func encodeOpenAIImageTestPNG(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.SetNRGBA(0, 0, color.NRGBA{R: 0xff, A: 0xff})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func encodeOpenAIImageTestJPEG(t *testing.T, width, height int) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	img.SetNRGBA(0, 0, color.NRGBA{G: 0xff, A: 0xff})
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func encodeOpenAIImageTestWebPVP8X(width, height int) string {
	header := make([]byte, 30)
	copy(header[0:4], "RIFF")
	copy(header[8:12], "WEBP")
	copy(header[12:16], "VP8X")
	width--
	height--
	header[24], header[25], header[26] = byte(width), byte(width>>8), byte(width>>16)
	header[27], header[28], header[29] = byte(height), byte(height>>8), byte(height>>16)
	return base64.StdEncoding.EncodeToString(header)
}

func encodeOpenAIImageTestWebPVP8(width, height int) string {
	header := make([]byte, 30)
	copy(header[0:4], "RIFF")
	copy(header[8:12], "WEBP")
	copy(header[12:16], "VP8 ")
	copy(header[23:26], "\x9d\x01\x2a")
	binary.LittleEndian.PutUint16(header[26:28], uint16(width))
	binary.LittleEndian.PutUint16(header[28:30], uint16(height))
	return base64.StdEncoding.EncodeToString(header)
}

func encodeOpenAIImageTestWebPVP8L(width, height int) string {
	header := make([]byte, 25)
	copy(header[0:4], "RIFF")
	copy(header[8:12], "WEBP")
	copy(header[12:16], "VP8L")
	header[20] = 0x2f
	width--
	height--
	header[21] = byte(width)
	header[22] = byte(width>>8)&0x3f | byte(height&0x03)<<6
	header[23] = byte(height >> 2)
	header[24] = byte(height>>10) & 0x0f
	return base64.StdEncoding.EncodeToString(header)
}
