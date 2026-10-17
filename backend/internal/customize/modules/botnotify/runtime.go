package botnotify

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
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

type Runtime struct {
	Store      *Store
	Platform   Platform
	Data       Data
	States     StateReader
	mu         sync.Mutex
	status     Status
	wake       chan struct{}
	cancel     context.CancelFunc
	done       chan struct{}
	unregister func()
}

func NewRuntime(store *Store, platform Platform, data Data, states StateReader) *Runtime {
	return &Runtime{Store: store, Platform: platform, Data: data, States: states, wake: make(chan struct{}, 1), done: make(chan struct{})}
}

func (r *Runtime) Status() Status { r.mu.Lock(); defer r.mu.Unlock(); return r.status }
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

func (r *Runtime) report(ctx context.Context, err error) {
	if err == nil || ctx.Err() != nil {
		return
	}
	// Transport errors are deliberately sanitized before reaching here.
	r.mu.Lock()
	now := time.Now()
	r.status.LastError = err.Error()
	r.status.LastErrorAt = &now
	r.mu.Unlock()
	slog.Warn("bot-notify task failed", "error", err)
}

func (r *Runtime) allowed(ctx context.Context) bool {
	enabled, err := r.States.Enabled(ctx, ID)
	if err != nil {
		r.report(ctx, err)
	}
	return err == nil && enabled
}

