package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNativeVideoSerializers(t *testing.T) {
	request := PlaygroundRunRequest{Model: "alias", VideoStandard: "minimax-h3", VideoMode: "reference", Prompt: "move", Duration: 10, Resolution: "768p", AspectRatio: "16:9",
		Images:          []PlaygroundRunImageInput{{DataURL: "https://cdn.test/one.png"}},
		ReferenceVideos: []PlaygroundRunVideoInput{{DataURL: "https://cdn.test/two.mp4"}, {DataURL: "https://cdn.test/depth.webm"}},
		ReferenceAudios: []PlaygroundRunVideoInput{{DataURL: "https://cdn.test/three.mp3"}},
	}
	payload, err := playgroundVideoGenerationPayload(request, false)
	require.NoError(t, err)
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	converted, path, task, err := nativeVideoGeneration("minimax-h3", body)
	require.NoError(t, err)
	require.Equal(t, "/v2/video_generation", path)
	require.Equal(t, "/v2/query/video_generation/", task.Path)
	require.Len(t, gjson.GetBytes(converted, "content").Array(), 5)
	require.Equal(t, "reference_image", gjson.GetBytes(converted, "content.1.role").String())
	require.Equal(t, "768P", gjson.GetBytes(converted, "resolution").String())
	require.False(t, gjson.GetBytes(converted, "video_standard").Exists())

	for _, protocol := range []string{"minimax", "kling", "kling-v3", "omni"} {
		_, _, _, err := nativeVideoGeneration(protocol, body)
		require.Error(t, err, protocol+" must not silently discard references")
	}

	request.ReferenceVideos, request.ReferenceAudios = nil, nil
	request.VideoStandard, request.VideoMode = "kling-3", "frames"
	request.Images = append(request.Images, PlaygroundRunImageInput{DataURL: "https://cdn.test/last.png", Frame: "last"})
	request.Images[0].Frame = "first"
	payload, err = playgroundVideoGenerationPayload(request, false)
	require.NoError(t, err)
	body, _ = json.Marshal(payload)
	converted, path, _, err = nativeVideoGeneration("kling-v3", body)
	require.NoError(t, err)
	require.Equal(t, "/image-to-video/kling-3.0", path)
	require.Equal(t, "first_frame", gjson.GetBytes(converted, "contents.1.type").String())
	require.Equal(t, "last_frame", gjson.GetBytes(converted, "contents.2.type").String())
	require.False(t, gjson.GetBytes(converted, "first_frame").Exists())

	converted, _, _, err = nativeVideoGeneration("minimax", body)
	require.NoError(t, err)
	require.Equal(t, "https://cdn.test/last.png", gjson.GetBytes(converted, "last_frame_image").String())
	converted, _, _, err = nativeVideoGeneration("kling", body)
	require.NoError(t, err)
	require.Equal(t, "10", gjson.GetBytes(converted, "duration").String())
	require.Equal(t, "https://cdn.test/last.png", gjson.GetBytes(converted, "image_tail").String())
	converted, _, _, err = nativeVideoGeneration("omni", body)
	require.NoError(t, err)
	require.Equal(t, "image", gjson.GetBytes(converted, "input.0.type").String())
	require.Contains(t, gjson.GetBytes(converted, "input.2.text").String(), "<LAST_FRAME>@Image2")
	require.False(t, gjson.GetBytes(converted, "duration").Exists())
}

func TestArkFailureDetailsAndExpiry(t *testing.T) {
	for _, detail := range []string{`"source unavailable"`, `{"message":"source unavailable"}`, `["source unavailable"]`} {
		body := []byte(`{"id":"task-test","status":"expired","error":` + detail + `}`)
		legacy := normalizeArkVideoStatusResponse(body)
		require.Equal(t, "failed", gjson.GetBytes(legacy, "status").String())
		require.Contains(t, gjson.GetBytes(legacy, "error.message").String(), "source unavailable")
		native, _, err := normalizeNativeVideoResponse(body, nativeVideoTask{Protocol: "ark"}, true)
		require.NoError(t, err)
		require.Equal(t, "failed", gjson.GetBytes(native, "status").String())
		require.Contains(t, gjson.GetBytes(native, "error.message").String(), "source unavailable")
	}
}

