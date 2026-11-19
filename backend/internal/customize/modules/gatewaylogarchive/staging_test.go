package gatewaylogarchive

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Simulate a destination that resets every HTTP/2 upload, including attempts
// made by another worker. The scheduled retry must use its persisted error.
type http1RelayTarget struct{ *memObjects }

func (o http1RelayTarget) UploadStream(ctx context.Context, key string, size int64, produce func(io.Writer) error, progress func(int64)) error {
	if !relayProtocolFor(ctx).useHTTP1() {
		return errors.New(peerReset)
	}
	return o.memObjects.UploadStream(ctx, key, size, produce, progress)
}

func TestRustFSStagingFallbackRestartAndIndependentDeliveries(t *testing.T) {
	store := sqliteStore(t)
	ctx := context.Background()
	dir := t.TempDir()
	day := func(time.Time) string { return "2026-11-19" }
	_, err := store.DB.Exec(`PRAGMA foreign_keys=ON`)
	require.NoError(t, err)
	makeBucket := func(name, role, privacy string) BucketConfig {
		bucket, err := store.CreateBucket(ctx, BucketConfig{Bucket: name, StorageRole: role, PrivacyMode: privacy, Enabled: true})
		require.NoError(t, err)
		return bucket
	}
	primary := makeBucket("private-rustfs", "rustfs", "original")
	raw := makeBucket("trusted-copy", "destination", "original")
	masked := makeBucket("external-copy", "destination", "redact")
	_, err = store.CreateBucket(ctx, BucketConfig{Bucket: "second-primary", StorageRole: "rustfs", Enabled: true})
	require.Error(t, err)
	clients := map[int64]*memObjects{}
	for _, cfg := range []BucketConfig{primary, raw, masked} {
		client := newMemObjects()
		client.name = cfg.Bucket
		clients[cfg.ID] = client
	}
	factory := func(cfg BucketConfig) (objectStore, error) {
		if cfg.ID == masked.ID {
			return http1RelayTarget{clients[cfg.ID]}, nil
		}
		return clients[cfg.ID], nil
	}
	all, err := store.Buckets(ctx)
	require.NoError(t, err)
	var pipe *pipeline
	start := func() {
		pipe, err = startPipeline(store, factory, "node-a", dir, day, DefaultConfig(), func(err error) { t.Log(err) }, &stats{})
		require.NoError(t, err)
		pipe.setBuckets(all)
	}
	start()
	defer func() {
		if pipe != nil {
			pipe.stop(false)
		}
	}()
	appendSession := func(sid string) {
		x := exchange(sid, "m", "m", sid)
		x.Request = []byte(`{"email":"alice@example.com","keep":9007199254740993}`)
		require.True(t, pipe.submit(x))
		done := make(chan struct{})
		pipe.flushReq <- done
		<-done
		deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		require.NoError(t, pipe.waitIdle(deadline))
	}
	appendSession("one")
	items, _, err := store.ListBundles(ctx, BundleFilter{}, 1, 20)
	require.NoError(t, err)
	require.Len(t, items, 1)
	row := items[0]
	entries, _, err := store.ListEntries(ctx, EntryFilter{BundleID: row.ID}, 1, 10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.True(t, entries[0].StagedRemote)
	require.NoDirExists(t, row.LocalPath)
	preview := &Runtime{nodeID: "node-b", buckets: store.Buckets, objects: factory}
	body, err := preview.OpenEntry(ctx, entries[0], row)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	body.Close()
	require.Contains(t, string(data), "alice@example.com")
	clients[primary.ID].fail = errors.New("RustFS full")
	appendSession("two")
	entries, _, err = store.ListEntries(ctx, EntryFilter{BundleID: row.ID}, 1, 10)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.False(t, entries[1].StagedRemote)
	fallback := fallbackPath(row, entries[1].StageKey)
	require.FileExists(t, fallback)
	pureLocal, err := os.ReadFile(fallback)
	require.NoError(t, err)
	require.Contains(t, string(pureLocal), "alice@example.com")
	files, err := filepath.Glob(filepath.Join(pipe.spoolDir(), "*.spool"))
	require.NoError(t, err)
	require.Empty(t, files)
	pipe.stop(false)
	pipe = nil
	clients[primary.ID].fail = nil
	start()
	pipe.wakeUploader()
	require.Eventually(t, func() bool {
		jobs, err := store.deliveries(ctx, row.ID)
		return err == nil && len(jobs) == 2 && jobs[0].Status == "uploaded" && jobs[1].Status == "failed"
	}, 8*time.Second, 20*time.Millisecond)
	row, err = store.Bundle(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, StatusUploaded, row.Status)
	require.False(t, row.StageRedacted)
	require.Eventually(t, func() bool { r, _ := store.Bundle(ctx, row.ID); return r.LocalDeleted }, 3*time.Second, 10*time.Millisecond)
	pipe.stop(false)
	pipe = nil
	require.NoFileExists(t, fallback)
	require.Len(t, clients[primary.ID].objects, 1)
	master := clients[primary.ID].objects[row.ObjectKey]
	require.Len(t, jsonlDocs(t, master), 2)
	require.Contains(t, string(master), "alice@example.com")
	require.Empty(t, clients[masked.ID].objects)
	jobs, err := store.deliveries(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, master, clients[raw.ID].objects[jobs[0].Key])
	require.Contains(t, jobs[1].LastError, "写入目标桶 external-copy")
	rawUploads := clients[raw.ID].uploads
	require.NoError(t, store.Prune(ctx, time.Now().Add(time.Hour)))
	_, err = store.Bundle(ctx, row.ID)
	require.NoError(t, err, "pending remote copies retain source indexes")
	previousAttempts := clients[masked.ID].uploads
	// Old versions persisted the unadorned Go transport error. Even this
	// pre-upgrade row should use HTTP/1.1 without intervention or a new key.
	require.NoError(t, store.exec(ctx, `UPDATE gateway_log_archive_deliveries SET next_attempt_at=NULL,last_error=$1 WHERE bundle_id=$2 AND bucket_id=$3`, peerReset, row.ID, masked.ID))
	other := &pipeline{ctx: ctx, store: store, objects: factory, node: "node-b", report: func(err error) { t.Log(err) }}
	other.setConfig(DefaultConfig())
	peer := &pipeline{ctx: ctx, store: store, objects: factory, node: "node-c", report: other.report}
	peer.setConfig(DefaultConfig())
	var workers sync.WaitGroup
	for _, worker := range []*pipeline{other, peer} {
		workers.Add(1)
		go func() { defer workers.Done(); worker.relayPending() }()
	}
	workers.Wait()
	require.Equal(t, previousAttempts+1, clients[masked.ID].uploads, "only one node may perform this retry")
	jobs, err = store.deliveries(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, "uploaded", jobs[1].Status)
	copy := string(clients[masked.ID].objects[jobs[1].Key])
	require.NotContains(t, copy, "alice@example.com")
	require.Contains(t, copy, piiMask)
	require.Contains(t, copy, "9007199254740993")
	require.Equal(t, 2, strings.Count(copy, "\n"))
	require.Equal(t, rawUploads, clients[raw.ID].uploads)
	usage, err := store.BucketUsage(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(len(copy)), usage[masked.ID].SizeBytes)
	require.Equal(t, int64(1), usage[raw.ID].BundleCount)
}

func TestRedactedPrimaryNeverExposesRawStagingOrMakesAnOriginalCopy(t *testing.T) {
	ctx := context.Background()
	store := sqliteStore(t)
	primary, err := store.CreateBucket(ctx, BucketConfig{Bucket: "private", StorageRole: "rustfs", PrivacyMode: "redact", Enabled: true})
	require.NoError(t, err)
	target, err := store.CreateBucket(ctx, BucketConfig{Bucket: "external", PrivacyMode: "original", Enabled: true})
	require.NoError(t, err)
	master, copy := newMemObjects(), newMemObjects()
	master.name = primary.Bucket
	copy.name = target.Bucket
	factory := func(bucket BucketConfig) (objectStore, error) {
		if bucket.ID == primary.ID {
			return master, nil
		}
		return copy, nil
	}
	pipe, err := startPipeline(store, factory, "node", t.TempDir(), func(time.Time) string { return "2026-11-19" }, DefaultConfig(), func(err error) { t.Log(err) }, &stats{})
	require.NoError(t, err)
	defer pipe.stop(false)
	pipe.setBuckets([]BucketConfig{primary, target})
	x := exchange("one", "m", "m", "one")
	x.Request = []byte(`{"email":"alice@example.com","n":9007199254740993}`)
	require.True(t, pipe.submit(x))
	done := make(chan struct{})
	pipe.flushReq <- done
	<-done
	require.NoError(t, pipe.waitIdle(ctx))
	master.mu.Lock()
	var staged string
	for key, data := range master.objects {
		if strings.Contains(key, "_gateway-staging/") {
			staged = string(data)
		}
	}
	master.mu.Unlock()
	require.Contains(t, staged, piiMask)
	require.NotContains(t, staged, "alice@example.com")
	pipe.sealAll()
	var bundleID int64
	require.Eventually(t, func() bool {
		rows, _, err := store.ListBundles(ctx, BundleFilter{Status: StatusUploaded}, 1, 1)
		if err != nil || len(rows) == 0 {
			return false
		}
		bundleID = rows[0].ID
		jobs, err := store.deliveries(ctx, bundleID)
		return err == nil && len(jobs) == 1 && jobs[0].Status == "failed" && strings.Contains(jobs[0].LastError, "无法还原原文")
	}, 5*time.Second, 10*time.Millisecond)
	require.Empty(t, copy.objects)
	_, err = store.DeleteBucket(ctx, primary.ID)
	require.ErrorIs(t, err, errBucketInUse)
}

func TestPerBucketPolicyReevaluatedOnPriorityFailover(t *testing.T) {
	for _, policy := range []string{"original", "redact"} {
		t.Run(policy, func(t *testing.T) {
			store := newMemStore()
			first, last := newMemObjects(), newMemObjects()
			first.fail = errors.New("unavailable")
			otherPolicy := "redact"
			if policy == "redact" {
				otherPolicy = "original"
			}
			pipe := failoverPipeline(t, store, map[int64]*memObjects{1: first, 2: last}, []BucketConfig{{ID: 1, Enabled: true, PrivacyMode: otherPolicy}, {ID: 2, Enabled: true, PrivacyMode: policy}})
			x := exchange("abc", "m", "m", "req")
			x.Request = []byte(`{"email":"alice@example.com"}`)
			flushAll(t, pipe, x)
			waitUploaded(t, store, 1)
			data := string(last.objects[store.bundleList()[0].ObjectKey])
			require.Equal(t, policy == "original", strings.Contains(data, "alice@example.com"))
			require.Equal(t, policy == "redact", strings.Contains(data, piiMask))
		})
	}
}

func TestRustFSProbeDrainsLocalFallbackWithoutAnotherSession(t *testing.T) {
	ctx := context.Background()
	store := sqliteStore(t)
	primary, err := store.CreateBucket(ctx, BucketConfig{Bucket: "rustfs", StorageRole: "rustfs", Enabled: true})
	require.NoError(t, err)
	objects := newMemObjects()
	objects.fail = errors.New("offline")
	pipe, err := startPipeline(store, func(BucketConfig) (objectStore, error) { return objects, nil }, "node", t.TempDir(), func(time.Time) string { return "2026-11-19" }, DefaultConfig(), func(err error) { t.Log(err) }, &stats{}, primary)
	require.NoError(t, err)
	defer pipe.stop(false)
	x := exchange("probe", "m", "m", "id")
	x.Request = []byte(`{"text":"alice@example.com"}`)
	require.True(t, pipe.submit(x))
	done := make(chan struct{})
	pipe.flushReq <- done
	<-done
	require.NoError(t, pipe.waitIdle(ctx))
	require.NotZero(t, pipe.stageBackoff.Load())
	require.Len(t, pipe.openBundles(), 1)
	pipe.stageBackoff.Store(1)
	pipe.resumeStaging()
	require.Len(t, pipe.openBundles(), 1)
	objects.mu.Lock()
	objects.fail = nil
	objects.mu.Unlock()
	pipe.stageBackoff.Store(1)
	pipe.wakeUploader()
	require.Eventually(t, func() bool {
		rows, _, err := store.ListBundles(ctx, BundleFilter{Status: StatusUploaded}, 1, 10)
		if err != nil || len(rows) != 1 {
			return false
		}
		objects.mu.Lock()
		defer objects.mu.Unlock()
		data := string(objects.objects[rows[0].ObjectKey])
		return rows[0].LocalDeleted && strings.Contains(data, piiMask) && !strings.Contains(data, "alice@example.com")
	}, 5*time.Second, 10*time.Millisecond)
	require.Zero(t, pipe.stageBackoff.Load())
}

type busyPrimary struct {
	*memObjects
	started, release chan struct{}
	wholeStageLimit  atomic.Bool
}

func (s *busyPrimary) Upload(ctx context.Context, key, path string, progress func(int64)) error {
	close(s.started)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return s.memObjects.Upload(ctx, key, path, progress)
	}
}

