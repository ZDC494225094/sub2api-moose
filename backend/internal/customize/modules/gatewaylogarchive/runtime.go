package gatewaylogarchive

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/google/uuid"
)

type StateReader interface {
	Enabled(context.Context, string) (bool, error)
}

type stats struct {
	captured      atomic.Int64
	skippedUser   atomic.Int64
	skippedModel  atomic.Int64
	skippedStatus atomic.Int64
	dropped       atomic.Int64
	sessions      atomic.Int64
	flushErrors   atomic.Int64
	uploaded      atomic.Int64
	uploadErrors  atomic.Int64
}

// Runtime supervises capture and the archive pipeline. Capture is a single
// atomic pointer load on the gateway hot path when the module is off.
type Runtime struct {
	Store   *Store
	States  StateReader
	Hooks   Hooks
	DataDir string

	repo    archiveStore
	objects func(BucketConfig) (objectStore, error)
	buckets func(context.Context) ([]BucketConfig, error)
	dayOf   func(time.Time) string

	policy     atomic.Pointer[capturePolicy]
	stats      stats
	exportBusy atomic.Bool

	mu        sync.Mutex
	pipe      *pipeline
	enabled   bool
	reason    string
	diskFree  int64
	lastErr   string
	lastErrAt *time.Time
	nodeID    string
	lastPrune time.Time
	storage   int // enabled buckets at the last reconcile

	wake       chan struct{}
	cancel     context.CancelFunc
	done       chan struct{}
	unregister func()
}

func NewRuntime(store *Store, states StateReader, hooks Hooks, dataDir string) *Runtime {
	r := &Runtime{
		Store: store, States: states, Hooks: hooks, DataDir: dataDir,
		repo: store, objects: newS3Objects,
		dayOf: func(t time.Time) string { return t.In(timezone.Location()).Format("2006-01-02") },
		wake:  make(chan struct{}, 1), done: make(chan struct{}), reason: "未启动",
	}
	r.buckets = func(ctx context.Context) ([]BucketConfig, error) { return r.Store.Buckets(ctx) }
	return r
}

func (r *Runtime) dir() string { return filepath.Join(r.DataDir, "gateway-log-archive") }

func (r *Runtime) signal() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Runtime) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.unregister = customize.RegisterLifecycle(func(id string) {
		if id == ID {
			r.signal()
		}
	})
	go r.manage(ctx)
}

// Stop runs after the HTTP server has drained. Sessions and open bundles stay
// on disk and continue on the next start.
func (r *Runtime) Stop() {
	if r.cancel == nil {
		return
	}
	r.unregister()
	r.cancel()
	<-r.done
	r.policy.Store(nil)
	r.mu.Lock()
	pipe := r.pipe
	r.pipe = nil
	r.mu.Unlock()
	if pipe != nil {
		pipe.stop(false)
	}
}

func (r *Runtime) report(err error) {
	if err == nil {
		return
	}
	now := time.Now()
	r.mu.Lock()
	r.lastErr, r.lastErrAt = storageErrorText(err), &now
	r.mu.Unlock()
	slog.Warn("gateway-log-archive", "error", err)
}

func (r *Runtime) manage(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		r.reconcile(checkCtx)
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
		case <-ticker.C:
		}
	}
}

