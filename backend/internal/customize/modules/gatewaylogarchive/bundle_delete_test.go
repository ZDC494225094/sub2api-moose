package gatewaylogarchive

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// stuckObjects never finishes an upload until its context is cancelled, like
// an endpoint that keeps resetting the connection.
type stuckObjects struct {
	*memObjects
	started   atomic.Int32
	cancelled atomic.Int32
}

func (o *stuckObjects) wait(ctx context.Context) error {
	o.started.Add(1)
	<-ctx.Done()
	o.cancelled.Add(1)
	return ctx.Err()
}

func (o *stuckObjects) Upload(ctx context.Context, _, _ string, _ func(int64)) error {
	return o.wait(ctx)
}

func (o *stuckObjects) UploadStream(ctx context.Context, _ string, _ int64, _ func(io.Writer) error, _ func(int64)) error {
	return o.wait(ctx)
}

func deletePipeline(t *testing.T, objects objectStore) (*Store, *pipeline, string) {
	t.Helper()
	store := sqliteStore(t)
	bucket, err := store.CreateBucket(context.Background(), BucketConfig{Name: "slow", Bucket: "slow-bucket", Enabled: true, Priority: 1})
	require.NoError(t, err)
	cfg := DefaultConfig()
	require.NoError(t, cfg.Normalize())
	dir := t.TempDir()
	p, err := startPipeline(store, func(BucketConfig) (objectStore, error) { return objects, nil }, "node-a", dir,
		func(time.Time) string { return "2026-11-22" }, cfg, func(err error) { t.Log(err) }, &stats{}, bucket)
	require.NoError(t, err)
	t.Cleanup(func() { p.stop(false) })
	return store, p, dir
}

