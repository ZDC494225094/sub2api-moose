package gatewaylogarchive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxSessionSpool = 512 << 20 // rotate one session document at this size
	maxSessions     = 20000
	maxPendingBytes = 512 << 20 // captured bytes waiting for the spool goroutine
	intakeCapacity  = 4096
	flushWorkers    = 2
	flushRetryDelay = 30 * time.Second
)

type sessionState struct {
	header sessionHeader
	path   string
	bytes  int64
	count  int
	lastAt time.Time
}

type flushJob struct {
	path    string
	retryAt time.Time
}

type bundle struct {
	mu      sync.Mutex
	row     Bundle
	zip     *zipAppender
	sealed  bool
	size    atomic.Int64
	entries atomic.Int64
}

// pipeline owns the on-disk state of one node: session spools, open bundles
// and the upload queue. It is created when the module is enabled.
type pipeline struct {
	store   archiveStore
	objects func(Config) (objectStore, error)
	node    string
	dir     string
	day     func(time.Time) string
	report  func(error)
	stats   *stats

	cfg       atomic.Pointer[Config]
	maxZip    atomic.Int64
	idle      atomic.Int64
	keepLocal atomic.Int64

	intakeMu sync.RWMutex
	open     bool
	in       chan *Exchange
	pending  atomic.Int64

	sessions  map[string]*sessionState // spool goroutine only
	nextPart  map[string]int
	active    atomic.Int64
	flushReq  chan chan struct{}
	stopSpool chan bool
	spoolDone chan struct{}

	jobs     chan flushJob
	retryMu  sync.Mutex
	retry    []flushJob
	inFlight atomic.Int64 // queued or running flush jobs
	quitting atomic.Bool

	bundlesMu sync.Mutex
	bundles   map[string]*bundle

	uploadWake chan struct{}
	uploading  atomic.Pointer[uploadProgress]

	ctx       context.Context
	cancel    context.CancelFunc
	workers   sync.WaitGroup
	lastPrune time.Time
}

type uploadProgress struct {
	BundleID int64  `json:"bundle_id"`
	FileName string `json:"file_name"`
	Total    int64  `json:"total"`
	sent     atomic.Int64
}

func (p *pipeline) spoolDir() string  { return filepath.Join(p.dir, "spool") }
func (p *pipeline) bundleDir() string { return filepath.Join(p.dir, "bundles") }

func startPipeline(store archiveStore, objects func(Config) (objectStore, error), node, dir string, day func(time.Time) string, cfg Config, report func(error), st *stats) (*pipeline, error) {
	p := &pipeline{
		store: store, objects: objects, node: node, dir: dir, day: day, report: report, stats: st,
		open: true, in: make(chan *Exchange, intakeCapacity),
		sessions: map[string]*sessionState{}, nextPart: map[string]int{},
		flushReq: make(chan chan struct{}), stopSpool: make(chan bool, 1), spoolDone: make(chan struct{}),
		jobs: make(chan flushJob, 65536), bundles: map[string]*bundle{}, uploadWake: make(chan struct{}, 1),
	}
	p.setConfig(cfg)
	for _, d := range []string{p.spoolDir(), p.bundleDir()} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	p.ctx, p.cancel = context.WithCancel(context.Background())
	if err := p.recover(); err != nil {
		p.cancel()
		return nil, err
	}
	go p.spoolLoop()
	for i := 0; i < flushWorkers; i++ {
		p.workers.Add(1)
		go p.flushLoop()
	}
	p.workers.Add(1)
	go p.uploadLoop()
	return p, nil
}

func (p *pipeline) setConfig(cfg Config) {
	c := cfg
	p.cfg.Store(&c)
	p.maxZip.Store(cfg.MaxZipSizeMB << 20)
	p.idle.Store(int64(cfg.SessionIdleSeconds) * int64(time.Second))
	p.keepLocal.Store(int64(cfg.KeepLocalHours) * int64(time.Hour))
}

// submit never blocks the request goroutine; a full queue drops the exchange.
func (p *pipeline) submit(x *Exchange) bool {
	p.intakeMu.RLock()
	defer p.intakeMu.RUnlock()
	if !p.open {
		return false
	}
	size := x.size()
	if p.pending.Load()+size > maxPendingBytes {
		return false
	}
	select {
	case p.in <- x:
		p.pending.Add(size)
		return true
	default:
		return false
	}
}

func (p *pipeline) spoolLoop() {
	defer close(p.spoolDone)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case x := <-p.in:
			p.pending.Add(-x.size())
			p.appendExchange(x)
		case <-ticker.C:
			p.flushIdle(time.Now(), false)
			p.retryDue(time.Now())
		case done := <-p.flushReq:
			p.drainIntake()
			p.flushIdle(time.Now(), true)
			close(done)
		case drain := <-p.stopSpool:
			p.drainIntake()
			if drain {
				p.flushIdle(time.Now(), true)
			}
			return
		}
	}
}

