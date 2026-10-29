package botnotify

import (
	"context"
	"sync"
	"time"
)

const (
	// aiQuietPeriod waits for follow-up messages (a screenshot, then the
	// question) so they are answered together.
	aiQuietPeriod = 2500 * time.Millisecond
	// aiMaxGather bounds that wait after the first queued message.
	aiMaxGather = 8 * time.Second
	// aiConcurrency bounds simultaneous model calls per bot.
	aiConcurrency = 3
	// aiMaxConversations bounds queued conversations per bot under a flood.
	aiMaxConversations = 256
	// aiMaxBatch bounds messages merged into one answer; older ones drop.
	aiMaxBatch = 6
)

// aiItem is one addressed message waiting for an AI answer.
type aiItem struct {
	msg   Message
	rowID int64 // bot_messages.id, marked replied once the answer is sent
	at    time.Time
}

// aiDispatcher answers AI messages off the polling loop, so one slow answer
// never stalls the bot. Each conversation is answered one batch at a time:
// messages that arrive while a batch waits or runs are merged into the next
// answer. The cooldown delays the next batch instead of dropping messages,
// and a semaphore bounds concurrent model calls.
type aiDispatcher struct {
	ctx      context.Context
	answer   func(context.Context, []aiItem)
	cooldown func(context.Context) time.Duration
	quiet    time.Duration
	gather   time.Duration
	sem      chan struct{}

	mu       sync.Mutex
	pending  map[string][]aiItem
	answered map[string]time.Time
	wg       sync.WaitGroup
}

func newAIDispatcher(ctx context.Context, answer func(context.Context, []aiItem), cooldown func(context.Context) time.Duration) *aiDispatcher {
	return &aiDispatcher{
		ctx:      ctx,
		answer:   answer,
		cooldown: cooldown,
		quiet:    aiQuietPeriod,
		gather:   aiMaxGather,
		sem:      make(chan struct{}, aiConcurrency),
		pending:  map[string][]aiItem{},
		answered: map[string]time.Time{},
	}
}

// enqueue adds a message to its conversation, starting the conversation's
// worker when none is running. It reports false when the bot is stopping or
// too many conversations are already waiting.
func (d *aiDispatcher) enqueue(key string, item aiItem) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ctx.Err() != nil {
		return false
	}
	queue, running := d.pending[key]
	if !running {
		if len(d.pending) >= aiMaxConversations {
			return false
		}
		d.wg.Add(1)
		go d.serve(key)
	}
	if len(queue) >= aiMaxBatch {
		queue = append(queue[:0:0], queue[1:]...)
	}
	d.pending[key] = append(queue, item)
	return true
}

// wait joins every conversation worker; call it after the worker context
// is canceled or once nothing enqueues anymore.
func (d *aiDispatcher) wait() { d.wg.Wait() }

// serve owns one conversation until its queue is empty. A key present in
// d.pending always has exactly one serve goroutine.
func (d *aiDispatcher) serve(key string) {
	defer d.wg.Done()
	for {
		if !d.settle(key) {
			return
		}
		select {
		case d.sem <- struct{}{}:
		case <-d.ctx.Done():
			return
		}
		d.mu.Lock()
		batch := d.pending[key]
		d.pending[key] = nil
		d.mu.Unlock()
		if len(batch) > 0 {
			d.answer(d.ctx, batch)
		}
		<-d.sem
		d.mu.Lock()
		d.answered[key] = time.Now()
		if len(d.pending[key]) == 0 {
			delete(d.pending, key)
			d.forgetOld()
			d.mu.Unlock()
			return
		}
		d.mu.Unlock()
	}
}

// settle sleeps until the conversation is quiet (or has gathered long
// enough) and its cooldown since the previous answer has passed.
func (d *aiDispatcher) settle(key string) bool {
	cooldown := d.cooldown(d.ctx)
	for {
		d.mu.Lock()
		queue := d.pending[key]
		var ready time.Time
		if len(queue) > 0 {
			ready = queue[len(queue)-1].at.Add(d.quiet)
			if limit := queue[0].at.Add(d.gather); ready.After(limit) {
				ready = limit
			}
		}
		if last, ok := d.answered[key]; ok && cooldown > 0 {
			if next := last.Add(cooldown); next.After(ready) {
				ready = next
			}
		}
		d.mu.Unlock()
		delay := time.Until(ready)
		if delay <= 0 {
			return d.ctx.Err() == nil
		}
		if !wait(d.ctx, delay) {
			return false
		}
	}
}

// forgetOld drops cooldown memory nobody can still be waiting on.
func (d *aiDispatcher) forgetOld() {
	if len(d.answered) < 1024 {
		return
	}
	for key, at := range d.answered {
		if time.Since(at) > time.Hour {
			delete(d.answered, key)
		}
	}
}

// conversationKey separates group members: each sender in a group chat has
// their own queue, history and cooldown, while a private chat is one person.
func conversationKey(m Message) string {
	if m.ChatType == "private" {
		return m.Platform + "|" + m.ChatID
	}
	return m.Platform + "|" + m.ChatID + "|" + m.SenderID
}
