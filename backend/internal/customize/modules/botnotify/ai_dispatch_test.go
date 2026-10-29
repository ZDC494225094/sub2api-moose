package botnotify

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type batchRecorder struct {
	mu      sync.Mutex
	batches [][]string
	at      []time.Time
	running int
	peak    int
	hold    time.Duration
}

func (b *batchRecorder) answer(_ context.Context, items []aiItem) {
	b.mu.Lock()
	b.running++
	if b.running > b.peak {
		b.peak = b.running
	}
	texts := make([]string, 0, len(items))
	for _, it := range items {
		texts = append(texts, it.msg.Text)
	}
	b.batches = append(b.batches, texts)
	b.at = append(b.at, time.Now())
	b.mu.Unlock()
	time.Sleep(b.hold)
	b.mu.Lock()
	b.running--
	b.mu.Unlock()
}

func testDispatcher(ctx context.Context, rec *batchRecorder, cooldown time.Duration) *aiDispatcher {
	d := newAIDispatcher(ctx, rec.answer, func(context.Context) time.Duration { return cooldown })
	d.quiet = 40 * time.Millisecond
	d.gather = 200 * time.Millisecond
	return d
}

func item(text string) aiItem { return aiItem{msg: Message{Text: text}, at: time.Now()} }

func TestDispatcherMergesFollowUpMessages(t *testing.T) {
	rec := &batchRecorder{}
	d := testDispatcher(context.Background(), rec, 0)
	require.True(t, d.enqueue("chat", item("[截图]")))
	time.Sleep(10 * time.Millisecond)
	require.True(t, d.enqueue("chat", item("这个报错怎么解决")))
	d.wait()
	require.Equal(t, [][]string{{"[截图]", "这个报错怎么解决"}}, rec.batches)
}

func TestDispatcherDelaysInsteadOfDroppingDuringCooldown(t *testing.T) {
	rec := &batchRecorder{hold: 30 * time.Millisecond}
	d := testDispatcher(context.Background(), rec, 150*time.Millisecond)
	require.True(t, d.enqueue("chat", item("第一个问题")))
	// Arrives while the first answer is running: queued for the next batch.
	time.Sleep(60 * time.Millisecond)
	require.True(t, d.enqueue("chat", item("第二个问题")))
	d.wait()
	require.Equal(t, [][]string{{"第一个问题"}, {"第二个问题"}}, rec.batches)
	require.GreaterOrEqual(t, rec.at[1].Sub(rec.at[0]), 150*time.Millisecond)
}

func TestDispatcherKeepsConversationsApartAndBoundsConcurrency(t *testing.T) {
	rec := &batchRecorder{hold: 60 * time.Millisecond}
	d := testDispatcher(context.Background(), rec, 0)
	for _, key := range []string{"a", "b", "c", "d", "e"} {
		require.True(t, d.enqueue(key, item(key)))
	}
	d.wait()
	require.Len(t, rec.batches, 5)
	require.LessOrEqual(t, rec.peak, aiConcurrency)
}

func TestDispatcherStopsOnCancelAndRefusesNewWork(t *testing.T) {
	rec := &batchRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	d := testDispatcher(ctx, rec, 0)
	require.True(t, d.enqueue("chat", item("会被取消")))
	cancel()
	d.wait()
	require.Empty(t, rec.batches)
	require.False(t, d.enqueue("chat", item("停止后")))
}

func TestConversationKeySeparatesGroupMembers(t *testing.T) {
	a := Message{Platform: "telegram", ChatID: "-1", ChatType: "group", SenderID: "7"}
	b := a
	b.SenderID = "8"
	require.NotEqual(t, conversationKey(a), conversationKey(b))
	p := Message{Platform: "qq", ChatID: "user:U1", ChatType: "private", SenderID: "U1"}
	require.Equal(t, "qq|user:U1", conversationKey(p))
}

func TestMergeAIItemsKeepsLatestImages(t *testing.T) {
	items := []aiItem{
		{msg: Message{Text: "第一张", Images: []ImageRef{{ID: "1"}, {ID: "2"}}}},
		{msg: Message{Images: []ImageRef{{ID: "3"}, {ID: "4"}}}},
		{msg: Message{Text: "怎么办", MessageID: 9}},
	}
	merged := mergeAIItems(items)
	require.Equal(t, "第一张\n怎么办", merged.Text)
	require.EqualValues(t, 9, merged.MessageID)
	require.Equal(t, []ImageRef{{ID: "2"}, {ID: "3"}, {ID: "4"}}, merged.Images)
	require.Equal(t, "第一张\n怎么办\n[图片×3]", auditQuestion(merged))
}
