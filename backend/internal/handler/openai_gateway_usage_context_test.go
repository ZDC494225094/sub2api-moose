package handler

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestSubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &GatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "client-request-123", gotClientRequestID)
	require.Equal(t, "request-456", gotRequestID)
}

func TestOpenAISubmitUsageRecordTaskCopiesRequestContext(t *testing.T) {
	parent := context.WithValue(context.Background(), ctxkey.ClientRequestID, "openai-client-request-123")
	parent = context.WithValue(parent, ctxkey.RequestID, "openai-request-456")

	var gotClientRequestID string
	var gotRequestID string
	h := &OpenAIGatewayHandler{}
	h.submitUsageRecordTask(parent, func(ctx context.Context) {
		gotClientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
		gotRequestID, _ = ctx.Value(ctxkey.RequestID).(string)
	})

	require.Equal(t, "openai-client-request-123", gotClientRequestID)
	require.Equal(t, "openai-request-456", gotRequestID)
}

func TestCanvasUsageNeverDropsUnderPoolOverflow(t *testing.T) {
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{WorkerCount: 1, QueueSize: 1, TaskTimeout: time.Minute, OverflowPolicy: config.UsageRecordOverflowPolicyDrop})
	t.Cleanup(pool.Stop)
	block := make(chan struct{})
	started := make(chan struct{})
	t.Cleanup(func() { close(block) })
	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(context.Context) { close(started); <-block }))
	<-started
	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(context.Context) { <-block }))
	charge := &service.CanvasCharge{ID: "verified", UserID: 7, APIKeyID: 9}
	parent, cancel := context.WithCancel(service.WithCanvasCharge(context.Background(), charge))
	cancel()
	openai := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
	generic := &GatewayHandler{usageRecordWorkerPool: pool}
	for _, submit := range []func(context.Context, service.UsageRecordTask){generic.submitUsageRecordTask, openai.submitUsageRecordTask, openai.submitMandatoryUsageRecordTask} {
		called := false
		submit(parent, func(ctx context.Context) {
			called = true
			require.NoError(t, ctx.Err())
			require.Same(t, charge, service.CanvasChargeFromContext(ctx))
		})
		require.True(t, called, "canvas money-critical usage must complete synchronously")
	}
}