func onlyBundle(t *testing.T, store *Store) Bundle {
	t.Helper()
	items, total, err := store.ListBundles(context.Background(), BundleFilter{}, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	return items[0]
}

func bundleCount(t *testing.T, store *Store) int {
	t.Helper()
	var n int
	require.NoError(t, store.DB.QueryRow(`SELECT COUNT(*) FROM gateway_log_archive_bundles`).Scan(&n))
	return n
}

// Deleting a bundle in the middle of a stuck upload stops that upload, removes
// the local file and the index, and is not counted as an upload failure.
func TestDeleteStopsStuckUploadAndFreesDisk(t *testing.T) {
	objects := &stuckObjects{memObjects: newMemObjects()}
	store, p, _ := deletePipeline(t, objects)
	flushAll(t, p, exchange("s1", "gpt-6-astra", "gpt-6-astra", "1"))
	require.Eventually(t, func() bool { return p.uploading.Load() != nil && objects.started.Load() == 1 }, 10*time.Second, 20*time.Millisecond)
	row := onlyBundle(t, store)
	require.Equal(t, StatusUploading, row.Status)
	up := p.uploading.Load()
	require.Equal(t, row.ID, up.BundleID)
	require.False(t, up.startedAt.IsZero())
	require.Equal(t, 1, up.attempt)
	require.Equal(t, "slow", up.bucket)
	_, err := os.Stat(row.LocalPath)
	require.NoError(t, err)

	requested, err := store.RequestDelete(context.Background(), row.ID)
	require.NoError(t, err)
	require.NotNil(t, requested.DeleteRequestedAt)
	again, err := store.RequestDelete(context.Background(), row.ID)
	require.NoError(t, err, "a repeated delete is accepted")
	require.NotNil(t, again.DeleteRequestedAt)
	p.cancelUpload(row.ID)

	require.Eventually(t, func() bool { return bundleCount(t, store) == 0 }, 10*time.Second, 20*time.Millisecond)
	require.EqualValues(t, 1, objects.cancelled.Load())
	_, err = os.Stat(row.LocalPath)
	require.True(t, os.IsNotExist(err), "the local archive is removed")
	require.Zero(t, p.stats.uploadErrors.Load())
	require.Nil(t, p.uploading.Load())
	var entries int
	require.NoError(t, store.DB.QueryRow(`SELECT COUNT(*) FROM gateway_log_archive_entries`).Scan(&entries))
	require.Zero(t, entries)
}

// Another node's delete request reaches the uploading node via the database.
func TestDeleteRequestedElsewhereCancelsUpload(t *testing.T) {
	objects := &stuckObjects{memObjects: newMemObjects()}
	store, p, _ := deletePipeline(t, objects)
	flushAll(t, p, exchange("s1", "gpt-6-astra", "gpt-6-astra", "1"))
	require.Eventually(t, func() bool { return objects.started.Load() == 1 }, 10*time.Second, 20*time.Millisecond)
	row := onlyBundle(t, store)
	_, err := store.RequestDelete(context.Background(), row.ID)
	require.NoError(t, err)
	// No cancelUpload call: the upload watcher polls the shared index.
	require.Eventually(t, func() bool { return bundleCount(t, store) == 0 }, 3*deleteWatchInterval, 50*time.Millisecond)
}

func TestDeleteFailedBundleAndRules(t *testing.T) {
	objects := newMemObjects()
	objects.fail = errors.New("read tcp: connection reset by peer")
	store, p, _ := deletePipeline(t, objects)
	ctx := context.Background()
	flushAll(t, p, exchange("s1", "gpt-6-astra", "gpt-6-astra", "1"))
	require.Eventually(t, func() bool { return onlyBundle(t, store).Status == StatusFailed }, 10*time.Second, 20*time.Millisecond)
	row := onlyBundle(t, store)

	_, err := store.RequestDelete(ctx, row.ID+100)
	require.ErrorIs(t, err, sql.ErrNoRows)
	_, err = store.RequestDelete(ctx, row.ID)
	require.NoError(t, err)
	retried, err := store.RetryBundle(ctx, row.ID)
	require.NoError(t, err)
	require.False(t, retried, "a deleted bundle cannot be queued again")
	claimed, err := store.ClaimUpload(ctx, "node-a", time.Now().Add(48*time.Hour))
	require.NoError(t, err)
	require.Nil(t, claimed)
	p.cancelUpload(row.ID)
	require.Eventually(t, func() bool { return bundleCount(t, store) == 0 }, 10*time.Second, 20*time.Millisecond)
	_, err = os.Stat(row.LocalPath)
	require.True(t, os.IsNotExist(err))

	// Open (being written) and uploaded bundles are not deletable here.
	for _, status := range []string{StatusOpen, StatusUploaded} {
		_, err = store.DB.Exec(`INSERT INTO gateway_log_archive_bundles (node_id, day, model_key, part, file_name, status) VALUES ('node-b', '2026-11-23', $1, 1, 'x.jsonl', $1)`, status)
		require.NoError(t, err)
		var id int64
		require.NoError(t, store.DB.QueryRow(`SELECT id FROM gateway_log_archive_bundles WHERE status = $1`, status).Scan(&id))
		_, err = store.RequestDelete(ctx, id)
		require.ErrorIs(t, err, errBundleNotDeletable, status)
	}
}

// An upload that completes as the delete arrives must not record the bundle.
func TestCommitAfterDeleteRequestIsRejected(t *testing.T) {
	store := sqliteStore(t)
	ctx := context.Background()
	b, err := store.CreateBundle(ctx, "node-a", "2026-11-22", "m", func(f string) string { return f })
	require.NoError(t, err)
	_, err = store.DB.Exec(`UPDATE gateway_log_archive_bundles SET status = 'uploading' WHERE id = $1`, b.ID)
	require.NoError(t, err)
	_, err = store.RequestDelete(ctx, b.ID)
	require.NoError(t, err)
	require.ErrorIs(t, store.MarkUploaded(ctx, b.ID, 1, "bucket", "key", nil), errBundleDeleted)
	// The purge waits until the upload has released the row.
	require.NoError(t, store.purgeBundle(ctx, b.ID))
	require.Equal(t, 1, bundleCount(t, store))
	require.NoError(t, store.releaseDeleted(ctx, b.ID))
	require.NoError(t, store.purgeBundle(ctx, b.ID))
	require.Equal(t, 0, bundleCount(t, store))
}
