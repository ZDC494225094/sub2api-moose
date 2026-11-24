package iqmonitor

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGatewayCallerStreamsAndSendsOnlyTheTemplate(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer sk-probe", r.Header.Get("Authorization"))
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"答案：\"}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"42\"},\"finish_reason\":\"length\"}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()
	g := &GatewayCaller{BaseURL: func() string { return srv.URL }}
	out, err := g.Call(context.Background(), CallRequest{APIKey: "sk-probe", Model: "m", Prompt: "题目", ReasoningEffort: "high", Timeout: 5 * time.Second})
	require.NoError(t, err)
	require.Equal(t, "答案：42", out.Content)
	require.True(t, out.Truncated, "finish_reason=length marks truncation")
	require.EqualValues(t, 12, out.InputTokens)
	require.EqualValues(t, 3, out.OutputTokens)
	require.NotNil(t, out.FirstTokenMS)
	msgs := body["messages"].([]any)
	require.Len(t, msgs, 1, "no system prompt is injected")
	require.Equal(t, "user", msgs[0].(map[string]any)["role"])
	require.Equal(t, "high", body["reasoning_effort"])
	require.Equal(t, true, body["stream"])
	_, hasMax := body["max_tokens"]
	require.False(t, hasMax, "no output cap unless configured")
}

func TestGatewayCallerClassifiesErrors(t *testing.T) {
	status := http.StatusBadGateway
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = io.WriteString(w, `{"error":{"message":"upstream down"}}`)
	}))
	defer srv.Close()
	g := &GatewayCaller{BaseURL: func() string { return srv.URL }}
	_, err := g.Call(context.Background(), CallRequest{APIKey: "k", Model: "m", Prompt: "p", Timeout: 5 * time.Second})
	var ce *CallError
	require.True(t, errors.As(err, &ce))
	require.True(t, ce.Retryable)
	require.Contains(t, ce.Message, "upstream down")
	status = http.StatusBadRequest
	_, err = g.Call(context.Background(), CallRequest{APIKey: "k", Model: "m", Prompt: "p", Timeout: 5 * time.Second})
	require.True(t, errors.As(err, &ce))
	require.False(t, ce.Retryable)
}

func TestGatewayCallerTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The client disconnect is not always observed promptly on every
		// platform; the test releases the handler so Close can return.
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer srv.Close()
	defer close(release)
	g := &GatewayCaller{BaseURL: func() string { return srv.URL }}
	_, err := g.Call(context.Background(), CallRequest{APIKey: "k", Model: "m", Prompt: "p", Timeout: 100 * time.Millisecond})
	var ce *CallError
	require.True(t, errors.As(err, &ce))
	require.Contains(t, ce.Message, "超时")
	require.True(t, ce.Retryable)
}