func (p *pipeline) drainIntake() {
	for {
		select {
		case x := <-p.in:
			p.pending.Add(-x.size())
			p.appendExchange(x)
		default:
			return
		}
	}
}

func (p *pipeline) appendExchange(x *Exchange) {
	s := p.sessions[x.SessionKey]
	if s == nil {
		part := p.nextPart[x.SessionKey]
		header := sessionHeader{
			SessionID: x.SessionID, Key: x.SessionKey, Model: x.Model, ModelKey: x.ModelKey, Platform: x.Platform,
			UserID: x.UserID, APIKeyID: x.APIKeyID, Part: part, StartedAt: x.StartedAt, Ephemeral: x.Ephemeral,
		}
		raw, _ := json.Marshal(header)
		path := filepath.Join(p.spoolDir(), x.SessionKey+"-"+fmt.Sprint(time.Now().UnixNano())+".spool")
		n, err := appendRecord(path, x.StartedAt, raw)
		if err != nil {
			p.stats.dropped.Add(1)
			p.report(fmt.Errorf("写入会话缓存失败：%w", err))
			return
		}
		s = &sessionState{header: header, path: path, bytes: n}
		p.sessions[x.SessionKey] = s
		p.active.Store(int64(len(p.sessions)))
	}
	n, err := appendRecord(s.path, x.FinishedAt, encodeExchange(s.count+1, x))
	if err != nil {
		p.stats.dropped.Add(1)
		p.report(fmt.Errorf("写入会话缓存失败：%w", err))
		return
	}
	s.count++
	s.bytes += n
	s.lastAt = time.Now()
	if x.Ephemeral {
		p.enqueue(s, false)
	} else if s.bytes >= maxSessionSpool {
		p.enqueue(s, true)
	}
	if len(p.sessions) > maxSessions {
		p.flushOldest()
	}
}

func (p *pipeline) enqueue(s *sessionState, rotated bool) {
	delete(p.sessions, s.header.Key)
	if rotated {
		next := s.header.Part + 1
		if next < 2 {
			next = 2
		}
		p.nextPart[s.header.Key] = next
	} else {
		delete(p.nextPart, s.header.Key)
	}
	p.active.Store(int64(len(p.sessions)))
	p.push(flushJob{path: s.path})
}

func (p *pipeline) push(job flushJob) {
	p.inFlight.Add(1)
	select {
	case p.jobs <- job:
	default:
		p.inFlight.Add(-1)
		p.retryMu.Lock()
		job.retryAt = time.Now()
		p.retry = append(p.retry, job)
		p.retryMu.Unlock()
	}
}

func (p *pipeline) flushOldest() {
	var oldest *sessionState
	for _, s := range p.sessions {
		if oldest == nil || s.lastAt.Before(oldest.lastAt) {
			oldest = s
		}
	}
	if oldest != nil {
		p.enqueue(oldest, false)
	}
}

func (p *pipeline) flushIdle(now time.Time, all bool) {
	idle := time.Duration(p.idle.Load())
	for _, s := range p.sessions {
		if all || now.Sub(s.lastAt) >= idle {
			p.enqueue(s, false)
		}
	}
}

func (p *pipeline) retryDue(now time.Time) {
	p.retryMu.Lock()
	var keep []flushJob
	var due []flushJob
	for _, j := range p.retry {
		if now.Before(j.retryAt) {
			keep = append(keep, j)
		} else {
			due = append(due, j)
		}
	}
	p.retry = keep
	p.retryMu.Unlock()
	for _, j := range due {
		p.push(j)
	}
}

func (p *pipeline) flushLoop() {
	defer p.workers.Done()
	for job := range p.jobs {
		if p.quitting.Load() {
			p.inFlight.Add(-1)
			continue // left on disk; recovered on the next start
		}
		if err := p.flush(job); err != nil {
			p.stats.flushErrors.Add(1)
			p.report(err)
			p.retryMu.Lock()
			job.retryAt = time.Now().Add(flushRetryDelay)
			p.retry = append(p.retry, job)
			p.retryMu.Unlock()
		} else {
			p.stats.sessions.Add(1)
		}
		p.inFlight.Add(-1)
	}
}

