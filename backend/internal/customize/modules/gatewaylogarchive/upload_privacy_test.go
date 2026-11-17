package gatewaylogarchive

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func waitUploaded(t *testing.T, store *memStore, count int) {
	t.Helper()
	require.Eventually(t, func() bool {
		rows := store.bundleList()
		if len(rows) != count {
			return false
		}
		for _, row := range rows {
			if row.Status != StatusUploaded {
				return false
			}
		}
		return true
	}, 10*time.Second, 10*time.Millisecond)
}

func TestPrivacyRunsOnlyDuringUploadAndRemoteRangesUseRedactedOffsets(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
			store, objects := newMemStore(), newMemObjects()
			cfg := DefaultConfig()
			cfg.RedactPII, cfg.KeepLocalHours = enabled, 24
			p, err := startPipeline(store, func(BucketConfig) (objectStore, error) { return objects, nil }, "node-a", t.TempDir(), func(time.Time) string { return "2026-11-17" }, cfg, func(err error) { t.Log(err) }, &stats{})
			require.NoError(t, err)
			defer p.stop(false)
			r := captureRuntime(cfg, &captured{})
			r.policy.Load().submit = p.submit
			engine := gatewayEngine(r)
			requests := []string{
				"{\n  \"model\": \"m\", \"profile\":{\"name\":\"张三\",\"email\":\"alice@example.com\"},\"n\":9007199254740993\n}",
				`{"model":"m","text":"电话 13800138000，其他文字保持 <>& 原样","n":1.2300e+05}`,
			}
			for _, input := range requests {
				response := post(engine, "/v1/messages", input)
				require.Equal(t, 200, response.Code)
				require.Equal(t, `{"echo":`+input+`}`, response.Body.String(), "privacy configuration never modifies normal request or response")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			require.NoError(t, p.flushNow(ctx))
			rows := store.bundleList()
			require.Len(t, rows, 1)
			before, err := os.ReadFile(rows[0].LocalPath)
			require.NoError(t, err)
			require.Contains(t, string(before), "alice@example.com", "local pending JSONL is raw")
			require.Contains(t, string(before), "张三")
			require.Contains(t, string(before), "13800138000")
			bucket := BucketConfig{ID: 1, Bucket: objects.Bucket(), Enabled: true}
			p.setBuckets([]BucketConfig{bucket})
			p.wakeUploader()
			waitUploaded(t, store, 1)
			row := store.bundleList()[0]
			objects.mu.Lock()
			uploaded := append([]byte(nil), objects.objects[row.ObjectKey]...)
			objects.mu.Unlock()
			require.Len(t, jsonlDocs(t, uploaded), 2)
			require.Equal(t, int64(len(uploaded)), row.SizeBytes)
			after, err := os.ReadFile(row.LocalPath)
			require.NoError(t, err)
			require.Equal(t, before, after, "upload does not rewrite the captured local copy")
			if enabled {
				for _, pii := range []string{"alice@example.com", "13800138000", "张三"} {
					require.NotContains(t, string(uploaded), pii)
				}
				require.Contains(t, string(uploaded), piiMask)
			} else {
				require.Equal(t, before, uploaded)
			}
			require.Contains(t, string(uploaded), "9007199254740993")
			require.Contains(t, string(uploaded), "1.2300e+05")
			require.Contains(t, string(uploaded), "其他文字保持 <>& 原样")
			reader := &Runtime{nodeID: "node-a", objects: func(BucketConfig) (objectStore, error) { return objects, nil }, buckets: func(context.Context) ([]BucketConfig, error) { return []BucketConfig{bucket}, nil }}
			store.mu.Lock()
			entries := append([]Entry(nil), store.entries[row.ID]...)
			store.mu.Unlock()
			lines := strings.Split(strings.TrimSuffix(string(uploaded), "\n"), "\n")
			for i, entry := range entries {
				rc, err := reader.OpenEntry(ctx, entry, row)
				require.NoError(t, err)
				data, err := io.ReadAll(rc)
				require.NoError(t, err)
				require.NoError(t, rc.Close())
				require.Equal(t, lines[i], string(data), "uploaded preview uses remote offsets even when a raw local copy exists")
				require.Equal(t, int64(len(data)), entry.USize)
			}
		})
	}
}

