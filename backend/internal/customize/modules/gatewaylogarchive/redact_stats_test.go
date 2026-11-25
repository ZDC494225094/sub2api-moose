package gatewaylogarchive

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRedactionCountsReplacementsByKind(t *testing.T) {
	det := &wordDetector{words: []string{"王小明"}, label: "private_person"}
	totals := &redactStats{}
	progress := &uploadProgress{}
	ctx := context.WithValue(context.Background(), uploadProgressKey{}, progress)
	docs := []string{
		// Rules: email, phone, ID card, labeled address, a personal field; the
		// model finds the name twice, once inside nested JSON.
		`{"messages":[{"role":"user","content":"我是王小明，邮箱 a@example.com，手机 13800138000，身份证 110101199003076632，家庭住址：朝阳路1号"}],"customer":{"phone":12345},"arguments":"{\"who\":\"王小明\"}"}`,
		`{"messages":[{"role":"user","content":"hello"}]}`,
		`{"tool_calls":[{"function":{"arguments":"王小明"}}],"content":"ok"}`,
	}
	for i, doc := range docs {
		var out bytes.Buffer
		require.NoError(t, redactJSON(ctx, &out, strings.NewReader(doc), true, redactOptions{det: det, stats: totals, naturalOnly: i == 2}), doc)
	}
	for _, v := range []RedactionCounts{totals.view(), progress.redaction.view()} {
		require.EqualValues(t, 3, v.Records)
		require.EqualValues(t, 1, v.Changed)
		require.EqualValues(t, 1, v.ToolSkipped)
		require.Equal(t, map[string]int64{
			ruleEmail: 1, rulePhone: 1, ruleIDCard: 1, ruleAddress: 1, ruleField: 1,
			modelPrefix + "private_person": 2,
		}, v.Replacements)
	}
}

func TestRedactionCountsIgnoreAlreadyMaskedText(t *testing.T) {
	counts := &docCounts{}
	require.Equal(t, "家庭住址：[REDACTED]", redactPlainText("家庭住址：[REDACTED]", counts))
	require.Empty(t, counts.kinds)
}

func TestUploadHistoryRecordsAttempts(t *testing.T) {
	var h uploadHistory
	ok := &uploadProgress{FileName: "a.jsonl", bucket: "main", startedAt: time.Now().Add(-2 * time.Second), redacted: true}
	ok.sent.Store(100)
	done := ok.network.begin()
	time.Sleep(20 * time.Millisecond)
	done()
	ok.redaction.merge(&docCounts{kinds: map[string]int64{ruleEmail: 2}})
	h.record(ok, true)
	failed := &uploadProgress{FileName: "b.jsonl", startedAt: time.Now()}
	h.record(failed, false)

	v := h.view()
	require.EqualValues(t, 1, v.Totals.Uploads)
	require.EqualValues(t, 1, v.Totals.Failed)
	require.EqualValues(t, 100, v.Totals.Bytes)
	require.GreaterOrEqual(t, v.Totals.DurationMS, int64(2000))
	require.GreaterOrEqual(t, v.Totals.NetworkMS, int64(20))
	require.Len(t, v.Recent, 2)
	require.Equal(t, "b.jsonl", v.Recent[0].FileName) // newest first
	require.False(t, v.Recent[0].OK)
	require.Equal(t, map[string]int64{ruleEmail: 2}, v.Recent[1].Redaction.Replacements)
	require.False(t, v.Totals.Since.IsZero())

	for i := 0; i < recentUploads+5; i++ {
		h.record(&uploadProgress{startedAt: time.Now()}, true)
	}
	require.Len(t, h.view().Recent, recentUploads)
}

func TestNetworkClockCountsParallelRequestsOnce(t *testing.T) {
	var c networkClock
	a, b := c.begin(), c.begin()
	time.Sleep(30 * time.Millisecond)
	a()
	b()
	require.Less(t, c.elapsed(), 55*time.Millisecond)
	require.GreaterOrEqual(t, c.elapsed(), 30*time.Millisecond)
	require.NotPanics(t, func() { (*networkClock)(nil).begin()() })
}