func (r *Runtime) reconcile(ctx context.Context) {
	enabled, err := r.States.Enabled(ctx, ID)
	if err != nil {
		// A transient state read failure keeps the current mode.
		r.report(err)
		return
	}
	if !enabled {
		r.policy.Store(nil)
		r.mu.Lock()
		pipe := r.pipe
		r.pipe, r.enabled, r.reason = nil, false, "扩展未启用"
		r.mu.Unlock()
		if pipe != nil {
			pipe.stop(true)
		}
		return
	}
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		// An unreadable capture policy cannot safely authorize new log copies.
		r.policy.Store(nil)
		r.report(err)
		return
	}
	all, err := r.buckets(ctx)
	if err != nil {
		// Keep the current destinations for uploads, but still honor the new
		// exclusions if e.g. an unrelated bucket credential cannot be decrypted.
		if current := r.policy.Load(); current != nil {
			next := *current
			next.cfg.ExcludedUserIDs = cfg.ExcludedUserIDs
			r.policy.Store(&next)
		}
		r.report(err)
		return
	}
	buckets := enabledBuckets(all)
	pipe, err := r.ensurePipeline(cfg, all)
	if err != nil {
		r.policy.Store(nil)
		r.report(fmt.Errorf("启动归档流程失败：%w", err))
		r.mu.Lock()
		r.enabled, r.reason = true, "启动失败："+err.Error()
		r.mu.Unlock()
		return
	}
	pipe.setConfig(cfg)
	key, keyErr := r.detectorKey(cfg.Detector)
	pipe.setDetector(cfg.Detector, key, keyErr)
	pipe.setBuckets(all)
	free := diskFree(r.dir())
	reason := ""
	switch {
	case !cfg.CaptureEnabled:
		reason = "采集未开启"
	case len(buckets) == 0:
		reason = "尚未配置可用的存储桶"
	case cfg.MinFreeDiskGB > 0 && free >= 0 && free < int64(cfg.MinFreeDiskGB)<<30:
		reason = fmt.Sprintf("数据目录剩余空间低于 %d GB，已暂停采集", cfg.MinFreeDiskGB)
	}
	if reason == "" {
		r.policy.Store(&capturePolicy{cfg: cfg, maxBody: cfg.MaxBodyMB << 20, submit: pipe.submit})
	} else {
		r.policy.Store(nil)
	}
	pipe.sealBefore(r.dayOf(time.Now()))
	r.mu.Lock()
	r.enabled, r.reason, r.diskFree, r.storage = true, reason, free, len(buckets)
	prune := time.Since(r.lastPrune) > time.Hour
	if prune {
		r.lastPrune = time.Now()
	}
	r.mu.Unlock()
	if prune {
		r.report(r.repo.Prune(ctx, time.Now().AddDate(0, 0, -cfg.EntryRetentionDays)))
	}
	pipe.wakeUploader()
}

// detectorKey decrypts the stored detection service key.
func (r *Runtime) detectorKey(d DetectorConfig) (string, error) {
	if d.APIKeyEnc == "" {
		return "", nil
	}
	if r.Store == nil {
		return "", errStoreUnavailable
	}
	return r.Store.decrypt(d.APIKeyEnc)
}

func (r *Runtime) ensurePipeline(cfg Config, buckets []BucketConfig) (*pipeline, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pipe != nil {
		return r.pipe, nil
	}
	if r.repo == nil {
		return nil, errStoreUnavailable
	}
	if s, ok := r.repo.(*Store); ok && s.ready() != nil {
		return nil, errStoreUnavailable
	}
	node, err := loadNodeID(r.dir())
	if err != nil {
		return nil, err
	}
	pipe, err := startPipeline(r.repo, r.objects, node, r.dir(), r.dayOf, cfg, r.report, &r.stats, buckets...)
	if err != nil {
		return nil, err
	}
	r.pipe, r.nodeID = pipe, node
	return pipe, nil
}

// loadNodeID keeps a stable identity in the data directory so a recreated
// container on the same volume resumes its own bundles.
func loadNodeID(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, "node-id")
	if raw, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(raw)); id != "" {
			return limit(id, 64), nil
		}
	}
	id := uuid.NewString()
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return "", err
	}
	return id, nil
}

func (r *Runtime) current() *pipeline {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pipe
}