func TestUploadFailureLeavesOriginalAndRetriesWithPrivacy(t *testing.T) {
	store, objects := newMemStore(), newMemObjects()
	objects.fail = errors.New("storage unavailable")
	dir := t.TempDir()
	p := testPipeline(t, store, objects, dir)
	flushAll(t, p, exchange("s", "m", "m", "alice@example.com"))
	require.Eventually(t, func() bool { rows := store.bundleList(); return len(rows) == 1 && rows[0].Status == StatusFailed }, 10*time.Second, 10*time.Millisecond)
	p.stop(false)
	row := store.bundleList()[0]
	data, err := os.ReadFile(row.LocalPath)
	require.NoError(t, err)
	require.Contains(t, string(data), "alice@example.com")
	require.NoFileExists(t, row.LocalPath+".upload-tmp")
	objects.fail = nil
	require.NoError(t, store.update(row.ID, func(b *Bundle) { b.Status = StatusSealed }))
	p = testPipeline(t, store, objects, dir)
	p.wakeUploader()
	waitUploaded(t, store, 1)
	p.stop(false)
	row = store.bundleList()[0]
	require.NotContains(t, string(objects.objects[row.ObjectKey]), "alice@example.com")
}

func TestInvalidUploadDocumentNeverFallsBackToRawData(t *testing.T) {
	store, objects := newMemStore(), newMemObjects()
	dir := t.TempDir()
	row, err := store.CreateBundle(context.Background(), "n", "2026-11-17", "m", func(name string) string { return filepath.Join(dir, name) })
	require.NoError(t, err)
	data := []byte(`{"email":"alice@example.com",broken}`)
	require.NoError(t, os.WriteFile(row.LocalPath, data, 0o600))
	require.NoError(t, store.CommitEntry(context.Background(), int64(len(data)), Entry{BundleID: row.ID, Seq: 1, Name: EntryName(1), USize: int64(len(data)), CSize: int64(len(data))}))
	row = store.bundleList()[0]
	p := &pipeline{store: store, ctx: context.Background(), objects: func(BucketConfig) (objectStore, error) { return objects, nil }}
	cfg := DefaultConfig()
	p.setConfig(cfg)
	err = p.uploadToBuckets([]BucketConfig{{ID: 1, Bucket: objects.Bucket()}}, &uploadPass{clients: map[int64]objectStore{}, down: map[int64]bool{}}, &row)
	require.Error(t, err)
	require.Zero(t, objects.uploads)
	require.NoFileExists(t, row.LocalPath+".upload-tmp")
	retained, err := os.ReadFile(row.LocalPath)
	require.NoError(t, err)
	require.Equal(t, data, retained)
}

func TestLegacyOpenZipConvertsOnUploadAndHistoricalZipCanStillBeRead(t *testing.T) {
	ctx := context.Background()
	store, objects := newMemStore(), newMemObjects()
	dir := t.TempDir()
	row, err := store.CreateBundle(ctx, "node-a", "2026-06-25", "m", func(name string) string { return filepath.Join(dir, strings.TrimSuffix(name, ".jsonl")+".zip") })
	require.NoError(t, err)
	require.NoError(t, store.update(row.ID, func(b *Bundle) { b.FileName = "m-2026-06-25.zip" }))
	z, err := createZip(row.LocalPath)
	require.NoError(t, err)
	for i, doc := range []string{`{"email":"alice@example.com","n":1}`, "{\n\"n\":2,\"phone\":\"13800138000\"\n}"} {
		e, err := z.Append(EntryName(i+1), time.Now(), strings.NewReader(doc))
		require.NoError(t, err)
		require.NoError(t, store.CommitEntry(ctx, z.Size(), Entry{BundleID: row.ID, Seq: i + 1, Name: e.Name, Offset: e.Offset, CSize: e.CSize, USize: e.USize, CRC: e.CRC, EndedAt: e.Mod}))
	}
	require.NoError(t, z.Close())
	p := testPipeline(t, store, objects, dir)
	p.wakeUploader()
	waitUploaded(t, store, 1)
	p.stop(false)
	row = store.bundleList()[0]
	require.Equal(t, "m-2026-06-25.jsonl", row.FileName)
	require.Equal(t, "2026-06-25/m-2026-06-25.jsonl", row.ObjectKey)
	require.Equal(t, "{\"email\":\"[REDACTED]\",\"n\":1}\n{\"n\":2,\"phone\":\"[REDACTED]\"}\n", string(objects.objects[row.ObjectKey]))

	// Already uploaded archives retain their historical ZIP format and offsets.
	path := filepath.Join(t.TempDir(), "historic.zip")
	z, err = createZip(path)
	require.NoError(t, err)
	e, err := z.Append(EntryName(1), time.Now(), strings.NewReader(`{"legacy":true}`))
	require.NoError(t, err)
	_, err = z.Seal()
	require.NoError(t, err)
	objects.objects["historic.zip"], err = os.ReadFile(path)
	require.NoError(t, err)
	r := &Runtime{objects: func(BucketConfig) (objectStore, error) { return objects, nil }, buckets: func(context.Context) ([]BucketConfig, error) { return []BucketConfig{{Bucket: objects.Bucket()}}, nil }}
	rc, err := r.OpenEntry(ctx, Entry{Name: e.Name, Offset: e.Offset, CSize: e.CSize, USize: e.USize}, Bundle{FileName: "historic.zip", Status: StatusUploaded, ObjectKey: "historic.zip", Bucket: objects.Bucket()})
	require.NoError(t, err)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	require.Equal(t, `{"legacy":true}`, string(data))
}