// flush writes one session spool into today's bundle of its model.
func (p *pipeline) flush(job flushJob) error {
	header, records, err := scanSpool(job.path)
	if err != nil || len(records) == 0 {
		_ = os.Remove(job.path)
		return nil
	}
	day := p.day(time.Now())
	b, err := p.acquire(day, header.ModelKey)
	if err != nil {
		return fmt.Errorf("创建压缩包失败：%w", err)
	}
	if b.zip.Count() > 0 && b.zip.Size() >= p.maxZip.Load() {
		p.sealLocked(b)
		b.mu.Unlock()
		if b, err = p.acquire(day, header.ModelKey); err != nil {
			return fmt.Errorf("创建压缩包失败：%w", err)
		}
	}
	defer b.mu.Unlock()
	doc, err := openDocument(job.path, header, records)
	if err != nil {
		return err
	}
	seq := b.zip.Count() + 1
	ended := records[len(records)-1].at
	entry, err := b.zip.Append(EntryName(seq), ended, doc)
	_ = doc.Close()
	if err != nil {
		return fmt.Errorf("写入压缩包失败：%w", err)
	}
	if err := b.zip.Sync(); err != nil {
		b.zip.RemoveLast()
		return fmt.Errorf("写入压缩包失败：%w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = p.store.CommitEntry(ctx, b.zip.Size(), Entry{
		BundleID: b.row.ID, Seq: seq, Name: entry.Name, SessionID: header.SessionID, Model: header.Model,
		Platform: header.Platform, UserID: header.UserID, APIKeyID: header.APIKeyID, ExchangeCount: len(records),
		Offset: entry.Offset, CSize: entry.CSize, USize: entry.USize, CRC: entry.CRC,
		StartedAt: header.StartedAt, EndedAt: ended,
	})
	if err != nil {
		b.zip.RemoveLast()
		return fmt.Errorf("记录压缩包条目失败：%w", err)
	}
	b.size.Store(b.zip.Size())
	b.entries.Store(int64(b.zip.Count()))
	_ = os.Remove(job.path)
	if b.zip.Size() >= p.maxZip.Load() {
		p.sealLocked(b)
	}
	return nil
}

// acquire returns the open bundle for (day, model), locked.
func (p *pipeline) acquire(day, modelKey string) (*bundle, error) {
	key := day + "/" + modelKey
	for {
		p.bundlesMu.Lock()
		b := p.bundles[key]
		if b == nil {
			created, err := p.createBundle(day, modelKey)
			if err != nil {
				p.bundlesMu.Unlock()
				return nil, err
			}
			b = created
			p.bundles[key] = b
		}
		p.bundlesMu.Unlock()
		b.mu.Lock()
		if !b.sealed {
			return b, nil
		}
		b.mu.Unlock()
	}
}

func (p *pipeline) createBundle(day, modelKey string) (*bundle, error) {
	dir := filepath.Join(p.bundleDir(), day)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	row, err := p.store.CreateBundle(ctx, p.node, day, modelKey, func(file string) string { return filepath.Join(dir, file) })
	if err != nil {
		return nil, err
	}
	z, err := createZip(row.LocalPath)
	if err != nil {
		_ = p.store.MarkBroken(ctx, row.ID, "创建本地文件失败："+err.Error())
		return nil, err
	}
	return &bundle{row: row, zip: z}, nil
}

// sealLocked finishes the archive and queues it for upload. b.mu is held.
func (p *pipeline) sealLocked(b *bundle) {
	if b.sealed {
		return
	}
	b.sealed = true
	p.bundlesMu.Lock()
	if p.bundles[b.row.Day+"/"+b.row.ModelKey] == b {
		delete(p.bundles, b.row.Day+"/"+b.row.ModelKey)
	}
	p.bundlesMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if b.zip.Count() == 0 {
		_ = b.zip.Close()
		_ = os.Remove(b.row.LocalPath)
		_ = p.store.MarkBroken(ctx, b.row.ID, "空压缩包，已丢弃")
		_ = p.store.MarkLocalDeleted(ctx, b.row.ID)
		return
	}
	size, err := b.zip.Seal()
	if err != nil {
		_ = b.zip.Close()
		p.report(fmt.Errorf("封包 %s 失败：%w", b.row.FileName, err))
		_ = p.store.MarkBroken(ctx, b.row.ID, "封包失败："+err.Error())
		return
	}
	if err := p.store.MarkSealed(ctx, b.row.ID, size); err != nil {
		// The row stays "open"; the next start truncates the directory and
		// seals again, so nothing is lost.
		p.report(fmt.Errorf("记录封包状态失败：%w", err))
		return
	}
	p.wakeUploader()
}

// sealBefore seals bundles of days before today (day rollover).
func (p *pipeline) sealBefore(today string) {
	p.sealWhere(func(b *bundle) bool { return b.row.Day < today })
}

func (p *pipeline) sealAll() { p.sealWhere(func(*bundle) bool { return true }) }

func (p *pipeline) sealWhere(match func(*bundle) bool) {
	p.bundlesMu.Lock()
	var list []*bundle
	for _, b := range p.bundles {
		if match(b) {
			list = append(list, b)
		}
	}
	p.bundlesMu.Unlock()
	for _, b := range list {
		b.mu.Lock()
		p.sealLocked(b)
		b.mu.Unlock()
	}
}