func TestNativeVideoTaskAndResponse(t *testing.T) {
	for _, tt := range []struct{ protocol, path, response, output string }{
		{"minimax-h3", "/v2/query/video_generation/", `{"base_resp":{"status_code":0},"task":{"task_id":"42","status":"succeeded","content":{"url":"https://cdn.test/h3.mp4"},"duration":12}}`, "https://cdn.test/h3.mp4"},
		{"kling-v3", "/tasks?task_ids=", `{"code":0,"data":[{"id":"42","status":"succeeded","outputs":[{"type":"video","url":"https://cdn.test/kling.mp4"}]}]}`, "https://cdn.test/kling.mp4"},
		{"kling", "/v1/videos/image2video/", `{"code":0,"data":{"task_id":"42","task_status":"succeed","task_result":{"videos":[{"url":"https://cdn.test/old.mp4"}]}}}`, "https://cdn.test/old.mp4"},
		{"omni", "/v1beta/interactions/", `{"id":"42","status":"completed","steps":[{"type":"model_output","content":[{"type":"video","uri":"https://generativelanguage.googleapis.com/v1beta/files/42:download"}]}]}`, "https://generativelanguage.googleapis.com/v1beta/files/42:download"},
	} {
		t.Run(tt.protocol, func(t *testing.T) {
			task := nativeVideoTask{Protocol: tt.protocol, Path: tt.path, ID: "42", Model: "mapped", BillingModel: "original"}
			decoded, native, err := parseNativeVideoTask(task.key())
			require.NoError(t, err)
			require.True(t, native)
			require.Equal(t, task, decoded)
			body, current, err := normalizeNativeVideoResponse([]byte(tt.response), decoded, false)
			require.NoError(t, err)
			require.Equal(t, task.key(), current.key())
			require.Equal(t, "completed", gjson.GetBytes(body, "status").String())
			require.Equal(t, tt.output, gjson.GetBytes(body, "video.url").String())
			require.Equal(t, "original", gjson.GetBytes(body, "model").String())
		})
	}
	_, _, err := parseNativeVideoTask((nativeVideoTask{Protocol: "omni", Path: "https://evil.test/", ID: "42"}).key())
	require.Error(t, err)
	_, _, err = normalizeNativeVideoResponse([]byte(`{"base_resp":{"status_code":1008,"status_msg":"insufficient balance"}}`), nativeVideoTask{}, true)
	require.ErrorContains(t, err, "insufficient balance")
	_, err = nativeOmniContentURL("https://generativelanguage.googleapis.com", "https://evil.test/key")
	require.Error(t, err)
	_, err = nativeOmniContentURL("https://generativelanguage.googleapis.com", "https://generativelanguage.googleapis.com.evil.test/key")
	require.Error(t, err)
}

func TestNativeVideoProtocolAndManualProfiles(t *testing.T) {
	require.Equal(t, "", nativeVideoProtocol("https://api.wxart.space/v1", []byte(`{"model":"seedance2.0-fast"}`)))
	require.Equal(t, "", nativeVideoProtocol("https://api.wxart.space/v1", []byte(`{"model":"seedance2.0-fast","_video_protocol":"relay"}`)))
	require.Equal(t, "", nativeVideoProtocol("https://api.wxart.space.evil.test", []byte(`{"model":"seedance2.0-fast"}`)))
	require.Equal(t, "", nativeVideoProtocol("https://api.minimax.io", []byte(`{"_video_protocol":"relay"}`)))
	require.Equal(t, "minimax-h3", nativeVideoProtocol("https://relay.test/v1", []byte(`{"_video_protocol":"minimax-h3"}`)))
	require.Equal(t, "", nativeVideoProtocol("https://api.minimax.io.evil.test", []byte(`{}`)))
	require.Equal(t, "kling-v3", nativeVideoProtocol("https://api-singapore.klingai.com", []byte(`{"model":"alias","video_standard":"kling-o1"}`)))
	request := PlaygroundRunRequest{Model: "seedance-private-alias", VideoStandard: "custom", Duration: 40}
	require.NoError(t, normalizePlaygroundVideoRequest(&request))
	require.Equal(t, 40, request.Duration)
	request.VideoProtocol = "bad"
	require.Error(t, normalizePlaygroundVideoRequest(&request))
	request = PlaygroundRunRequest{VideoStandard: "veo-3.1", VideoMode: "frames", ReferenceAudios: []PlaygroundRunVideoInput{{DataURL: "https://cdn.test/audio.mp3"}}}
	_, err := playgroundVideoGenerationPayload(request, true)
	require.ErrorContains(t, err, "素材未截断")
	converted, _, _, err := nativeVideoGeneration("ark", []byte(`{"model":"alias","duration":8,"generate_audio":true,"watermark":false,"seed":123}`))
	require.NoError(t, err)
	require.Equal(t, gjson.True, gjson.GetBytes(converted, "generate_audio").Type)
	require.Equal(t, gjson.False, gjson.GetBytes(converted, "watermark").Type)
	require.Equal(t, int64(123), gjson.GetBytes(converted, "seed").Int())
}

