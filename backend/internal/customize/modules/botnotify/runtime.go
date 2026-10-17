package botnotify

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/customize"
)

type StateReader interface {
	Enabled(context.Context, string) (bool, error)
}

type Status struct {
	Bot         Bot        `json:"bot"`
	Polling     bool       `json:"polling"`
	Pushing     bool       `json:"pushing"`
	LastError   string     `json:"last_error"`
	LastErrorAt *time.Time `json:"last_error_at"`
}

// botWorker tracks one running bot: its poll/push goroutines share a context
// and a per-bot advisory lease so multiple server instances never consume the
// same Telegram offset or duplicate a scheduled broadcast.
type botWorker struct {
	cancel    context.CancelFunc
	done      chan struct{}
	signature string
}

// Runtime supervises one worker per configured bot.
type Runtime struct {
	Store      *Store
	Platform   Platform
	Data       Data
	States     StateReader
	mu         sync.Mutex
	statuses   map[int64]*Status
	workers    map[int64]*botWorker
	wake       chan struct{}
	cancel     context.CancelFunc
	done       chan struct{}
	unregister func()
}

func NewRuntime(store *Store, platform Platform, data Data, states StateReader) *Runtime {
	return &Runtime{
		Store:    store,
		Platform: platform,
		Data:     data,
		States:   states,
		statuses: map[int64]*Status{},
		workers:  map[int64]*botWorker{},
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
}

func (r *Runtime) Status(botID int64) Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.statuses[botID]; ok {
		return *s
	}
	return Status{}
}

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

func (r *Runtime) Stop() {
	if r.cancel != nil {
		r.unregister()
		r.cancel()
		<-r.done
	}
}

func (r *Runtime) report(botID int64, ctx context.Context, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	// Transport errors are deliberately sanitized before reaching here.
	if botID > 0 {
		r.mu.Lock()
		s, ok := r.statuses[botID]
		if !ok {
			s = &Status{}
			r.statuses[botID] = s
		}
		now := time.Now()
		s.LastError = err.Error()
		s.LastErrorAt = &now
		r.mu.Unlock()
	}
	slog.Warn("bot-notify task failed", "bot", botID, "error", err)
}

func (r *Runtime) allowed(ctx context.Context) bool {
	enabled, err := r.States.Enabled(ctx, ID)
	if err != nil {
		r.report(0, ctx, err)
	}
	return err == nil && enabled
}