type RuntimeStatus struct {
	ModuleEnabled    bool                `json:"module_enabled"`
	Running          bool                `json:"running"`
	Capturing        bool                `json:"capturing"`
	Reason           string              `json:"reason"`
	NodeID           string              `json:"node_id"`
	DataDir          string              `json:"data_dir"`
	Today            string              `json:"today"`
	DiskFreeBytes    int64               `json:"disk_free_bytes"`
	StorageBuckets   int                 `json:"storage_buckets"`
	RustFSFallback   bool                `json:"rustfs_fallback"`
	RustFSProbe      *stagingProbeResult `json:"rustfs_probe,omitempty"`
	UploadedBundles  int64               `json:"uploaded_bundles"`
	UploadedBytes    int64               `json:"uploaded_bytes"`
	ActiveSessions   int64               `json:"active_sessions"`
	QueuedExchanges  int                 `json:"queued_exchanges"`
	PendingFlushes   int                 `json:"pending_flushes"`
	OpenBundles      []openBundleView    `json:"open_bundles"`
	Uploading        *uploadView         `json:"uploading"`
	Captured         int64               `json:"captured"`
	SkippedUser      int64               `json:"skipped_user"`
	SkippedModel     int64               `json:"skipped_model"`
	SkippedStatus    int64               `json:"skipped_status"`
	Dropped          int64               `json:"dropped"`
	SessionsArchived int64               `json:"sessions_archived"`
	FlushErrors      int64               `json:"flush_errors"`
	Uploaded         int64               `json:"uploaded"`
	UploadErrors     int64               `json:"upload_errors"`
	LastError        string              `json:"last_error"`
	LastErrorAt      *time.Time          `json:"last_error_at"`
	Bundles          []StatusCount       `json:"bundles"`
	Detector         *DetectorStatus     `json:"detector,omitempty"`
}

type uploadView struct {
	BundleID  int64  `json:"bundle_id"`
	FileName  string `json:"file_name"`
	Total     int64  `json:"total"`
	Sent      int64  `json:"sent"`
	BodyBytes int64  `json:"body_bytes"`
	Retries   int64  `json:"part_retries"`
	Phase     string `json:"phase,omitempty"`
	// When this attempt started, which attempt it is, and the target bucket.
	StartedAt time.Time `json:"started_at"`
	Attempt   int       `json:"attempt"`
	Bucket    string    `json:"bucket,omitempty"`
}

func (r *Runtime) Status(ctx context.Context) RuntimeStatus {
	r.mu.Lock()
	st := RuntimeStatus{
		ModuleEnabled: r.enabled, Running: r.pipe != nil, Reason: r.reason, NodeID: r.nodeID, DataDir: r.dir(),
		DiskFreeBytes: r.diskFree, StorageBuckets: r.storage, LastError: r.lastErr, LastErrorAt: r.lastErrAt,
	}
	pipe := r.pipe
	r.mu.Unlock()
	st.Capturing = r.policy.Load() != nil
	st.Today = r.dayOf(time.Now())
	if pipe != nil {
		st.RustFSFallback = pipe.stageBackoff.Load() != 0
		st.RustFSProbe = pipe.stageProbe.Load()
		st.ActiveSessions = pipe.active.Load()
		st.QueuedExchanges = len(pipe.in)
		pipe.retryMu.Lock()
		st.PendingFlushes = len(pipe.retry) + int(pipe.inFlight.Load())
		pipe.retryMu.Unlock()
		st.OpenBundles = pipe.openBundles()
		if d := pipe.remoteDetector.Load(); d != nil {
			st.Detector = d.status()
		}
		if up := pipe.uploading.Load(); up != nil {
			st.Uploading = &uploadView{BundleID: up.BundleID, FileName: up.FileName, Total: up.Total, Sent: up.sent.Load(), BodyBytes: up.bodyBytes.Load(), Retries: up.retries.Load(),
				StartedAt: up.startedAt, Attempt: up.attempt, Bucket: up.bucket}
			if phase := up.phase.Load(); phase != nil {
				st.Uploading.Phase = *phase
			}
		}
	}
	st.Captured = r.stats.captured.Load()
	st.SkippedUser = r.stats.skippedUser.Load()
	st.SkippedModel = r.stats.skippedModel.Load()
	st.SkippedStatus = r.stats.skippedStatus.Load()
	st.Dropped = r.stats.dropped.Load()
	st.SessionsArchived = r.stats.sessions.Load()
	st.FlushErrors = r.stats.flushErrors.Load()
	st.Uploaded = r.stats.uploaded.Load()
	st.UploadErrors = r.stats.uploadErrors.Load()
	if r.Store != nil && r.Store.DB != nil {
		st.Bundles, _ = r.Store.Summary(ctx)
		// Cumulative totals survive index pruning, unlike the bundle summary.
		if usage, err := r.Store.BucketUsage(ctx); err == nil {
			for _, u := range usage {
				st.UploadedBundles += u.BundleCount
				st.UploadedBytes += u.SizeBytes
			}
		}
	}
	return st
}

