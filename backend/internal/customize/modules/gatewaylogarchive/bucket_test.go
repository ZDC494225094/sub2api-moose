package gatewaylogarchive

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func failoverPipeline(t *testing.T, store *memStore, byBucket map[int64]*memObjects, buckets []BucketConfig) *pipeline {
	t.Helper()
	cfg := DefaultConfig()
	require.NoError(t, cfg.Normalize())
	p, err := startPipeline(store, func(b BucketConfig) (objectStore, error) {
		objects, ok := byBucket[b.ID]
		if !ok {
			return nil, errors.New("no client")
		}
		return objects, nil
	}, "node-a", t.TempDir(), func(time.Time) string { return "2026-06-25" }, cfg, func(err error) { t.Log(err) }, &stats{})
	require.NoError(t, err)
	p.setBuckets(buckets)
	t.Cleanup(func() { p.stop(false) })
	return p
}

func flushAll(t *testing.T, p *pipeline, xs ...*Exchange) {
	t.Helper()
	for _, x := range xs {
		require.True(t, p.submit(x))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	require.NoError(t, p.flushNow(ctx))
	p.wakeUploader()
}

// A failing primary bucket does not block uploads: bundles move to the next
// enabled bucket and record where they went, so reads use the right bucket.
func TestUploadFallsOverToNextBucket(t *testing.T) {
	store := newMemStore()
	primary, secondary := newMemObjects(), newMemObjects()
	primary.name, secondary.name = "primary-bucket", "secondary-bucket"
	primary.fail = errors.New("503 SlowDown")
	p := failoverPipeline(t, store, map[int64]*memObjects{1: primary, 2: secondary}, []BucketConfig{
		{ID: 1, Name: "主存储", Bucket: "primary-bucket", Enabled: true, Priority: 1},
		{ID: 2, Name: "备用", Bucket: "secondary-bucket", Enabled: true, Priority: 2},
	})
	flushAll(t, p, exchange("s1", "claude-opus-5", "Opus5", "1"), exchange("s2", "gpt-6", "gpt-6", "2"))
	require.Eventually(t, func() bool {
		list := store.bundleList()
		for _, b := range list {
			if b.Status != StatusUploaded {
				return false
			}
		}
		return len(list) == 2
	}, 10*time.Second, 20*time.Millisecond)
	for _, b := range store.bundleList() {
		require.NotNil(t, b.BucketID)
		require.Equal(t, int64(2), *b.BucketID)
		require.Equal(t, "secondary-bucket", b.Bucket)
	}
	require.Len(t, secondary.objects, 2)
	require.Empty(t, primary.objects)
	require.GreaterOrEqual(t, primary.uploads, 1)
}

func TestUploadReportsEveryFailedBucket(t *testing.T) {
	store := newMemStore()
	a, b := newMemObjects(), newMemObjects()
	a.fail, b.fail = errors.New("access denied"), errors.New("timeout")
	p := failoverPipeline(t, store, map[int64]*memObjects{1: a, 2: b}, []BucketConfig{
		{ID: 1, Name: "A", Bucket: "a", Enabled: true},
		{ID: 2, Name: "B", Bucket: "b", Enabled: true},
	})
	flushAll(t, p, exchange("s1", "claude-opus-5", "Opus5", "1"))
	require.Eventually(t, func() bool {
		list := store.bundleList()
		return len(list) == 1 && list[0].Status == StatusFailed
	}, 10*time.Second, 20*time.Millisecond)
	msg := store.bundleList()[0].LastError
	require.Contains(t, msg, "A：access denied")
	require.Contains(t, msg, "B：timeout")
}

// sqliteStore applies the module migrations to an in-memory SQLite database,
// mapping the PostgreSQL-only column types, so the store SQL runs for real.
func sqliteStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", "file::memory:?cache=private")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TIMESTAMP)`)
	require.NoError(t, err)
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "sql", ID, "*.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	sort.Strings(files)
	for _, file := range files {
		raw, err := os.ReadFile(file)
		require.NoError(t, err)
		ddl := strings.NewReplacer("BIGSERIAL PRIMARY KEY", "INTEGER PRIMARY KEY AUTOINCREMENT", "TIMESTAMPTZ", "TIMESTAMP").Replace(string(raw))
		_, err = db.Exec(ddl)
		require.NoError(t, err, file)
	}
	return &Store{DB: db, Cipher: plainCipher{}}
}