func TestNativeVideoKlingVariants(t *testing.T) {
	for _, tt := range []struct{ standard, path string }{
		{"kling-o1", "/omni-video/kling-o1"},
		{"kling-3-turbo", "/image-to-video/kling-3.0-turbo"},
	} {
		t.Run(tt.standard, func(t *testing.T) {
			request := PlaygroundRunRequest{Model: "alias", VideoStandard: tt.standard, VideoMode: "frames", Prompt: "waves", Duration: 5, Resolution: "1080p", AspectRatio: "16:9", Images: []PlaygroundRunImageInput{{DataURL: "data:image/png;base64,YWJj", Frame: "first"}}}
			payload, err := playgroundVideoGenerationPayload(request, false)
			require.NoError(t, err)
			body, _ := json.Marshal(payload)
			converted, path, _, err := nativeVideoGeneration("kling-v3", body)
			require.NoError(t, err)
			require.Equal(t, tt.path, path)
			require.Equal(t, "YWJj", gjson.GetBytes(converted, "contents.1.url").String())
			require.False(t, gjson.GetBytes(converted, "settings.multi_shot").Exists())
			if tt.standard == "kling-3-turbo" {
				require.False(t, gjson.GetBytes(converted, "settings.audio").Exists())
			}
		})
	}
}

func TestNativeVideoSeedanceEdit(t *testing.T) {
	for _, mode := range []string{"edit", "extend", "frames"} {
		request := PlaygroundRunRequest{Model: "alias", VideoStandard: "seedance-2.5", VideoMode: mode, Duration: 10, AspectRatio: "16:9", Images: []PlaygroundRunImageInput{{DataURL: "https://cdn.test/frame.png", Frame: "first"}}}
		payload, err := playgroundVideoGenerationPayload(request, false)
		require.NoError(t, err)
		body, _ := json.Marshal(payload)
		converted, _, _, err := nativeVideoGeneration("ark", body)
		require.NoError(t, err)
		require.Equal(t, "adaptive", gjson.GetBytes(converted, "ratio").String())
		if mode == "edit" {
			require.Equal(t, int64(-1), gjson.GetBytes(converted, "duration").Int())
		}
	}
	request := PlaygroundRunRequest{Model: "alias", VideoStandard: "seedance-2", Duration: 10}
	payload, err := playgroundVideoGenerationPayload(request, false)
	require.NoError(t, err)
	require.NotContains(t, payload, "mode", "Kling-only mode must not leak to other relays")
	forwarded, _, err := sanitizeGrokMediaForwardBody(GrokMediaEndpointVideosGenerations, []byte(`{"model":"grok-imagine-video","_video_protocol":"auto","video_standard":"custom","video_mode":"frames","reference_videos":["https://cdn.test/a.mp4"]}`), "application/json")
	require.NoError(t, err)
	require.False(t, gjson.GetBytes(forwarded, "_video_protocol").Exists())
	require.False(t, gjson.GetBytes(forwarded, "video_standard").Exists())
	require.Len(t, gjson.GetBytes(forwarded, "reference_videos").Array(), 1)
}
