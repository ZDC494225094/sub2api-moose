package gatewaylogarchive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func addExportFragment(t *testing.T, s *Store, objects *memObjects, node, day, model, sid string, userID, keyID int64, exchanges ...json.RawMessage) Entry {
	t.Helper()
	ctx := context.Background()
	b, err := s.CreateBundle(ctx, node, day, model, func(file string) string { return filepath.Join(t.TempDir(), file) })
	require.NoError(t, err)
	start, err := time.Parse(time.RFC3339Nano, gjson.GetBytes(exchanges[0], "started_at").String())
	require.NoError(t, err)
	doc := rawJSON(map[string]any{"session_id": sid, "exchanges": exchanges})
	j, err := createJSONL(b.LocalPath)
	require.NoError(t, err)
	e, err := j.Append(EntryName(1), start, strings.NewReader(string(doc)))
	require.NoError(t, err)
	require.NoError(t, s.CommitEntry(ctx, j.Size(), Entry{BundleID: b.ID, Seq: 1, Name: e.Name, SessionID: sid, Model: model, UserID: userID, APIKeyID: keyID, ExchangeCount: len(exchanges), Offset: e.Offset, USize: e.USize, CSize: e.CSize, CRC: e.CRC, StartedAt: start, EndedAt: start}))
	size, err := j.Seal()
	require.NoError(t, err)
	require.NoError(t, s.MarkSealed(ctx, b.ID, size))
	key := day + "/" + b.FileName
	require.NoError(t, objects.Upload(ctx, key, b.LocalPath, func(int64) {}))
	require.NoError(t, s.MarkUploaded(ctx, b.ID, 1, objects.Bucket(), key, nil))
	entries, _, err := s.ListEntries(ctx, EntryFilter{BundleID: b.ID}, 1, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	return entries[0]
}

func exportExchangeFixture(id, at, request string, response any, stream bool) json.RawMessage {
	return rawJSON(map[string]any{"index": 1, "request_id": id, "started_at": at, "finished_at": at, "request": json.RawMessage(request), "response": response, "stream": stream, "status": 200, "transport": "http"})
}

func TestSessionExportMergesTwoNodesAcrossModelsDaysAndProtectsTenantScope(t *testing.T) {
	s, objects := sqliteStore(t), newMemObjects()
	first := exportExchangeFixture("req-1", "2026-11-16T23:59:00Z", `{"messages":[{"role":"user","content":"do work"}]}`, anthropicToolStream(), true)
	second := exportExchangeFixture("req-2", "2026-11-17T00:01:00Z", `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"done"}]}]}`, json.RawMessage(`{"id":"msg-2","role":"assistant","content":[{"type":"text","text":"finished"}],"stop_reason":"end_turn"}`), false)
	// Insert node B first to prove export order is not DB id or file order.
	anchor := addExportFragment(t, s, objects, "node-b", "2026-11-17", "model-b", "same-session", 3, 9, second)
	addExportFragment(t, s, objects, "node-a", "2026-11-16", "model-a", "same-session", 3, 9, first)
	addExportFragment(t, s, objects, "node-a", "2026-11-17", "model-c", "same-session", 3, 9, first)
	addExportFragment(t, s, objects, "node-a", "2026-11-17", "other-tenant", "same-session", 4, 10, first)
	addExportFragment(t, s, objects, "node-a", "2026-11-17", "other-key", "same-session", 3, 11, first)
	r := NewRuntime(s, nil, Hooks{}, t.TempDir())
	r.objects = func(BucketConfig) (objectStore, error) { return objects, nil }
	r.buckets = func(context.Context) ([]BucketConfig, error) {
		return []BucketConfig{{ID: 1, Bucket: objects.Bucket()}}, nil
	}
	export, err := r.ExportSession(context.Background(), anchor)
	require.NoError(t, err)
	defer export.Close()
	raw, err := io.ReadAll(export.File)
	require.NoError(t, err)
	require.True(t, json.Valid(raw))
	require.Equal(t, 3, export.Summary.FragmentCount)
	require.Equal(t, 2, export.Summary.NodeCount)
	require.Equal(t, 2, export.Summary.ExchangeCount)
	require.Equal(t, 1, export.Summary.DuplicateCount)
	require.Equal(t, 2, export.Summary.AssistantTurns)
	require.Equal(t, 1, export.Summary.ValidToolCalls)
	require.Equal(t, 1, export.Summary.Signatures)
	require.NotContains(t, export.Summary.Warnings, "tool_result_without_matching_call")
	require.NotContains(t, export.Summary.Warnings, "tool_call_without_result_in_snapshot")
	require.Contains(t, export.Summary.Warnings, "active_or_unindexed_requests_not_included")
	require.Equal(t, "req-1", gjson.GetBytes(raw, "exchanges.0.request_id").String())
	require.Equal(t, "req-2", gjson.GetBytes(raw, "exchanges.1.request_id").String())
	require.Equal(t, "sig-13800138000", gjson.GetBytes(raw, "exchanges.0.analysis.normalized_response.content.0.signature").String())
	require.Equal(t, anthropicToolStream(), gjson.GetBytes(raw, "exchanges.0.response").String(), "raw SSE remains intact")
	require.Equal(t, sessionCorrelationID(9, "same-session"), gjson.GetBytes(raw, "correlation_id").String())
	require.NotContains(t, string(raw), "same-session", "raw SQL session identifier need not be exposed by merged export")
	files, err := filepath.Glob(filepath.Join(r.dir(), "session-export-source-*.tmp"))
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestSessionExportRefusesPendingAndUnreadableFragments(t *testing.T) {
	fragments := []sessionFragment{{Entry: Entry{ID: 1, USize: 12}, Bundle: Bundle{NodeID: "a", Status: StatusUploaded}}, {Entry: Entry{ID: 2, USize: 12}, Bundle: Bundle{NodeID: "b", Status: StatusSealed}}}
	called := false
	open := func(context.Context, Entry, Bundle) (io.ReadCloser, error) { called = true; return nil, os.ErrNotExist }
	_, err := buildSessionExport(context.Background(), t.TempDir(), fragments, open)
	require.ErrorContains(t, err, "尚未上传")
	require.False(t, called)
	fragments[1].Bundle.Status = StatusUploaded
	_, err = buildSessionExport(context.Background(), t.TempDir(), fragments, open)
	require.ErrorContains(t, err, "读取会话片段")
	require.True(t, called)
	fragments[0].Entry.USize = maxSessionExportBytes + 1
	_, err = buildSessionExport(context.Background(), t.TempDir(), fragments, open)
	require.ErrorContains(t, err, "未截断导出")
}

func TestSessionExportReportsMissingLinksAndPreservesDistinctRetries(t *testing.T) {
	one := exportExchangeFixture("same-request", "2026-11-17T00:00:00Z", `{"previous_response_id":"missing-parent","input":[{"type":"function_call_output","call_id":"missing-call","output":"x"}]}`, json.RawMessage(`{"id":"r1","status":"completed","output":[{"type":"message","role":"assistant","content":[]}]}`), false)
	two := exportExchangeFixture("same-request", "2026-11-17T00:00:01Z", `{}`, json.RawMessage(`{"id":"r2","choices":[{"message":{"role":"assistant","content":"retry"},"finish_reason":"length"}]}`), false)
	doc := rawJSON(map[string]any{"exchanges": []json.RawMessage{one, two}})
	f := sessionFragment{Entry: Entry{ID: 1, SessionID: "s", USize: int64(len(doc)), ExchangeCount: 2}, Bundle: Bundle{Status: StatusUploaded, NodeID: "a"}}
	export, err := buildSessionExport(context.Background(), t.TempDir(), []sessionFragment{f}, func(context.Context, Entry, Bundle) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(string(doc))), nil
	})
	require.NoError(t, err)
	defer export.Close()
	require.Equal(t, 2, export.Summary.ExchangeCount)
	require.Zero(t, export.Summary.DuplicateCount)
	for _, warning := range []string{"previous_response_not_in_snapshot", "tool_result_without_matching_call", "repeated_request_id_with_distinct_payload", "output_token_limit"} {
		require.Contains(t, export.Summary.Warnings, warning)
	}
}