// A pinned advisory lock prevents multiple server instances from consuming the
// same Telegram offset or duplicating scheduled broadcasts. The connection is
// reserved for the worker lifetime and released before storage shuts down.
func (r *Runtime) manage(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	var workerCancel context.CancelFunc
	var workerDone chan struct{}
	var signature string
	var lease *sql.Conn
	stop := func() {
		if workerCancel != nil {
			workerCancel()
			<-workerDone
			workerCancel = nil
		}
		if lease != nil {
			unlockCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			lease.ExecContext(unlockCtx, "SELECT pg_advisory_unlock(20261017, 7101)")
			cancel()
			lease.Close()
			lease = nil
		}
		signature = ""
		r.mu.Lock()
		r.status.Polling = false
		r.status.Pushing = false
		r.mu.Unlock()
	}
	defer stop()
	for {
		checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		enabled := r.allowed(checkCtx)
		var cfg Config
		var err error
		if enabled {
			cfg, err = r.Store.Config(checkCtx, true)
			r.report(checkCtx, err)
		}
		raw, _ := json.Marshal(cfg)
		if !enabled || err != nil || cfg.BotToken == "" || (!cfg.ReceiveEnabled && !cfg.PushEnabled) {
			stop()
		} else if signature != string(raw) {
			stop()
			lease, err = r.Store.DB.Conn(checkCtx)
			if err == nil {
				var locked bool
				err = lease.QueryRowContext(checkCtx, "SELECT pg_try_advisory_lock(20261017, 7101)").Scan(&locked)
				if err != nil || !locked {
					lease.Close()
					lease = nil
				}
			}
			if err != nil {
				r.report(checkCtx, err)
			}
			if lease != nil {
				signature = string(raw)
				workerCtx, c := context.WithCancel(ctx)
				workerCancel = c
				workerDone = make(chan struct{})
				r.mu.Lock()
				r.status.Bot = Bot{ID: cfg.BotID, Username: cfg.BotUsername}
				r.status.Polling = cfg.ReceiveEnabled
				r.status.Pushing = cfg.PushEnabled
				r.mu.Unlock()
				go func() { defer close(workerDone); r.run(workerCtx, cfg) }()
			}
		} else if lease != nil {
			if pingErr := lease.PingContext(checkCtx); pingErr != nil {
				r.report(checkCtx, pingErr)
				stop()
			}
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

func (r *Runtime) run(ctx context.Context, c Config) {
	var wg sync.WaitGroup
	if c.ReceiveEnabled {
		wg.Add(1)
		go func() { defer wg.Done(); r.poll(ctx, c) }()
	}
	if c.PushEnabled {
		wg.Add(1)
		go func() { defer wg.Done(); r.push(ctx, c) }()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			cleanupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			r.report(cleanupCtx, r.Store.Prune(cleanupCtx))
			cancel()
			if !wait(ctx, time.Hour) {
				return
			}
		}
	}()
	wg.Wait()
}

func (r *Runtime) push(ctx context.Context, c Config) {
	interval := time.Duration(c.IntervalMinutes) * time.Minute
	if interval < time.Minute {
		interval = time.Hour
	}
	for wait(ctx, interval) {
		if !r.allowed(ctx) {
			return
		}
		workCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		text, err := Render(workCtx, r.Data, c.PushTemplate, time.Now())
		r.report(workCtx, err)
		if err == nil && text != "" {
			for _, chat := range c.ChatIDs {
				if !r.allowed(workCtx) {
					break
				}
				r.report(workCtx, r.Platform.SendMessage(workCtx, c.BotToken, chat, text))
			}
		}
		cancel()
	}
}

func (r *Runtime) poll(ctx context.Context, c Config) {
	var offset int64
	for ctx.Err() == nil {
		if !r.allowed(ctx) {
			return
		}
		err := r.Store.DB.QueryRowContext(ctx, "SELECT update_offset FROM bot_notify_config WHERE id=1").Scan(&offset)
		if err != nil {
			r.report(ctx, err)
			if !wait(ctx, 5*time.Second) {
				return
			}
			continue
		}
		messages, err := r.Platform.Poll(ctx, c.BotToken, offset)
		if err != nil {
			r.report(ctx, err)
			if !wait(ctx, 5*time.Second) {
				return
			}
			continue
		}
		for _, m := range messages {
			if !r.allowed(ctx) {
				return
			}
			if err = r.process(ctx, c, m); err != nil {
				r.report(ctx, err)
				break
			}
		}
		if len(messages) > 0 {
			r.report(ctx, r.Store.Prune(ctx))
		}
		if err != nil && !wait(ctx, 5*time.Second) {
			return
		}
	}
}

func (r *Runtime) process(ctx context.Context, c Config, m Message) error {
	var id int64
	err := r.Store.DB.QueryRowContext(ctx, `INSERT INTO bot_messages(platform,bot_id,chat_id,chat_type,sender_id,sender_name,text,message_id,update_id)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(platform,bot_id,update_id) DO NOTHING RETURNING id`, m.Platform, c.BotID, m.ChatID, m.ChatType, m.SenderID, m.SenderName, m.Text, m.MessageID, m.UpdateID).Scan(&id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// Existing events are not sent twice after restart. Telegram has no
	// idempotency keys; an interrupted send is intentionally not replayed.
	if err == nil && Addressed(m, Bot{ID: c.BotID, Username: c.BotUsername}) {
		rules, ruleErr := r.Store.Rules(ctx)
		if ruleErr != nil {
			return ruleErr
		}
		for _, rule := range rules {
			if !Matches(rule, m.Text, c.BotUsername) {
				continue
			}
			text, renderErr := Render(ctx, r.Data, rule.ReplyTemplate, time.Now())
			if _, saveErr := r.Store.DB.ExecContext(ctx, "UPDATE bot_messages SET rule_id=$1,reply_content=$2 WHERE id=$3", rule.ID, text, id); saveErr != nil {
				return saveErr
			}
			if renderErr == nil && text != "" && r.allowed(ctx) {
				renderErr = r.Platform.SendMessage(ctx, c.BotToken, m.ChatID, text)
				if renderErr == nil {
					_, renderErr = r.Store.DB.ExecContext(ctx, "UPDATE bot_messages SET replied=true WHERE id=$1", id)
				}
			}
			r.report(ctx, renderErr)
			break
		}
	}
	_, err = r.Store.DB.ExecContext(ctx, "UPDATE bot_notify_config SET update_offset=GREATEST(update_offset,$1) WHERE id=1 AND (config->>'bot_id')::bigint=$2", m.UpdateID+1, c.BotID)
	return err
}