func (p *pipeline) closeOpen() {
	p.bundlesMu.Lock()
	defer p.bundlesMu.Unlock()
	for key, b := range p.bundles {
		b.mu.Lock()
		_ = b.zip.Close()
		b.sealed = true
		b.mu.Unlock()
		delete(p.bundles, key)
	}
}

// flushNow ends every active session and seals all open bundles.
func (p *pipeline) flushNow(ctx context.Context) error {
	done := make(chan struct{})
	select {
	case p.flushReq <- done:
	case <-ctx.Done():
		return ctx.Err()
	}
	<-done
	if err := p.waitIdle(ctx); err != nil {
		return err
	}
	p.sealAll()
	return nil
}

func (p *pipeline) waitIdle(ctx context.Context) error {
	for p.inFlight.Load() > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return nil
}

// stop shuts the pipeline down. drain=true (module switched off) ends every
// session and seals bundles; drain=false (process exit) leaves spools and
// open bundles on disk for recovery.
func (p *pipeline) stop(drain bool) {
	p.intakeMu.Lock()
	p.open = false
	p.intakeMu.Unlock()
	p.stopSpool <- drain
	<-p.spoolDone
	if drain {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_ = p.waitIdle(ctx)
		cancel()
	} else {
		p.quitting.Store(true)
	}
	close(p.jobs)
	p.cancel()
	p.workers.Wait()
	if drain {
		p.sealAll()
	}
	p.closeOpen()
}

func (p *pipeline) wakeUploader() {
	select {
	case p.uploadWake <- struct{}{}:
	default:
	}
}

// recover rebuilds state after a restart: open bundles continue at their
// committed length and leftover spools become active sessions again.
func (p *pipeline) recover() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := p.store.ResetUploading(ctx, p.node); err != nil {
		return err
	}
	rows, err := p.store.OpenBundles(ctx, p.node)
	if err != nil {
		return err
	}
	today := p.day(time.Now())
	for _, row := range rows {
		entries, err := p.store.BundleEntries(ctx, row.ID)
		if err != nil {
			return err
		}
		z, err := reopenZip(row.LocalPath, row.SizeBytes, entries)
		if err != nil {
			_ = p.store.MarkBroken(ctx, row.ID, "本地压缩包丢失或损坏："+err.Error())
			continue
		}
		b := &bundle{row: row, zip: z}
		b.size.Store(z.Size())
		b.entries.Store(int64(z.Count()))
		key := row.Day + "/" + row.ModelKey
		if old := p.bundles[key]; old != nil {
			old.mu.Lock()
			p.sealLocked(old)
			old.mu.Unlock()
		}
		p.bundles[key] = b
	}
	p.sealBefore(today)
	files, _ := filepath.Glob(filepath.Join(p.spoolDir(), "*.spool"))
	sort.Strings(files)
	for _, path := range files {
		header, records, err := scanSpool(path)
		if err != nil || len(records) == 0 {
			_ = os.Remove(path)
			continue
		}
		info, _ := os.Stat(path)
		s := &sessionState{header: header, path: path, count: len(records), lastAt: records[len(records)-1].at}
		if info != nil {
			s.bytes = info.Size()
		}
		if header.Ephemeral || header.Key == "" {
			p.push(flushJob{path: path})
			continue
		}
		// Names sort by creation time, so an older spool of the same session
		// (a rotated part) is flushed and the newest stays active.
		if old := p.sessions[header.Key]; old != nil {
			p.push(flushJob{path: old.path})
		}
		p.sessions[header.Key] = s
	}
	p.active.Store(int64(len(p.sessions)))
	return nil
}

type openBundleView struct {
	ID       int64  `json:"id"`
	Day      string `json:"day"`
	ModelKey string `json:"model_key"`
	FileName string `json:"file_name"`
	Size     int64  `json:"size_bytes"`
	Entries  int64  `json:"entry_count"`
}

func (p *pipeline) openBundles() []openBundleView {
	p.bundlesMu.Lock()
	defer p.bundlesMu.Unlock()
	out := make([]openBundleView, 0, len(p.bundles))
	for _, b := range p.bundles {
		out = append(out, openBundleView{ID: b.row.ID, Day: b.row.Day, ModelKey: b.row.ModelKey, FileName: b.row.FileName, Size: b.size.Load(), Entries: b.entries.Load()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].FileName < out[j].FileName })
	return out
}

// RemoveLast undoes the most recent Append (its DB commit failed).
func (z *zipAppender) RemoveLast() {
	if len(z.entries) == 0 {
		return
	}
	last := z.entries[len(z.entries)-1]
	z.entries = z.entries[:len(z.entries)-1]
	_ = z.f.Truncate(last.Offset)
	z.size = last.Offset
}

var errNoStorage = errors.New("尚未配置存储桶")