func TestStoreBucketsAndUploadStatsOnSQLite(t *testing.T) {
	ctx := context.Background()
	s := sqliteStore(t)
	// A deployment that configured the single bucket of the previous version and
	// already uploaded one bundle (backfilled into the stats by the migration).
	_, err := s.DB.Exec(`UPDATE gateway_log_archive_config SET config = $1, access_key_id = 'enc:AK123456789', secret_access_key = 'enc:SK' WHERE id = 1`,
		`{"capture_enabled":true,"mode":"all","rules":[],"provider":"aws","bucket":"legacy-bucket","region":"us-east-1","prefix":"logs"}`)
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO gateway_log_archive_bundles (node_id, day, model_key, part, file_name, local_path, status, size_bytes, entry_count, bucket, object_key, uploaded_at)
		VALUES ('n', '2026-06-01', 'Opus5', 1, 'Opus5-2026-06-01.zip', '', 'uploaded', 1000, 4, 'legacy-bucket', 'logs/2026-06-01/Opus5-2026-06-01.zip', CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	_, err = s.DB.Exec(`INSERT INTO gateway_log_archive_upload_stats (bucket_id, model_key, bundle_count, entry_count, size_bytes) VALUES (0, 'Opus5', 1, 4, 1000)`)
	require.NoError(t, err)

	buckets, err := s.Buckets(ctx)
	require.NoError(t, err)
	require.Len(t, buckets, 1)
	legacy := buckets[0]
	require.Equal(t, "legacy-bucket", legacy.Bucket)
	require.Equal(t, "logs", legacy.Prefix)
	require.Equal(t, "AK123456789", legacy.AccessKeyID, "credentials survive adoption")
	require.True(t, legacy.Enabled)
	cfg, _, err := s.Config(ctx)
	require.NoError(t, err)
	require.True(t, cfg.BucketsMigrated)
	require.Empty(t, cfg.Bucket, "legacy fields are cleared after adoption")
	require.True(t, cfg.CaptureEnabled, "capture settings are kept")

	old, _, err := s.ListBundles(ctx, BundleFilter{}, 1, 10)
	require.NoError(t, err)
	require.NotNil(t, old[0].BucketID)
	require.Equal(t, legacy.ID, *old[0].BucketID, "earlier uploads are attributed to the adopted bucket")

	second, err := s.CreateBucket(ctx, BucketConfig{Name: "备用", Provider: ProviderCompat, Bucket: "backup", Endpoint: "https://r2.example", Enabled: true, Priority: 200, AccessKeyID: "AK2", SecretAccessKey: "SK2"})
	require.NoError(t, err)
	var stored string
	require.NoError(t, s.DB.QueryRow(`SELECT secret_access_key FROM gateway_log_archive_buckets WHERE id = $1`, second.ID).Scan(&stored))
	require.Equal(t, "enc:SK2", stored, "secrets are encrypted at rest")

	// Upload two new bundles to the second bucket; a repeated MarkUploaded counts once.
	for i, model := range []string{"gpt-6", "Opus5"} {
		b, err := s.CreateBundle(ctx, "n", "2026-06-25", model, func(f string) string { return "/tmp/" + f })
		require.NoError(t, err)
		require.NoError(t, s.CommitEntry(ctx, int64(500*(i+1)), Entry{BundleID: b.ID, Seq: 1, Name: EntryName(1), StartedAt: time.Now(), EndedAt: time.Now()}))
		require.NoError(t, s.MarkSealed(ctx, b.ID, int64(500*(i+1))))
		require.NoError(t, s.MarkUploaded(ctx, b.ID, second.ID, "backup", "k"+model))
		require.NoError(t, s.MarkUploaded(ctx, b.ID, second.ID, "backup", "k"+model))
	}

	all, err := s.ModelStats(ctx, 0)
	require.NoError(t, err)
	require.Len(t, all, 2)
	require.Equal(t, "Opus5", all[0].ModelKey, "largest total first")
	require.Equal(t, int64(2000), all[0].SizeBytes)
	require.Equal(t, int64(2), all[0].BundleCount)
	require.Equal(t, int64(500), all[1].SizeBytes)
	require.NotNil(t, all[0].LastUploadedAt)

	onlySecond, err := s.ModelStats(ctx, second.ID)
	require.NoError(t, err)
	require.Len(t, onlySecond, 2)
	require.Equal(t, int64(1000), onlySecond[0].SizeBytes, "Opus5 in the second bucket only")
	require.Equal(t, int64(500), onlySecond[1].SizeBytes)

	usage, err := s.BucketUsage(ctx)
	require.NoError(t, err)
	require.Equal(t, BucketUsage{BundleCount: 1, SizeBytes: 1000}, usage[legacy.ID])
	require.Equal(t, BucketUsage{BundleCount: 2, SizeBytes: 1500}, usage[second.ID])

	// Prune removes old index rows but never the cumulative totals.
	require.NoError(t, s.MarkLocalDeleted(ctx, old[0].ID))
	require.NoError(t, s.Prune(ctx, time.Now().Add(time.Hour)))
	usage, err = s.BucketUsage(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1000), usage[legacy.ID].SizeBytes)

	second.Enabled, second.Priority = false, 1
	updated, err := s.UpdateBucket(ctx, second)
	require.NoError(t, err)
	require.False(t, updated.Enabled)
	buckets, err = s.Buckets(ctx)
	require.NoError(t, err)
	require.Equal(t, second.ID, buckets[0].ID, "listed in priority order")
	require.Len(t, enabledBuckets(buckets), 1)

	deleted, err := s.DeleteBucket(ctx, second.ID)
	require.NoError(t, err)
	require.True(t, deleted)
	usage, err = s.BucketUsage(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(1500), usage[second.ID].SizeBytes, "totals of a deleted bucket remain")
}
