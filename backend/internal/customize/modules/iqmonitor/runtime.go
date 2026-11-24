package iqmonitor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"os"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
)

type StateReader interface {
	Enabled(context.Context, string) (bool, error)
}

const (
	// MaxConcurrentRequests caps in-flight gateway requests per node,
	// shared by probe rounds and topic reviews.
	MaxConcurrentRequests = 4
	heartbeatEvery        = 20 * time.Second
	staleAfter            = 3 * time.Minute
	tickEvery             = 30 * time.Second
	reviewEvery           = 10 * time.Second
	pruneEvery            = time.Hour
)

// Runtime schedules rounds, runs claimed rounds and reviews submissions.
type Runtime struct {
	Store  *Store
	States StateReader
	Host   Host
	Caller Caller
	NodeID string
	// sleep waits between request retries (tests shorten it).
	sleep func(context.Context, time.Duration) bool

	sem    chan struct{}
	signer previewSigner

	mu        sync.Mutex
	running   bool // a round runs on this node
	lastPrune time.Time
	lastErr   string

	wake       chan struct{}
	cancel     context.CancelFunc
	ctx        context.Context
	wg         sync.WaitGroup
	done       chan struct{}
	unregister func()
}

func NewRuntime(store *Store, states StateReader, host Host, caller Caller) *Runtime {
	return &Runtime{
		Store: store, States: states, Host: host, Caller: caller,
		NodeID: nodeID(), sem: make(chan struct{}, MaxConcurrentRequests),
		wake: make(chan struct{}, 1), done: make(chan struct{}),
		sleep: sleepCtx,
	}
}

func nodeID() string {
	host, _ := os.Hostname()
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return truncateRunes(fmt.Sprintf("%s-%d-%s", host, os.Getpid(), hex.EncodeToString(b)), 120)
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func randIndex(n int) int {
	if n <= 1 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(v.Int64())
}

func (r *Runtime) signal() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Runtime) Start() {
	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.unregister = customize.RegisterLifecycle(func(id string) {
		if id == ID {
			r.signal()
		}
	})
	go r.manage()
	r.wg.Add(1)
	go r.reviewLoop()
}

// Stop cancels in-flight work; a round this node was running is marked
// interrupted so it never stays "running".
func (r *Runtime) Stop() {
	if r.cancel == nil {
		return
	}
	r.unregister()
	r.cancel()
	<-r.done
	r.wg.Wait()
}

func (r *Runtime) report(err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	r.lastErr = err.Error()
	r.mu.Unlock()
	slog.Warn("iq-monitor", "error", err)
}

func (r *Runtime) enabled(ctx context.Context) bool {
	ok, err := r.States.Enabled(ctx, ID)
	return err == nil && ok
}

func (r *Runtime) manage() {
	defer close(r.done)
	ticker := time.NewTicker(tickEvery)
	defer ticker.Stop()
	for {
		r.tick(r.ctx)
		select {
		case <-r.ctx.Done():
			return
		case <-r.wake:
		case <-ticker.C:
		}
	}
}

func (r *Runtime) tick(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	if !r.enabled(ctx) {
		return
	}
	now := r.Store.now()
	if _, err := r.Store.InterruptStale(ctx, now.Add(-staleAfter)); err != nil {
		r.report(err)
	}
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		r.report(err)
		return
	}
	r.mu.Lock()
	prune := now.Sub(r.lastPrune) >= pruneEvery
	if prune {
		r.lastPrune = now
	}
	r.mu.Unlock()
	if prune {
		r.report(r.Store.Prune(ctx, now.AddDate(0, 0, -cfg.RetentionDays)))
	}
	r.maybeSchedule(ctx, cfg, now)
}

// slotStart aligns rounds to the interval since the Unix epoch, so every
// node computes the same slot for the same moment.
func slotStart(now time.Time, minutes int) time.Time {
	step := int64(minutes) * 60
	return time.Unix(now.Unix()/step*step, 0).UTC()
}

func (r *Runtime) maybeSchedule(ctx context.Context, cfg Config, now time.Time) {
	if !cfg.Logic.Enabled && !cfg.Drawing.Enabled {
		return
	}
	slot := slotStart(now, cfg.IntervalMinutes)
	latest, err := r.Store.LatestRound(ctx)
	if err != nil {
		r.report(err)
		return
	}
	// One round per slot (a manual round inside the slot counts), and no
	// overlap with a round another node is still running.
	if latest != nil && (!latest.StartedAt.Before(slot) || (latest.Status == RoundRunning && latest.HeartbeatAt.After(now.Add(-staleAfter)))) {
		return
	}
	_, err = r.startRound(ctx, cfg, fmt.Sprintf("slot:%d:%d", cfg.IntervalMinutes, slot.Unix()), "schedule")
	if err != nil && !errors.Is(err, errSlotTaken) && !errors.Is(err, errNothingToRun) {
		r.report(err)
	}
}

var (
	errSubmissionsClosed = errors.New("投稿暂未开放")
	errSlotTaken         = errors.New("本时间槽已由其他节点执行")
	errNothingToRun      = errors.New("没有启用的被测目标或测试")
	ErrRoundRunning      = errors.New("已有一轮测试正在进行，请等待结束后再触发")
)

// TriggerNow starts a manual round on this node.
func (r *Runtime) TriggerNow(ctx context.Context) (int64, error) {
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		return 0, err
	}
	if !cfg.Logic.Enabled && !cfg.Drawing.Enabled {
		return 0, errors.New("逻辑测试和画图测试都未开启")
	}
	running, err := r.Store.RunningRound(ctx, r.Store.now().Add(-staleAfter))
	if err != nil {
		return 0, err
	}
	if running != nil {
		return 0, ErrRoundRunning
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	id, err := r.startRound(ctx, cfg, "manual:"+hex.EncodeToString(b), "manual")
	if errors.Is(err, errNothingToRun) {
		return 0, errors.New("没有启用的被测目标")
	}
	return id, err
}

func (r *Runtime) enabledTargets(ctx context.Context) ([]Target, error) {
	all, err := r.Store.Targets(ctx)
	if err != nil {
		return nil, err
	}
	var out []Target
	for _, t := range all {
		if t.Enabled {
			out = append(out, t)
		}
	}
	return out, nil
}

// startRound claims the slot and runs the round in the background.
func (r *Runtime) startRound(ctx context.Context, cfg Config, slotKey, trigger string) (int64, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return 0, ErrRoundRunning
	}
	r.running = true
	r.mu.Unlock()
	release := func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}
	targets, err := r.enabledTargets(ctx)
	if err != nil {
		release()
		return 0, err
	}
	if len(targets) == 0 {
		release()
		return 0, errNothingToRun
	}
	round := Round{SlotKey: slotKey, Trigger: trigger, NodeID: r.NodeID, LogicEnabled: cfg.Logic.Enabled, DrawingEnabled: cfg.Drawing.Enabled}
	id, ok, err := r.Store.ClaimRound(ctx, round)
	if err != nil {
		release()
		return 0, err
	}
	if !ok {
		release()
		return 0, errSlotTaken
	}
	round.ID = id
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer release()
		r.runRound(r.ctx, cfg, round, targets)
	}()
	return id, nil
}
