package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGrokVideoE2EDurationFromCreatedAt(t *testing.T) {
	t.Parallel()
	created := time.Now().UTC().Add(-45 * time.Second)
	d := GrokVideoE2EDuration(created.Format(time.RFC3339Nano), time.Now().UTC())
	require.GreaterOrEqual(t, d, 44*time.Second)
	require.LessOrEqual(t, d, 47*time.Second)

	require.Equal(t, time.Duration(0), GrokVideoE2EDuration("", time.Now()))
	require.Equal(t, time.Duration(0), GrokVideoE2EDuration("not-a-time", time.Now()))
	// Future CreatedAt clamps to zero (clock skew).
	require.Equal(t, time.Duration(0), GrokVideoE2EDuration(time.Now().Add(time.Hour).UTC().Format(time.RFC3339Nano), time.Now()))
}

func TestGrokVideoPendingCreatedAtStampOnStoreShape(t *testing.T) {
	t.Parallel()
	// GrokVideoPendingCreatedAtNow must be parseable by GrokVideoE2EDuration.
	stamp := GrokVideoPendingCreatedAtNow()
	require.NotEmpty(t, stamp)
	d := GrokVideoE2EDuration(stamp, time.Now().UTC().Add(2*time.Second))
	require.GreaterOrEqual(t, d, time.Second)
	require.LessOrEqual(t, d, 3*time.Second)
}

func TestIsGrokVideoStatusBillable(t *testing.T) {
	t.Parallel()
	// Official success: status=done + video.url
	require.True(t, IsGrokVideoStatusBillable([]byte(`{
		"status":"done",
		"model":"grok-imagine-video-1.5",
		"video":{"url":"https://vidgen.x.ai/x.mp4","duration":8,"respect_moderation":true}
	}`)))

	// Official non-success states
	require.False(t, IsGrokVideoStatusBillable(nil))
	require.False(t, IsGrokVideoStatusBillable([]byte(`{"status":"pending"}`)))
	require.False(t, IsGrokVideoStatusBillable([]byte(`{"status":"expired"}`)))
	require.False(t, IsGrokVideoStatusBillable([]byte(`{"status":"failed"}`)))
	// done without video.url is not billable
	require.False(t, IsGrokVideoStatusBillable([]byte(`{"status":"done"}`)))
	// URL alone (legacy/non-official shapes) is not enough
	require.False(t, IsGrokVideoStatusBillable([]byte(`{"url":"https://example.com/v.mp4"}`)))
	require.False(t, IsGrokVideoStatusBillable([]byte(`{"download_url":"/v1/videos/task/content"}`)))
	// "completed" is not the official xAI enum value.
	require.False(t, IsGrokVideoStatusBillable([]byte(`{"status":"completed","video":{"url":"https://vidgen.x.ai/x.mp4"}}`)))
	// Native Ark/Seedance normalizes a successful task to completed.
	require.True(t, IsOpenAICompatibleVideoStatusBillable([]byte(`{"status":"completed","video":{"url":"https://vidgen.x.ai/x.mp4"}}`)))
	// Other OpenAI-compatible Seedance relays can expose the terminal status directly.
	require.True(t, IsOpenAICompatibleVideoStatusBillable([]byte(`{"status":"succeeded","video":{"url":"https://vidgen.x.ai/x.mp4"}}`)))
	// Relays are not required to normalize their result into video.url.
	require.True(t, IsOpenAICompatibleVideoStatusBillable([]byte(`{"status":"succeeded","content":{"video_url":"https://cdn.example.test/seedance.mp4"}}`)))
	require.True(t, IsOpenAICompatibleVideoStatusBillable([]byte(`{"status":"finished","data":[{"url":"https://cdn.example.test/seedance.mp4"}]}`)))
	require.True(t, IsOpenAICompatibleVideoStatusBillable([]byte(`{"data":{"status":"succeeded","video_url":"https://cdn.example.test/seedance.mp4"}}`)))
	require.False(t, IsOpenAICompatibleVideoStatusBillable([]byte(`{"status":"succeeded","content":{}}`)))
}