func signatureOf(e BotEntry) string {
	raw, _ := json.Marshal(e)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (r *Runtime) stopWorker(id int64) {
	r.mu.Lock()
	w, ok := r.workers[id]
	if ok {
		delete(r.workers, id)
	}
	r.mu.Unlock()
	if ok {
		w.cancel()
		<-w.done
	}
}

func (r *Runtime) stopAllWorkers() {
	r.mu.Lock()
	ids := make([]int64, 0, len(r.workers))
	for id := range r.workers {
		ids = append(ids, id)
	}
	r.mu.Unlock()
	for _, id := range ids {
		r.stopWorker(id)
	}
}

// manage reconciles the running worker set with the configured bots every
// couple of seconds (and immediately on signal): workers start, stop and
// restart as bots are created, edited, deleted or the module is toggled.
func (r *Runtime) manage(ctx context.Context) {
	defer close(r.done)
	defer r.stopAllWorkers()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	lastPrune := time.Now()
	for {
		checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		if r.allowed(checkCtx) {
			entries, err := r.Store.Bots(checkCtx, true)
			if err != nil {
				r.report(0, checkCtx, err)
			} else {
				r.reconcile(ctx, entries)
			}
			if time.Since(lastPrune) > time.Hour {
				r.report(0, checkCtx, r.Store.Prune(checkCtx))
				lastPrune = time.Now()
			}
		} else {
			r.stopAllWorkers()
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-r.wake:
		case <-ticker.C:
		}
	}
}

func (r *Runtime) reconcile(parent context.Context, entries []BotEntry) {
	desired := make(map[int64]BotEntry, len(entries))
	for _, e := range entries {
		if e.BotToken == "" || (!e.ReceiveEnabled && !e.PushEnabled) {
			continue
		}
		desired[e.ID] = e
	}
	// Stop workers whose bot vanished or whose config changed.
	r.mu.Lock()
	running := make(map[int64]string, len(r.workers))
	for id, w := range r.workers {
		running[id] = w.signature
	}
	r.mu.Unlock()
	for id, sig := range running {
		e, ok := desired[id]
		if !ok || signatureOf(e) != sig {
			r.stopWorker(id)
		}
	}
	// Start workers that are not running yet (a worker that exited on its own
	// counts as not running and is restarted with the same config).
	for id, e := range desired {
		r.mu.Lock()
		w, exists := r.workers[id]
		if exists {
			select {
			case <-w.done:
				delete(r.workers, id)
				exists = false
			default:
			}
		}
		r.mu.Unlock()
		if exists {
			continue
		}
		workerCtx, cancelWorker := context.WithCancel(parent)
		w = &botWorker{cancel: cancelWorker, done: make(chan struct{}), signature: signatureOf(e)}
		r.mu.Lock()
		r.workers[id] = w
		if _, ok := r.statuses[id]; !ok {
			r.statuses[id] = &Status{}
		}
		r.statuses[id].Bot = Bot{ID: e.BotID, Username: e.BotUsername}
		r.mu.Unlock()
		entry := e
		go func() {
			defer close(w.done)
			r.runBot(workerCtx, entry)
		}()
	}
	// Forget statuses of deleted bots.
	r.mu.Lock()
	for id := range r.statuses {
		if _, ok := desired[id]; !ok {
			if _, running := r.workers[id]; !running {
				delete(r.statuses, id)
			}
		}
	}
	r.mu.Unlock()
}

// runBot holds the bot's advisory lease for its lifetime, then runs its
// receive/push loops. If another instance holds the lease it retries quietly.
func (r *Runtime) runBot(ctx context.Context, e BotEntry) {
	for ctx.Err() == nil {
		lease, err := r.acquireLease(ctx, e.ID)
		if err != nil {
			if ctx.Err() == nil {
				r.report(e.ID, ctx, err)
			}
			if !wait(ctx, 5*time.Second) {
				return
			}
			continue
		}
		r.setRunning(e.ID, e.ReceiveEnabled, e.PushEnabled)
		var wg sync.WaitGroup
		if e.ReceiveEnabled {
			wg.Add(1)
			go func() { defer wg.Done(); r.poll(ctx, e) }()
		}
		if e.PushEnabled {
			wg.Add(1)
			go func() { defer wg.Done(); r.push(ctx, e) }()
		}
		wg.Wait()
		r.setRunning(e.ID, false, false)
		lease.Close()
		return
	}
}

func (r *Runtime) acquireLease(ctx context.Context, botRowID int64) (*sql.Conn, error) {
	for ctx.Err() == nil {
		conn, err := r.Store.DB.Conn(ctx)
		if err != nil {
			return nil, err
		}
		var locked bool
		err = conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock(20261017,"+strconv.FormatInt(770000+botRowID, 10)+")").Scan(&locked)
		if err == nil && locked {
			return conn, nil
		}
		if err != nil {
			conn.Close()
			return nil, err
		}
		conn.Close()
		if !wait(ctx, 5*time.Second) {
			return nil, ctx.Err()
		}
	}
	return nil, ctx.Err()
}

func (r *Runtime) setRunning(botID int64, polling, pushing bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.statuses[botID]
	if !ok {
		s = &Status{}
		r.statuses[botID] = s
	}
	s.Polling = polling
	s.Pushing = pushing
}

func wait(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// nextPushDelay returns the wait until the next wall-clock boundary that is
// a whole multiple of interval after local midnight, so a 60-minute interval
// fires exactly on the hour (13:00, 14:00, ...) instead of drifting with the
// worker's start time.
func nextPushDelay(now time.Time, interval time.Duration) time.Duration {
	if interval < time.Minute {
		interval = time.Hour
	}
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	elapsed := now.Sub(midnight)
	next := (elapsed/interval + 1) * interval
	return midnight.Add(next).Sub(now)
}

func (r *Runtime) push(ctx context.Context, e BotEntry) {
	interval := time.Duration(e.IntervalMinutes) * time.Minute
	if interval < time.Minute {
		interval = time.Hour
	}
	for {
		if !wait(ctx, nextPushDelay(time.Now(), interval)) {
			return
		}
		if !r.allowed(ctx) {
			return
		}
		r.broadcast(ctx, e)
	}
}

func (r *Runtime) broadcast(ctx context.Context, e BotEntry) {
	workCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if len(e.ChatIDs) == 0 {
		r.report(e.ID, workCtx, errors.New("已启用定时推送，但还没有配置推送目标 chat_id"))
		return
	}
	text, err := Render(workCtx, r.Data, e.PushTemplate, time.Now())
	r.report(e.ID, workCtx, err)
	if err != nil || text == "" {
		return
	}
	for _, chat := range e.ChatIDs {
		if !r.allowed(workCtx) {
			return
		}
		r.report(e.ID, workCtx, r.Platform.SendMessage(workCtx, e.BotToken, chat, text))
	}
}

// PushNow renders the bot's push template and sends it to every push target
// immediately; the admin UI uses it to verify a bot's push setup on demand.
func (r *Runtime) PushNow(ctx context.Context, e BotEntry) error {
	if e.BotToken == "" {
		return errors.New("请先配置 bot token")
	}
	if len(e.ChatIDs) == 0 {
		return errors.New("还没有配置推送目标 chat_id")
	}
	text, err := Render(ctx, r.Data, e.PushTemplate, time.Now())
	if err != nil {
		return err
	}
	if text == "" {
		return errors.New("推送模板渲染结果为空")
	}
	for _, chat := range e.ChatIDs {
		if err := r.Platform.SendMessage(ctx, e.BotToken, chat, text); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runtime) poll(ctx context.Context, e BotEntry) {
	var offset int64
	for ctx.Err() == nil {
		if !r.allowed(ctx) {
			return
		}
		err := r.Store.DB.QueryRowContext(ctx, "SELECT update_offset FROM bot_notify_bots WHERE id=$1", e.ID).Scan(&offset)
		if err != nil {
			r.report(e.ID, ctx, err)
			if !wait(ctx, 5*time.Second) {
				return
			}
			continue
		}
		messages, err := r.Platform.Poll(ctx, e.BotToken, offset)
		if err != nil {
			r.report(e.ID, ctx, err)
			if !wait(ctx, 5*time.Second) {
				return
			}
			continue
		}
		for _, m := range messages {
			if !r.allowed(ctx) {
				return
			}
			if err = r.process(ctx, e, m); err != nil {
				r.report(e.ID, ctx, err)
				break
			}
		}
		if err != nil && !wait(ctx, 5*time.Second) {
			return
		}
	}
}

func (r *Runtime) process(ctx context.Context, e BotEntry, m Message) error {
	var id int64
	err := r.Store.DB.QueryRowContext(ctx, `INSERT INTO bot_messages(platform,bot_id,bot_row_id,chat_id,chat_type,sender_id,sender_name,text,message_id,update_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(platform,bot_id,update_id) DO NOTHING RETURNING id`, m.Platform, e.BotID, e.ID, m.ChatID, m.ChatType, m.SenderID, m.SenderName, m.Text, m.MessageID, m.UpdateID).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// Existing events are not sent twice after restart. Telegram has no
	// idempotency keys; an interrupted send is intentionally not replayed.
	if err == nil && Addressed(m, Bot{ID: e.BotID, Username: e.BotUsername}) {
		rules, ruleErr := r.Store.Rules(ctx, e.ID)
		if ruleErr != nil {
			return ruleErr
		}
		for _, rule := range rules {
			if !Matches(rule, m.Text, e.BotUsername) {
				continue
			}
			text, renderErr := Render(ctx, r.Data, rule.ReplyTemplate, time.Now())
			if _, saveErr := r.Store.DB.ExecContext(ctx, "UPDATE bot_messages SET rule_id=$1,reply_content=$2 WHERE id=$3", rule.ID, text, id); saveErr != nil {
				return saveErr
			}
			if renderErr == nil && text != "" && r.allowed(ctx) {
				renderErr = r.Platform.SendMessage(ctx, e.BotToken, m.ChatID, text)
				if renderErr == nil {
					_, renderErr = r.Store.DB.ExecContext(ctx, "UPDATE bot_messages SET replied=true WHERE id=$1", id)
				}
			}
			r.report(e.ID, ctx, renderErr)
			break
		}
	}
	_, err = r.Store.DB.ExecContext(ctx, "UPDATE bot_notify_bots SET update_offset=GREATEST(update_offset,$1) WHERE id=$2 AND (config->>'bot_id')::bigint=$3", m.UpdateID+1, e.ID, e.BotID)
	return err
}
