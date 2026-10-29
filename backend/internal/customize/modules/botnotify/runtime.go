package botnotify

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
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
	Announcing  bool       `json:"announcing"`
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
	Platforms  map[string]Platform
	Data       Data
	States     StateReader
	mu         sync.Mutex
	statuses   map[int64]*Status
	workers    map[int64]*botWorker
	wake       chan struct{}
	cancel     context.CancelFunc
	done       chan struct{}
	unregister func()
	AI         AIProvider
	Knowledge  KnowledgeSource
}

func NewRuntime(store *Store, platforms map[string]Platform, data Data, states StateReader) *Runtime {
	return &Runtime{
		Store:     store,
		Platforms: platforms,
		Data:      data,
		States:    states,
		statuses:  map[int64]*Status{},
		workers:   map[int64]*botWorker{},
		wake:      make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
}

// SetAIProvider installs the optional, server-owned completion port. Keeping it
// nil is a safe no-op and preserves the original keyword-only behavior.
func (r *Runtime) SetAIProvider(provider AIProvider) { r.AI = provider }

// SetAIKnowledge installs the public site context (contacts, docs manual).
// Without it the AI only knows the site name and the knowledge base.
func (r *Runtime) SetAIKnowledge(source KnowledgeSource) { r.Knowledge = source }

// PlatformFor resolves a bot's platform adapter ("telegram", "qq").
func (r *Runtime) PlatformFor(name string) (Platform, bool) {
	if name == "" {
		name = "telegram"
	}
	p, ok := r.Platforms[name]
	return p, ok
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
		if e.BotToken == "" || (!e.ReceiveEnabled && !e.PushEnabled && !e.AnnounceSync) {
			continue
		}
		if _, ok := r.PlatformFor(e.Platform); !ok {
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
		platform, _ := r.PlatformFor(e.Platform)
		go func() {
			defer close(w.done)
			r.runBot(workerCtx, entry, platform)
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
// receive/push/announcement loops. If another instance holds the lease it
// retries quietly.
func (r *Runtime) runBot(ctx context.Context, e BotEntry, platform Platform) {
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
		announcing := e.AnnounceSync && len(e.ChatIDs) > 0
		r.setRunning(e.ID, e.ReceiveEnabled, e.PushEnabled, announcing)
		// AI answers run beside the poll loop; they are canceled and joined
		// before the lease is released, like every other bot task.
		aiCtx, cancelAI := context.WithCancel(ctx)
		var dispatcher *aiDispatcher
		if e.ReceiveEnabled && r.AI != nil {
			dispatcher = r.newBotDispatcher(aiCtx, e, platform)
		}
		var wg sync.WaitGroup
		if e.ReceiveEnabled {
			wg.Add(1)
			go func() { defer wg.Done(); r.poll(ctx, e, platform, dispatcher) }()
		}
		if e.PushEnabled {
			wg.Add(1)
			go func() { defer wg.Done(); r.push(ctx, e, platform) }()
		}
		if announcing {
			wg.Add(1)
			go func() { defer wg.Done(); r.announceWatch(ctx, e, platform) }()
		}
		wg.Wait()
		cancelAI()
		if dispatcher != nil {
			dispatcher.wait()
		}
		r.setRunning(e.ID, false, false, false)
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

func (r *Runtime) setRunning(botID int64, polling, pushing, announcing bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.statuses[botID]
	if !ok {
		s = &Status{}
		r.statuses[botID] = s
	}
	s.Polling = polling
	s.Pushing = pushing
	s.Announcing = announcing
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

func (r *Runtime) push(ctx context.Context, e BotEntry, platform Platform) {
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
		r.broadcast(ctx, e, platform)
	}
}

func (r *Runtime) broadcast(ctx context.Context, e BotEntry, platform Platform) {
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
	// Collect per-target failures and report them together: reporting each
	// send in turn would let a later success erase an earlier failure.
	var failures []string
	for _, chat := range e.ChatIDs {
		if !r.allowed(workCtx) {
			return
		}
		if err := platform.SendMessage(workCtx, e.BotToken, chat, text); err != nil {
			failures = append(failures, fmt.Sprintf("%s（%v）", chat, err))
		}
	}
	if len(failures) > 0 {
		r.report(e.ID, workCtx, fmt.Errorf("定时推送失败：%s", strings.Join(failures, "；")))
	} else {
		r.report(e.ID, workCtx, nil)
	}
}

// PushNow renders the bot's push template and sends it to every push target
// immediately; the admin UI uses it to verify a bot's push setup on demand.
func (r *Runtime) PushNow(ctx context.Context, e BotEntry) error {
	platform, ok := r.PlatformFor(e.Platform)
	if !ok {
		return errors.New("不支持的机器人平台")
	}
	if e.BotToken == "" {
		return errors.New("请先配置机器人凭证")
	}
	if len(e.ChatIDs) == 0 {
		return errors.New("还没有配置推送目标")
	}
	text, err := Render(ctx, r.Data, e.PushTemplate, time.Now())
	if err != nil {
		return err
	}
	if text == "" {
		return errors.New("推送模板渲染结果为空")
	}
	for _, chat := range e.ChatIDs {
		if err := platform.SendMessage(ctx, e.BotToken, chat, text); err != nil {
			return err
		}
	}
	return nil
}

// announceWatch polls the active announcements every 30 seconds and pushes
// the ones this bot has not sent yet (new publications and content edits) to
// its push targets. The first pass only records the baseline watermark, so
// enabling sync never replays the announcement history.
func (r *Runtime) announceWatch(ctx context.Context, e BotEntry, platform Platform) {
	state, err := r.Store.AnnounceState(ctx, e.ID)
	if err != nil {
		r.report(e.ID, ctx, err)
	}
	check := func() {
		if !r.allowed(ctx) {
			return
		}
		workCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()
		list, err := r.Data.Announcements(workCtx)
		if err != nil {
			r.report(e.ID, workCtx, err)
			return
		}
		changed := false
		mark := func(a Announcement, chat, hash string) {
			key := strconv.FormatInt(a.ID, 10)
			if state.Delivered[key] == nil {
				state.Delivered[key] = map[string]string{}
			}
			state.Delivered[key][chat] = hash
		}
		// Baseline any target that is new to this bot (including every target
		// on the first pass): it must not receive announcements published
		// before it was added, so enabling sync never replays history.
		known := make(map[string]bool, len(state.Chats))
		for _, chat := range state.Chats {
			known[chat] = true
		}
		for _, chat := range e.ChatIDs {
			if known[chat] {
				continue
			}
			for _, a := range list {
				mark(a, chat, announcementHash(a))
			}
			state.Chats = append(state.Chats, chat)
			changed = true
		}
		// Deliver oldest first so group members read announcements in order.
		// Only successful targets are marked; failures retry on the next pass.
		for i := len(list) - 1; i >= 0; i-- {
			a := list[i]
			key := strconv.FormatInt(a.ID, 10)
			hash := announcementHash(a)
			text := FormatAnnouncement(a)
			for _, chat := range e.ChatIDs {
				if state.Delivered[key][chat] == hash {
					continue
				}
				if err := platform.SendMessage(workCtx, e.BotToken, chat, text); err != nil {
					r.report(e.ID, workCtx, fmt.Errorf("公告推送到 %s 失败：%w", chat, err))
					continue
				}
				mark(a, chat, hash)
				changed = true
			}
		}
		if !state.Seeded {
			state.Seeded = true
			changed = true
		}
		if changed {
			r.report(e.ID, workCtx, r.Store.SaveAnnounceState(workCtx, e.ID, state))
		}
	}
	check()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

func announcementHash(a Announcement) string {
	sum := sha256.Sum256([]byte(a.Title + "\x00" + a.Content))
	return hex.EncodeToString(sum[:8])
}

func (r *Runtime) poll(ctx context.Context, e BotEntry, platform Platform, dispatcher *aiDispatcher) {
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
		messages, err := platform.Poll(ctx, e.BotToken, offset)
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
			if err = r.process(ctx, e, platform, m, dispatcher); err != nil {
				r.report(e.ID, ctx, err)
				break
			}
		}
		if err != nil && !wait(ctx, 5*time.Second) {
			return
		}
	}
}

func (r *Runtime) process(ctx context.Context, e BotEntry, platform Platform, m Message, dispatcher *aiDispatcher) error {
	var id int64
	err := r.Store.DB.QueryRowContext(ctx, `INSERT INTO bot_messages(platform,bot_id,bot_row_id,chat_id,chat_type,sender_id,sender_name,text,message_id,update_id,image_count)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(platform,bot_id,update_id) DO NOTHING RETURNING id`, m.Platform, e.BotID, e.ID, m.ChatID, m.ChatType, m.SenderID, m.SenderName, m.Text, m.MessageID, m.UpdateID, len(m.Images)).Scan(&id)
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
		matched := false
		for _, rule := range rules {
			if !Matches(rule, m.Text, e.BotUsername) {
				continue
			}
			matched = true
			text, renderErr := Render(ctx, r.Data, rule.ReplyTemplate, time.Now())
			if _, saveErr := r.Store.DB.ExecContext(ctx, "UPDATE bot_messages SET rule_id=$1,reply_content=$2 WHERE id=$3", rule.ID, text, id); saveErr != nil {
				return saveErr
			}
			if renderErr == nil && text != "" && r.allowed(ctx) {
				renderErr = r.sendReply(ctx, e, platform, m, text)
				if renderErr == nil {
					_, renderErr = r.Store.DB.ExecContext(ctx, "UPDATE bot_messages SET replied=true WHERE id=$1", id)
				}
			}
			r.report(e.ID, ctx, renderErr)
			break
		}
		// Keyword rules are authoritative. AI is only a fallback for messages
		// that were not matched, so enabling it cannot change existing replies.
		if !matched && r.AI != nil {
			r.routeAI(ctx, e, platform, m, id, dispatcher)
		}
	}
	_, err = r.Store.DB.ExecContext(ctx, "UPDATE bot_notify_bots SET update_offset=GREATEST(update_offset,$1) WHERE id=$2 AND (config->>'bot_id')::bigint=$3", m.UpdateID+1, e.ID, e.BotID)
	return err
}

// sendReply threads the reply to the inbound message when the transport
// supports it (QQ needs that to reply at all; Telegram quotes it in groups).
func (r *Runtime) sendReply(ctx context.Context, e BotEntry, platform Platform, to Message, text string) error {
	if replier, ok := platform.(Replier); ok {
		return replier.Reply(ctx, e.BotToken, to, text)
	}
	return platform.SendMessage(ctx, e.BotToken, to.ChatID, text)
}