func TestNewNodeFragmentsShareCorrelationButHaveDistinctSegmentIDs(t *testing.T) {
	store, objects := newMemStore(), newMemObjects()
	var docs []gjson.Result
	for _, node := range []string{"node-a", "node-b"} {
		cfg := DefaultConfig()
		p, err := startPipeline(store, func(BucketConfig) (objectStore, error) { return objects, nil }, node, t.TempDir(), func(time.Time) string { return "2026-11-17" }, cfg, func(err error) { t.Log(err) }, &stats{})
		require.NoError(t, err)
		p.setBuckets([]BucketConfig{{ID: 1, Bucket: objects.Bucket(), Enabled: true}})
		flushAll(t, p, exchange("s", "m", "m", node))
		waitUploaded(t, store, len(docs)+1)
		p.stop(false)
		row := store.bundleList()[len(docs)]
		docs = append(docs, gjson.ParseBytes(objects.objects[row.ObjectKey]))
	}
	require.Equal(t, docs[0].Get("correlation_id").String(), docs[1].Get("correlation_id").String())
	require.NotEqual(t, docs[0].Get("segment_id").String(), docs[1].Get("segment_id").String())
	require.NotEqual(t, docs[0].Get("node_id").String(), docs[1].Get("node_id").String())
	require.Equal(t, "node_fragment", docs[0].Get("scope").String())
}

func TestSessionExportHTTPUsesModuleGateAndFullDownload(t *testing.T) {
	s, objects := sqliteStore(t), newMemObjects()
	e := addExportFragment(t, s, objects, "node-a", "2026-11-17", "m", "s", 1, 7, exportExchangeFixture("r", "2026-11-17T00:00:00Z", `{}`, json.RawMessage(`{"role":"assistant","content":[{"type":"text","text":"ok"}]}`), false))
	r := NewRuntime(s, nil, Hooks{}, t.TempDir())
	r.objects = func(BucketConfig) (objectStore, error) { return objects, nil }
	r.buckets = func(context.Context) ([]BucketConfig, error) {
		return []BucketConfig{{ID: 1, Bucket: objects.Bucket()}}, nil
	}
	for _, enabled := range []string{"false", "true"} {
		engine := gin.New()
		(&Handler{Runtime: r}).Register(engine.Group("/api/v1/admin"), customize.NewManager(flagStore{enabled}))
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/gateway-log-archive/entries/%d/session", e.ID), nil))
		if enabled == "false" {
			require.Equal(t, 403, w.Code)
			continue
		}
		require.Equal(t, 200, w.Code, w.Body.String())
		require.Contains(t, w.Header().Get("Content-Disposition"), "session-")
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		require.True(t, json.Valid(w.Body.Bytes()))
		require.Equal(t, int64(1), gjson.GetBytes(w.Body.Bytes(), "summary.assistant_turns").Int())
	}
}