func TestStoreCommitsUploadLayoutAndStatsAtomically(t *testing.T) {
	s := sqliteStore(t)
	ctx := context.Background()
	b, err := s.CreateBundle(ctx, "n", "2026-11-17", "m", func(file string) string { return file })
	require.NoError(t, err)
	for seq := 1; seq <= 2; seq++ {
		require.NoError(t, s.CommitEntry(ctx, int64(seq*100), Entry{BundleID: b.ID, Seq: seq, Name: EntryName(seq), Offset: int64(seq * 10), USize: 100, CSize: 100, StartedAt: time.Now(), EndedAt: time.Now()}))
	}
	require.NoError(t, s.MarkSealed(ctx, b.ID, 200))
	layout := &uploadLayout{FileName: b.FileName, Size: 42, Entries: []zipEntry{{Offset: 0, CSize: 20, USize: 20, CRC: 12}, {Offset: 21, CSize: 20, USize: 20, CRC: 34}}}
	bad := *layout
	bad.Entries = bad.Entries[:1]
	require.Error(t, s.MarkUploaded(ctx, b.ID, 7, "test", "key", &bad))
	current, err := s.Bundle(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, StatusSealed, current.Status, "failed metadata changes roll back destination and status")
	require.NoError(t, s.MarkUploaded(ctx, b.ID, 7, "test", "key", layout))
	require.NoError(t, s.MarkUploaded(ctx, b.ID, 7, "test", "key", layout))
	entries, err := s.BundleEntries(ctx, b.ID)
	require.NoError(t, err)
	require.Equal(t, int64(21), entries[1].Offset)
	require.Equal(t, int64(20), entries[1].USize)
	usage, err := s.BucketUsage(ctx)
	require.NoError(t, err)
	require.Equal(t, BucketUsage{BundleCount: 1, SizeBytes: 42}, usage[7])
}

func TestStoreUpdatesAllUploadIndexBatches(t *testing.T) {
	s := sqliteStore(t)
	ctx := context.Background()
	b, err := s.CreateBundle(ctx, "n", "2026-11-17", "m", func(file string) string { return file })
	require.NoError(t, err)
	const count = 1005
	var entries []zipEntry
	for seq := 1; seq <= count; seq++ {
		require.NoError(t, s.CommitEntry(ctx, int64(seq*100), Entry{BundleID: b.ID, Seq: seq, Name: EntryName(seq), USize: 100, CSize: 100, StartedAt: time.Now(), EndedAt: time.Now()}))
		entries = append(entries, zipEntry{Offset: int64((seq - 1) * 21), USize: 20, CSize: 20, CRC: uint32(seq)})
	}
	require.NoError(t, s.MarkUploaded(ctx, b.ID, 1, "test", "key", &uploadLayout{FileName: b.FileName, Size: count * 21, Entries: entries}))
	stored, err := s.BundleEntries(ctx, b.ID)
	require.NoError(t, err)
	require.Len(t, stored, count)
	for i, e := range stored {
		require.Equal(t, entries[i].Offset, e.Offset)
		require.Equal(t, entries[i].CRC, e.CRC)
	}
}
