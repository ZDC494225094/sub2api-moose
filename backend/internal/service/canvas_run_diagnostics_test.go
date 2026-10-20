package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestCanvasDiagnosticsSurviveRefundAndRedisLoss(t *testing.T) {
	r := &canvasRepoStub{}
	billing, key := canvasTestBilling(r)
	s := ProvidePlaygroundRunService(nil, nil, nil, billing)
	defer s.Stop()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"reference video format unsupported https://bucket.test/key?signature=SECRET"}}`))
	}))
	defer server.Close()
	_, err := s.Start(7, PlaygroundRunRequest{CanvasKey: key, ID: "diagnostic-run", Mode: "video", Model: "video-model", APIKey: key.Key}, server.URL)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.charge != nil && r.charge.Status == "failed"
	}, 3*time.Second, 10*time.Millisecond)
	// A new instance has neither the producer's memory nor Redis snapshot.
	reader := NewPlaygroundRunService()
	reader.canvasBilling = billing
	run, found := reader.Get(7, "diagnostic-run")
	require.True(t, found)
	require.Equal(t, PlaygroundRunFailed, run.Status)
	require.Equal(t, "upstream_submit", run.Phase)
	require.Contains(t, run.Error, "reference video format unsupported")
	require.NotContains(t, run.Error, "SECRET")
	_, found = reader.Get(8, "diagnostic-run")
	require.False(t, found)
	// A stale local running snapshot must not hide a durable failure forever.
	reader.runs[playgroundRunKey(7, run.ID)] = &PlaygroundRun{ID: run.ID, Status: PlaygroundRunRunning}
	run, found = reader.Get(7, run.ID)
	require.True(t, found)
	require.Equal(t, PlaygroundRunFailed, run.Status)
}

func TestCanvasDiagnosticsRedisStartupFailure(t *testing.T) {
	cache := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: cache.Addr(), MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	cache.Close()
	r := &canvasRepoStub{}
	billing, key := canvasTestBilling(r)
	s := ProvidePlaygroundRunService(client, nil, nil, billing)
	defer s.Stop()
	_, err := s.Start(7, PlaygroundRunRequest{CanvasKey: key, ID: "redis-failure", Mode: "video", Model: "video-model", APIKey: key.Key}, "http://127.0.0.1:1")
	require.ErrorContains(t, err, "Redis")
	run, found := s.Get(7, "redis-failure")
	require.True(t, found)
	require.Equal(t, "task_persist", run.Phase)
	require.Contains(t, run.Error, "Redis")
	require.Equal(t, 1, r.refunds)
}

func TestCanvasDiagnosticsBrokenStorageNeverSubmits(t *testing.T) {
	r := &canvasRepoStub{}
	billing, key := canvasTestBilling(r)
	s := ProvidePlaygroundRunService(nil, nil, nil, billing)
	defer s.Stop()
	s.videoStorageResolver = func() (*ImageResultUploader, bool, error) {
		return nil, false, errors.New("bucket configuration missing")
	}
	_, err := s.Start(7, PlaygroundRunRequest{CanvasKey: key, ID: "storage-failure", Mode: "video", Model: "video-model", APIKey: key.Key, Images: []PlaygroundRunImageInput{{DataURL: videoInputDataURL()}}}, "http://127.0.0.1:1")
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		run, found := s.Get(7, "storage-failure")
		return found && run.Status == PlaygroundRunFailed && run.Phase == "reference_upload" && run.Error != ""
	}, 3*time.Second, 10*time.Millisecond)
	reader := NewPlaygroundRunService()
	reader.canvasBilling = billing
	run, found := reader.Get(7, "storage-failure")
	require.True(t, found)
	require.Contains(t, run.Error, "bucket configuration missing")
}

func TestCanvasDiagnosticsPreserveCancellationCause(t *testing.T) {
	r := &canvasRepoStub{charge: &CanvasCharge{ID: "charge", RunID: "canceled", UserID: 7, Status: "reserved"}}
	billing, _ := canvasTestBilling(r)
	s := NewPlaygroundRunService()
	s.canvasBilling = billing
	s.runs[playgroundRunKey(7, "canceled")] = &PlaygroundRun{ID: "canceled", Phase: "upstream_poll", UpstreamTaskID: "provider-task"}
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("任务计费心跳续期失败"))
	charge, err := s.settleCanvasCharge(ctx, PlaygroundRunRequest{ID: "canceled", canvasCharge: r.charge}, context.Cause(ctx))
	require.NoError(t, err)
	require.Equal(t, "failed", charge.Status)
	require.Equal(t, "provider-task", charge.UpstreamTaskID)
	require.Contains(t, charge.ErrorMessage, "心跳续期失败")
}

func TestCanvasDiagnosticsRedactSecrets(t *testing.T) {
	message := `HTTP 400 invalid video data:video/webm;base64,SECRETMEDIA https://example.test/file?X-Amz-Signature=SECRETURL Authorization: Bearer SECRETBEARER api_key=SECRETKEY sk-SECRETSK`
	got := sanitizeCanvasRunError(message)
	require.Contains(t, got, "HTTP 400 invalid video")
	for _, secret := range []string{"SECRETMEDIA", "SECRETURL", "SECRETBEARER", "SECRETKEY", "SECRETSK"} {
		require.NotContains(t, got, secret)
	}
	require.Equal(t, "task-provider-123", sanitizeCanvasRunError("task-provider-123"))
}