func TestExtractGrokVideoBillingFromStatusBodyPrefersUpstreamParams(t *testing.T) {
	t.Parallel()
	pending := &GrokVideoPendingBilling{
		Model:                "pending-model",
		BillingModel:         "pending-billing",
		UpstreamModel:        "pending-upstream",
		VideoResolution:      VideoBillingResolution720P,
		VideoDurationSeconds: 8,
	}
	// Official completed body from docs.x.ai Video Generation.
	body := []byte(`{
		"status":"done",
		"model":"grok-imagine-video-1.5",
		"video":{"url":"https://vidgen.x.ai/signed.mp4","duration":12,"respect_moderation":true}
	}`)
	result := ExtractGrokVideoBillingFromStatusBody(body, pending, "req-1")
	require.NotNil(t, result)
	require.Equal(t, 1, result.VideoCount)
	require.Equal(t, "grok-imagine-video-1.5", result.Model)
	// Resolution is not in official status response — use create-time request.
	require.Equal(t, VideoBillingResolution720P, result.VideoResolution)
	// Duration prefers official video.duration.
	require.Equal(t, 12, result.VideoDurationSeconds)
}

func TestExtractGrokVideoBillingFromStatusBodyFallsBackToPending(t *testing.T) {
	t.Parallel()
	pending := &GrokVideoPendingBilling{
		Model:                "create-model",
		BillingModel:         "create-billing",
		UpstreamModel:        "create-upstream",
		VideoResolution:      VideoBillingResolution1080P,
		VideoDurationSeconds: 10,
	}
	// done + video.url, but no model/duration in body.
	body := []byte(`{"status":"done","video":{"url":"https://vidgen.x.ai/signed.mp4"}}`)
	result := ExtractGrokVideoBillingFromStatusBody(body, pending, "req-2")
	require.NotNil(t, result)
	require.Equal(t, "create-billing", result.BillingModel)
	require.Equal(t, "create-upstream", result.UpstreamModel)
	require.Equal(t, VideoBillingResolution1080P, result.VideoResolution)
	require.Equal(t, 10, result.VideoDurationSeconds)
}

func TestExtractGrokVideoBillingRejectsNonDoneStatus(t *testing.T) {
	t.Parallel()
	pending := &GrokVideoPendingBilling{Model: "m", VideoDurationSeconds: 8, VideoResolution: "720p"}
	require.Nil(t, ExtractGrokVideoBillingFromStatusBody(
		[]byte(`{"status":"pending","video":{"url":"https://vidgen.x.ai/x.mp4","duration":8}}`),
		pending, "req",
	))
	require.Nil(t, ExtractGrokVideoBillingFromStatusBody(
		[]byte(`{"status":"completed","video":{"url":"https://vidgen.x.ai/x.mp4","duration":8}}`),
		pending, "req",
	))
	compatible := ExtractOpenAICompatibleVideoBillingFromStatusBody(
		[]byte(`{"id":"seedance-task","status":"completed","video":{"url":"https://vidgen.x.ai/x.mp4","duration":8}}`),
		pending, "req",
	)
	require.NotNil(t, compatible)
	require.Equal(t, "seedance-task", compatible.ResponseID)
	require.Equal(t, 1, compatible.VideoCount)
	require.Equal(t, 8, compatible.VideoDurationSeconds)
	// Missing status models must be restored from the create-time snapshot,
	// never defaulted to the unrelated Grok video family.
	require.Equal(t, "m", compatible.Model)
	require.Equal(t, "m", compatible.BillingModel)
	withoutPending := ExtractOpenAICompatibleVideoBillingFromStatusBody(
		[]byte(`{"id":"seedance-task","status":"completed","video":{"url":"https://vidgen.x.ai/x.mp4","duration":8}}`),
		nil, "req",
	)
	require.NotNil(t, withoutPending)
	require.Empty(t, withoutPending.Model)
}

func TestExtractOpenAICompatibleVideoBillingSupportsRelayResponseShapes(t *testing.T) {
	t.Parallel()
	pending := &GrokVideoPendingBilling{
		Model:                "doubao-seedance-2-0-fast-260128",
		BillingModel:         "doubao-seedance-2-0-fast-260128",
		UpstreamModel:        "doubao-seedance-2-0-fast-260128",
		VideoResolution:      VideoBillingResolution1080P,
		VideoDurationSeconds: 10,
	}

	contentResult := ExtractOpenAICompatibleVideoBillingFromStatusBody([]byte(`{
		"id":"seedance-content-task",
		"status":"succeeded",
		"content":{"video_url":"https://cdn.example.test/content.mp4","duration":12}
	}`), pending, "")
	require.NotNil(t, contentResult)
	require.Equal(t, "seedance-content-task", contentResult.ResponseID)
	require.Equal(t, 1, contentResult.VideoCount)
	require.Equal(t, "doubao-seedance-2-0-fast-260128", contentResult.Model)
	require.Equal(t, VideoBillingResolution1080P, contentResult.VideoResolution)
	require.Equal(t, 12, contentResult.VideoDurationSeconds)

	dataResult := ExtractOpenAICompatibleVideoBillingFromStatusBody([]byte(`{
		"task_id":"seedance-data-task",
		"status":"finished",
		"data":[{"url":"https://cdn.example.test/data.mp4"}]
	}`), pending, "")
	require.NotNil(t, dataResult)
	require.Equal(t, "seedance-data-task", dataResult.ResponseID)
	require.Equal(t, 1, dataResult.VideoCount)
	// The relay omitted duration, so normal API-key billing reuses the create-time snapshot.
	require.Equal(t, 10, dataResult.VideoDurationSeconds)
}