func (s *busyPrimary) UploadStream(ctx context.Context, key string, size int64, produce func(io.Writer) error, progress func(int64)) error {
	if _, limited := ctx.Deadline(); limited && strings.Contains(key, "_gateway-staging/") {
		s.wholeStageLimit.Store(true)
	}
	return s.memObjects.UploadStream(ctx, key, size, produce, progress)
}

func TestRustFSFallbackRecoversDuringOldBundleUpload(t *testing.T) {
	ctx := t.Context()
	store := sqliteStore(t)
	dir := t.TempDir()
	primary, err := store.CreateBucket(ctx, BucketConfig{Bucket: "primary", StorageRole: "rustfs", PrivacyMode: "original", Enabled: true})
	require.NoError(t, err)
	old, err := store.CreateBundle(ctx, "node", "2026-11-18", "m", func(file string) string { return filepath.Join(dir, file) })
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(old.LocalPath, []byte("{}\n"), 0o600))
	require.NoError(t, store.CommitEntry(ctx, 3, Entry{BundleID: old.ID, Seq: 1, Name: EntryName(1), USize: 2, CSize: 2, StartedAt: time.Now(), EndedAt: time.Now()}))
	require.NoError(t, store.MarkSealed(ctx, old.ID, 3))
	// Also exercise an upgrade from a node that stopped in 'uploading'.
	claimed, err := store.ClaimUpload(ctx, "node", time.Now())
	require.NoError(t, err)
	require.Equal(t, old.ID, claimed.ID)
	s := &busyPrimary{memObjects: newMemObjects(), started: make(chan struct{}), release: make(chan struct{})}
	s.fail = errors.New("storage offline")
	p, err := startPipeline(store, func(BucketConfig) (objectStore, error) { return s, nil }, "node", dir, func(time.Time) string { return "2026-11-19" }, DefaultConfig(), func(err error) { t.Log(err) }, &stats{}, primary)
	require.NoError(t, err)
	defer p.stop(false)
	select {
	case <-s.started:
	case <-time.After(5 * time.Second):
		t.Fatal("old local upload never started")
	}
	require.True(t, p.submit(exchange("slow", "m", "m", "new-session")))
	done := make(chan struct{})
	p.flushReq <- done
	<-done
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, p.waitIdle(deadline))
	require.False(t, s.wholeStageLimit.Load())
	require.NotZero(t, p.stageBackoff.Load())
	open := p.openBundles()
	require.Len(t, open, 1)
	p.stageBackoff.Store(1)
	p.wakeUploader()
	require.Eventually(t, func() bool {
		probe := p.stageProbe.Load()
		return probe != nil && strings.Contains(probe.Error, "storage offline")
	}, 3*time.Second, 10*time.Millisecond)
	s.mu.Lock()
	s.fail = nil
	s.mu.Unlock()
	p.stageBackoff.Store(1)
	p.wakeUploader()
	require.Eventually(t, func() bool {
		row, err := store.Bundle(ctx, open[0].ID)
		return err == nil && row.Status == StatusSealed && p.stageBackoff.Load() == 0
	}, 3*time.Second, 10*time.Millisecond, "probe and seal must not wait for an unrelated upload")
	require.Empty(t, p.stageProbe.Load().Error)
	require.FileExists(t, old.LocalPath)
	close(s.release)
	require.Eventually(t, func() bool {
		rows, _, err := store.ListBundles(ctx, BundleFilter{Status: StatusUploaded}, 1, 5)
		return err == nil && len(rows) == 2
	}, 5*time.Second, 10*time.Millisecond)
}