// FlushNow ends all active sessions, seals open bundles and starts uploads.
func (r *Runtime) FlushNow(ctx context.Context) error {
	pipe := r.current()
	if pipe == nil {
		return errors.New("归档流程未运行，请先启用扩展")
	}
	if err := pipe.flushNow(ctx); err != nil {
		return err
	}
	pipe.wakeUploader()
	return nil
}

// objectStoreFor opens the bucket that holds an uploaded bundle.
func (r *Runtime) objectStoreFor(ctx context.Context, b Bundle) (objectStore, error) {
	all, err := r.buckets(ctx)
	if err != nil {
		return nil, err
	}
	bucket, ok := resolveBucket(all, b)
	if !ok {
		return nil, errors.New("该归档文件所在的存储桶配置已删除，无法从存储桶读取")
	}
	return r.objects(bucket)
}

type readCloser struct {
	io.Reader
	closers []io.Closer
}

func (rc readCloser) Close() error {
	for _, c := range rc.closers {
		_ = c.Close()
	}
	return nil
}

// OpenEntry uses remote indexes after upload (the uploaded JSONL may be
// redacted, while a retained local source is raw). Pending entries use local
// offsets. Historical ZIP objects remain readable.
func (r *Runtime) OpenEntry(ctx context.Context, e Entry, b Bundle) (io.ReadCloser, error) {
	r.mu.Lock()
	node := r.nodeID
	r.mu.Unlock()
	jsonl := strings.HasSuffix(b.FileName, ".jsonl")
	if b.Status != StatusUploaded && b.StageBucketID != nil {
		if !e.StagedRemote {
			if node != "" && b.NodeID != node {
				return nil, errors.New("回退文件位于归档所属节点，恢复上传后可跨节点预览")
			}
			file, err := os.Open(fallbackPath(b, e.StageKey))
			if err != nil {
				return nil, err
			}
			return readCloser{Reader: io.LimitReader(file, e.USize), closers: []io.Closer{file}}, nil
		}
		objects, err := r.objectStoreFor(ctx, Bundle{BucketID: b.StageBucketID})
		if err != nil {
			return nil, err
		}
		return objects.OpenRange(ctx, e.StageKey, 0, e.USize)
	}
	if b.Status != StatusUploaded && !b.LocalDeleted && b.LocalPath != "" && (node == "" || b.NodeID == node) {
		if f, err := os.Open(b.LocalPath); err == nil {
			if jsonl {
				return readCloser{Reader: io.NewSectionReader(f, e.Offset, e.USize), closers: []io.Closer{f}}, nil
			}
			rd, err := readEntry(io.NewSectionReader(f, e.Offset, 30+int64(len(e.Name))+1024+e.CSize), e.CSize)
			if err != nil {
				_ = f.Close()
				return nil, err
			}
			return readCloser{Reader: rd, closers: []io.Closer{rd, f}}, nil
		}
	}
	if b.Status != StatusUploaded || b.ObjectKey == "" {
		return nil, errors.New("该数据所在归档文件不在本节点且尚未上传")
	}
	objects, err := r.objectStoreFor(ctx, b)
	if err != nil {
		return nil, err
	}
	length := 30 + int64(len(e.Name)) + 1024 + e.CSize
	if jsonl {
		length = e.USize
	}
	body, err := objects.OpenRange(ctx, b.ObjectKey, e.Offset, length)
	if err != nil {
		return nil, err
	}
	if jsonl {
		return body, nil
	}
	rd, err := readEntry(body, e.CSize)
	if err != nil {
		_ = body.Close()
		return nil, err
	}
	return readCloser{Reader: rd, closers: []io.Closer{rd, body}}, nil
}