func TestGrokMediaUsageFromResponseVideoCreateDoesNotBill(t *testing.T) {
	t.Parallel()
	info := GrokMediaRequestInfo{Model: "grok-imagine-video", Resolution: "720p", DurationSeconds: 10}
	meta := grokMediaUsageFromResponse(GrokMediaEndpointVideosGenerations, info, []byte(`{"request_id":"v1"}`))
	require.Equal(t, "v1", meta.ResponseID)
	require.Equal(t, 0, meta.VideoCount)
	require.Equal(t, 10, meta.VideoDurationSeconds)
	require.Equal(t, VideoBillingResolution720P, meta.VideoResolution)
}

func TestOpenAICompatibleVideoUsageFromResponseBillsCompletedGeneration(t *testing.T) {
	t.Parallel()
	info := GrokMediaRequestInfo{
		Model:           "doubao-seedance-2-0-fast-260128",
		Resolution:      VideoBillingResolution1080P,
		DurationSeconds: 10,
	}
	meta := openAICompatibleVideoUsageFromResponse(
		GrokMediaEndpointVideosGenerations,
		info,
		[]byte(`{
			"id":"sync-seedance-task",
			"content":{"video_url":"https://cdn.example.test/sync.mp4","duration":12}
		}`),
	)
	require.Equal(t, "sync-seedance-task", meta.ResponseID)
	require.Equal(t, 1, meta.VideoCount)
	require.Equal(t, VideoBillingResolution1080P, meta.VideoResolution)
	require.Equal(t, 12, meta.VideoDurationSeconds)
}

func TestGrokMediaUsageFromResponseVideoStatusBillsOnOfficialDone(t *testing.T) {
	t.Parallel()
	meta := grokMediaUsageFromResponse(
		GrokMediaEndpointVideoStatus,
		GrokMediaRequestInfo{},
		[]byte(`{"status":"done","model":"grok-imagine-video-1.5","video":{"url":"https://vidgen.x.ai/a.mp4","duration":9}}`),
	)
	require.Equal(t, 1, meta.VideoCount)
	require.Equal(t, 9, meta.VideoDurationSeconds)
	require.Equal(t, "grok-imagine-video-1.5", meta.Model)

	// Official non-done must not set billable units.
	pendingOnly := grokMediaUsageFromResponse(
		GrokMediaEndpointVideoStatus,
		GrokMediaRequestInfo{},
		[]byte(`{"status":"pending"}`),
	)
	require.Equal(t, 0, pendingOnly.VideoCount)

	// completed is not official xAI done.
	completed := grokMediaUsageFromResponse(
		GrokMediaEndpointVideoStatus,
		GrokMediaRequestInfo{},
		[]byte(`{"status":"completed","video":{"url":"https://vidgen.x.ai/a.mp4","duration":9}}`),
	)
	require.Equal(t, 0, completed.VideoCount)

	compatibleCompleted := openAICompatibleVideoUsageFromResponse(
		GrokMediaEndpointVideoStatus,
		GrokMediaRequestInfo{},
		[]byte(`{"id":"seedance-task","status":"completed","video":{"url":"https://vidgen.x.ai/a.mp4","duration":9}}`),
	)
	require.Equal(t, "seedance-task", compatibleCompleted.ResponseID)
	require.Equal(t, 1, compatibleCompleted.VideoCount)
	require.Equal(t, 9, compatibleCompleted.VideoDurationSeconds)
	require.Empty(t, compatibleCompleted.Model)

	relayCompleted := openAICompatibleVideoUsageFromResponse(
		GrokMediaEndpointVideoStatus,
		GrokMediaRequestInfo{},
		[]byte(`{"id":"relay-task","status":"succeeded","content":{"video_url":"https://cdn.example.test/a.mp4","duration":"11"}}`),
	)
	require.Equal(t, "relay-task", relayCompleted.ResponseID)
	require.Equal(t, 1, relayCompleted.VideoCount)
	require.Equal(t, 11, relayCompleted.VideoDurationSeconds)
	require.Empty(t, relayCompleted.Model)
}
