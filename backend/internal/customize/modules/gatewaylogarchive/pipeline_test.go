package gatewaylogarchive

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testPipeline(t *testing.T, store *memStore, objects *memObjects, dir string) *pipeline {
	t.Helper()
	cfg := DefaultConfig()
	require.NoError(t, cfg.Normalize())
	p, err := startPipeline(store, func(BucketConfig) (objectStore, error) { return objects, nil }, "node-a", dir,
		func(time.Time) string { return "2026-06-25" }, cfg, func(err error) { t.Log(err) }, &stats{})
	require.NoError(t, err)
	p.setBuckets([]BucketConfig{{ID: 1, Name: "primary", Bucket: "test-bucket", Enabled: true, Priority: 100}})
	return p
}

func exchange(session, model, modelKey, body string) *Exchange {
	now := time.Now()
	x := &Exchange{RequestID: "r-" + body, SessionID: session, Model: model, ModelKey: modelKey, APIKeyID: 7,
		Method: "POST", Path: "/v1/messages", Transport: "http", StartedAt: now, FinishedAt: now, Status: 200,
		RequestType: "application/json", ResponseType: "application/json",
		Request: []byte(`{"model":"` + model + `","n":"` + body + `"}`), Response: []byte(`{"id":"` + body + `"}`)}
	finishExchange(x)
	return x
}

type sessionDoc struct {
	SessionID string            `json:"session_id"`
	ModelKey  string            `json:"model_key"`
	Count     int               `json:"exchange_count"`
	Exchanges []json.RawMessage `json:"exchanges"`
}

func jsonlDocs(t *testing.T, data []byte) map[string]sessionDoc {
	t.Helper()
	require.NotEmpty(t, data)
	require.Equal(t, byte('\n'), data[len(data)-1])
	out := map[string]sessionDoc{}
	for i, raw := range bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'}) {
		var doc sessionDoc
		require.NoError(t, json.Unmarshal(raw, &doc), string(raw))
		out[EntryName(i+1)] = doc
	}
	return out
}

func TestPipelineArchivesOneDocumentPerLineAndOneJSONLPerModel(t *testing.T) {
	store, objects := newMemStore(), newMemObjects()
	p := testPipeline(t, store, objects, t.TempDir())
	for _, x := range []*Exchange{
		exchange("s1", "claude-opus-5", "Opus5", "1"),
		exchange("s1", "claude-opus-5", "Opus5", "2"),
		exchange("s2", "claude-opus-5-20261001", "Opus5", "3"),
		exchange("s1", "gpt-6.1-sol", "gpt-6.1-sol", "4"),
		exchange("", "gpt-6.1-sol", "gpt-6.1-sol", "5"),
	} {
		require.True(t, p.submit(x))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	require.NoError(t, p.flushNow(ctx))
	require.Eventually(t, func() bool {
		for _, b := range store.bundleList() {
			if b.Status != StatusUploaded {
				return false
			}
		}
		return len(store.bundleList()) == 2
	}, 10*time.Second, 50*time.Millisecond)
	p.stop(false)

	opus := jsonlDocs(t, objects.objects["2026-06-25/Opus5-2026-06-25.jsonl"])
	require.Len(t, opus, 2, "two sessions share the Opus5 archive")
	counts := map[string]int{}
	for _, doc := range opus {
		require.Equal(t, "Opus5", doc.ModelKey)
		require.Len(t, doc.Exchanges, doc.Count)
		counts[doc.SessionID] = doc.Count
	}
	require.Equal(t, map[string]int{"s1": 2, "s2": 1}, counts)
	var first map[string]any
	for _, doc := range opus {
		if doc.SessionID == "s1" {
			require.NoError(t, json.Unmarshal(doc.Exchanges[0], &first))
		}
	}
	require.Equal(t, map[string]any{"model": "claude-opus-5", "n": "1"}, first["request"], "request JSON is embedded as-is")

	gpt := jsonlDocs(t, objects.objects["2026-06-25/gpt-6.1-sol-2026-06-25.jsonl"])
	require.Len(t, gpt, 2, "a request without session id is its own document")
	for _, b := range store.bundleList() {
		require.True(t, b.LocalDeleted, "local copies are removed after upload by default")
	}
}

func TestPipelineRecoversSessionsAndOpenBundlesAfterRestart(t *testing.T) {
	store, objects := newMemStore(), newMemObjects()
	dir := t.TempDir()
	p := testPipeline(t, store, objects, dir)
	require.True(t, p.submit(exchange("s1", "m", "m", "1")))
	require.True(t, p.submit(exchange("", "m", "m", "2")))
	require.Eventually(t, func() bool {
		bs := store.bundleList()
		return len(bs) == 1 && bs[0].EntryCount == 1
	}, 10*time.Second, 50*time.Millisecond, "the per-request document is flushed immediately")
	p.stop(false) // process exit: the s1 spool and the open bundle stay on disk

	files, _ := filepath.Glob(filepath.Join(dir, "spool", "*.spool"))
	require.Len(t, files, 1)
	p = testPipeline(t, store, objects, dir)
	require.True(t, p.submit(exchange("s1", "m", "m", "3")))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	require.NoError(t, p.flushNow(ctx))
	require.Eventually(t, func() bool {
		bs := store.bundleList()
		return len(bs) == 1 && bs[0].Status == StatusUploaded
	}, 10*time.Second, 50*time.Millisecond)
	p.stop(false)
	docs := jsonlDocs(t, objects.objects["2026-06-25/m-2026-06-25.jsonl"])
	require.Len(t, docs, 2)
	require.Equal(t, 2, docs[EntryName(2)].Count, "the session continued across the restart in one document")
}

func TestPipelineRollsBundlesAtSizeLimit(t *testing.T) {
	store, objects := newMemStore(), newMemObjects()
	p := testPipeline(t, store, objects, t.TempDir())
	p.maxFile.Store(1) // every document fills a bundle
	for _, n := range []string{"1", "2", "3"} {
		require.True(t, p.submit(exchange("", "m", "m", n)))
	}
	require.Eventually(t, func() bool {
		n := 0
		for _, b := range store.bundleList() {
			if b.Status == StatusUploaded {
				n++
			}
		}
		return n == 3
	}, 10*time.Second, 50*time.Millisecond)
	p.stop(false)
	for _, key := range []string{"2026-06-25/m-2026-06-25.jsonl", "2026-06-25/m-2026-06-25-part002.jsonl", "2026-06-25/m-2026-06-25-part003.jsonl"} {
		require.Contains(t, objects.objects, key)
	}
}

func TestUploadPicksNextNameWhenObjectExists(t *testing.T) {
	store, objects := newMemStore(), newMemObjects()
	objects.objects["2026-06-25/m-2026-06-25.jsonl"] = []byte("from another deployment")
	p := testPipeline(t, store, objects, t.TempDir())
	require.True(t, p.submit(exchange("", "m", "m", "1")))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	require.NoError(t, p.flushNow(ctx))
	require.Eventually(t, func() bool {
		bs := store.bundleList()
		return len(bs) == 1 && bs[0].Status == StatusUploaded
	}, 10*time.Second, 50*time.Millisecond)
	p.stop(false)
	require.Equal(t, "from another deployment", string(objects.objects["2026-06-25/m-2026-06-25.jsonl"]))
	require.Equal(t, "2026-06-25/m-2026-06-25-part002.jsonl", store.bundleList()[0].ObjectKey)
}
