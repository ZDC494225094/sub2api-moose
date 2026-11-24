package iqmonitor

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// clock advances one second per read so ordering by time is stable.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(time.Second)
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// newTestStore runs the real module migration on SQLite next to minimal
// host tables (serial and timestamptz columns rewritten for SQLite).
func newTestStore(t *testing.T) (*Store, *clock) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT);
CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT NOT NULL, username TEXT NOT NULL DEFAULT '', deleted_at TEXT);
INSERT INTO users (id, email, username) VALUES (1, 'probe@example.com', 'probe'), (7, 'alice@example.com', 'alice'), (8, 'bob@example.com', '');`)
	require.NoError(t, err)
	raw, err := os.ReadFile("../../migrations/sql/iq-monitor/202611240001_iq_monitor.sql")
	require.NoError(t, err)
	ddl := strings.NewReplacer("BIGSERIAL PRIMARY KEY", "INTEGER PRIMARY KEY", "TIMESTAMPTZ", "TIMESTAMP").Replace(string(raw))
	_, err = db.Exec(ddl)
	require.NoError(t, err)
	var flag string
	require.NoError(t, db.QueryRow(`SELECT value FROM settings WHERE key = 'custom_extensions.iq-monitor.enabled'`).Scan(&flag))
	require.Equal(t, "false", flag, "new modules start switched off")
	c := &clock{t: time.Date(2026, 11, 24, 8, 0, 0, 0, time.UTC)}
	return &Store{DB: db, Now: c.now}, c
}

type fakeStates struct{ on bool }

func (f fakeStates) Enabled(context.Context, string) (bool, error) { return f.on, nil }

// fakeCaller answers by model name: "smart" is always right, "dumb" always
// wrong, "flaky" errors once then answers right, "learner" is wrong first.
type fakeCaller struct {
	mu    sync.Mutex
	calls map[string]int
	total atomic.Int64
}

const goodSVG = "```html\n<svg viewBox=\"0 0 10 10\"><circle cx=\"5\" cy=\"5\" r=\"3\"/></svg>\n```"

func (f *fakeCaller) Call(_ context.Context, req CallRequest) (CallResult, error) {
	f.total.Add(1)
	f.mu.Lock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	drawing := strings.Contains(req.Prompt, "SVG")
	// Calls are counted per model and test, since both tests run at once.
	counter := req.Model + map[bool]string{true: "/drawing", false: "/logic"}[drawing]
	f.calls[counter]++
	n := f.calls[counter]
	f.mu.Unlock()
	ms := 10
	res := CallResult{InputTokens: 100, OutputTokens: 50, FirstTokenMS: &ms, TotalMS: 20}
	switch {
	case strings.HasPrefix(req.Model, "review"):
		// Judge only the submission, not the rules text of the prompt.
		_, topic, _ := strings.Cut(req.Prompt, "<<<")
		if strings.Contains(topic, "政治") {
			res.Content = `{"approved": false, "reason": "涉及政治"}`
		} else if strings.Contains(topic, "坏掉") {
			return res, &CallError{Message: "HTTP 502", Retryable: true, Result: res}
		} else {
			res.Content = `{"approved": true, "reason": ""}`
		}
	case req.Model == "forbidden":
		res.InputTokens, res.OutputTokens = 0, 0
		return res, &CallError{Message: "HTTP 403：permission_error：This group is restricted to Claude Code clients", Retryable: false, Result: res}
	case req.Model == "flaky" && n == 1:
		return res, &CallError{Message: "HTTP 502：upstream", Retryable: true, Result: res}
	case drawing && req.Model == "dumb":
		res.Content, res.FinishReason, res.Truncated = "<svg viewBox=\"0 0 1 1\">", "length", true
	case drawing:
		res.Content = goodSVG
	case req.Model == "dumb" || (req.Model == "learner" && n == 1):
		res.Content = "答案：41"
	default:
		res.Content = "简单推理。\n答案：42"
	}
	return res, nil
}

type sent struct {
	mu    sync.Mutex
	texts []string
}

func newTestRuntime(t *testing.T) (*Runtime, *fakeCaller, *sent, *clock) {
	store, c := newTestStore(t)
	caller := &fakeCaller{}
	out := &sent{}
	host := Host{
		APIKey: func(_ context.Context, keyID, userID int64) (string, error) {
			if userID != 1 || keyID == 99 {
				return "", errors.New("not owned")
			}
			return "sk-test", nil
		},
		Groups: func(context.Context) ([]GroupOption, error) {
			return []GroupOption{{ID: 10, Name: "Claude 组", Platform: "anthropic", RateMultiplier: 1.5}}, nil
		},
		Effort: func(_ context.Context, _ int64, _ string, effort string) EffortPreview {
			if effort == "xhigh" {
				return EffortPreview{Sent: "max"}
			}
			return EffortPreview{Sent: effort}
		},
		Notify: func(_ context.Context, _ []int64, text string) error {
			out.mu.Lock()
			out.texts = append(out.texts, text)
			out.mu.Unlock()
			return nil
		},
	}
	r := NewRuntime(store, fakeStates{on: true}, host, caller)
	r.sleep = func(context.Context, time.Duration) bool { return true }
	r.ctx, r.cancel = context.WithCancel(context.Background())
	t.Cleanup(r.cancel)
	cfg := DefaultConfig()
	cfg.ProbeUserID = 1
	cfg.Notify = NotifyConfig{Enabled: true, BotIDs: []int64{1}}
	cfg.Submissions.ReviewAPIKeyID, cfg.Submissions.ReviewModel = 5, "review-model"
	require.NoError(t, store.SaveConfig(context.Background(), cfg))
	_, err := store.SaveQuestion(context.Background(), Question{Question: "6×7=?", Answers: []string{"42"}, Enabled: true})
	require.NoError(t, err)
	return r, caller, out, c
}

func addTarget(t *testing.T, s *Store, model, effort string) int64 {
	id, err := s.CreateTarget(context.Background(), Target{GroupID: 10, Model: model, DisplayName: model, ReasoningEffort: effort, APIKeyID: 5, Enabled: true})
	require.NoError(t, err)
	return id
}

func runOnce(t *testing.T, r *Runtime) Round {
	id, err := r.TriggerNow(context.Background())
	require.NoError(t, err)
	r.wg.Wait()
	round, err := r.Store.Round(context.Background(), id)
	require.NoError(t, err)
	return round
}

func resultsByModel(t *testing.T, s *Store, roundID int64, kind string) map[string]Result {
	list, err := s.RoundResults(context.Background(), roundID, kind)
	require.NoError(t, err)
	out := map[string]Result{}
	for _, r := range list {
		out[r.Model] = r
	}
	return out
}

func TestRoundSharesMaterialAndJudgesEveryTarget(t *testing.T) {
	r, caller, notes, _ := newTestRuntime(t)
	ctx := context.Background()
	for _, m := range []string{"smart", "dumb", "flaky", "learner"} {
		addTarget(t, r.Store, m, "")
	}
	addTarget(t, r.Store, "smart", "xhigh")
	round := runOnce(t, r)
	require.Equal(t, RoundDone, round.Status)
	require.Equal(t, "6×7=?", round.Question)
	require.Equal(t, "admin", round.TopicSource)
	require.Contains(t, DefaultTopics, round.Topic)

	logic := resultsByModel(t, r.Store, round.ID, KindLogic)
	require.Equal(t, StatusFail, logic["dumb"].Status)
	require.Equal(t, 2, logic["dumb"].Attempts, "default config re-asks once after a wrong answer")
	require.Equal(t, StatusRetryPass, logic["learner"].Status)
	require.Equal(t, StatusPass, logic["flaky"].Status)
	require.Equal(t, 2, logic["flaky"].RequestCount, "request-error retries are counted")
	require.EqualValues(t, 200, logic["flaky"].InputTokens, "tokens of every request are counted")
	responses, err := r.Store.Responses(ctx, logic["learner"].ID)
	require.NoError(t, err)
	require.Len(t, responses, 2, "the earlier wrong answer is kept")
	require.False(t, responses[0].Correct)
	require.True(t, responses[1].Correct)

	drawing := resultsByModel(t, r.Store, round.ID, KindDrawing)
	require.Equal(t, StatusFail, drawing["dumb"].Status)
	require.True(t, drawing["dumb"].Truncated)
	require.Equal(t, StatusPass, drawing["learner"].Status)

	all, err := r.Store.RoundResults(ctx, round.ID, KindLogic)
	require.NoError(t, err)
	var effortRow Result
	for _, res := range all {
		if res.ReasoningEffort == "xhigh" {
			effortRow = res
		}
	}
	require.Equal(t, "max", effortRow.SentEffort)

	// dumb fails logic → degraded → one notification; a second round keeps
	// it degraded and must not notify again.
	require.Len(t, notes.texts, 1)
	require.Contains(t, notes.texts[0], "dumb")
	require.NotContains(t, notes.texts[0], "smart")
	runOnce(t, r)
	require.Len(t, notes.texts, 1, "no repeated push while still degraded")

	ov, err := r.Overview(ctx)
	require.NoError(t, err)
	require.Len(t, ov.Models, 5)
	require.Equal(t, "smart", ov.Models[0].Model, "cards follow target order")
	for _, m := range ov.Models {
		if m.Model == "dumb" {
			require.Equal(t, HealthDegraded, m.Health)
			require.Len(t, m.Logic.Timeline, 2)
		}
	}
	require.NotNil(t, ov.Wall)
	require.Len(t, ov.Wall.Cards, 5)
	require.Equal(t, 1.5, ov.Wall.Cards[0].RateMultiplier)
	require.Greater(t, caller.total.Load(), int64(10))
}

func TestSlotClaimRunsOncePerCluster(t *testing.T) {
	r, _, _, _ := newTestRuntime(t)
	addTarget(t, r.Store, "smart", "")
	other := NewRuntime(r.Store, fakeStates{on: true}, r.Host, r.Caller)
	other.ctx = r.ctx
	cfg, _, err := r.Store.Config(context.Background())
	require.NoError(t, err)
	_, err = r.startRound(context.Background(), cfg, "slot:30:1", "schedule")
	require.NoError(t, err)
	_, err = other.startRound(context.Background(), cfg, "slot:30:1", "schedule")
	require.ErrorIs(t, err, errSlotTaken)
	r.wg.Wait()
}

func TestStaleRoundIsInterrupted(t *testing.T) {
	r, _, _, c := newTestRuntime(t)
	ctx := context.Background()
	id, ok, err := r.Store.ClaimRound(ctx, Round{SlotKey: "slot:x", Trigger: "schedule", NodeID: "gone", LogicEnabled: true})
	require.NoError(t, err)
	require.True(t, ok)
	resID, err := r.Store.CreateResult(ctx, Result{RoundID: id, TargetID: 1, Kind: KindLogic, GroupID: 10, Model: "m"})
	require.NoError(t, err)
	require.NoError(t, r.Store.StartResult(ctx, resID))
	c.advance(10 * time.Minute)
	n, err := r.Store.InterruptStale(ctx, r.Store.now().Add(-staleAfter))
	require.NoError(t, err)
	require.Equal(t, 1, n)
	res, err := r.Store.Result(ctx, resID)
	require.NoError(t, err)
	require.Equal(t, StatusInterrupted, res.Status)
	round, err := r.Store.Round(ctx, id)
	require.NoError(t, err)
	require.Equal(t, RoundInterrupted, round.Status)
	// A late verdict from the stopped node must not overwrite it.
	res.Status = StatusPass
	require.NoError(t, r.Store.SaveResult(ctx, res))
	res, _ = r.Store.Result(ctx, resID)
	require.Equal(t, StatusInterrupted, res.Status)
}

func TestSubmissionQuotaReviewAndQueue(t *testing.T) {
	r, _, _, _ := newTestRuntime(t)
	ctx := context.Background()
	_, err := r.Submit(ctx, 7, "一只猫在月亮上钓鱼")
	require.NoError(t, err)
	_, err = r.Submit(ctx, 7, "谈谈政治人物的选举")
	require.NoError(t, err)
	_, err = r.Submit(ctx, 7, "第三个主题会超额")
	require.ErrorIs(t, err, ErrQuotaExceeded)
	_, err = r.Submit(ctx, 8, "x")
	require.Error(t, err, "too short")

	require.True(t, r.reviewOne(ctx))
	require.True(t, r.reviewOne(ctx))
	require.False(t, r.reviewOne(ctx))
	mine, err := r.Submissions(ctx, 7)
	require.NoError(t, err)
	require.Equal(t, 2, mine.Used)
	statuses := map[string]Topic{}
	for _, it := range mine.Items {
		statuses[it.Topic] = it
	}
	require.Equal(t, TopicApproved, statuses["一只猫在月亮上钓鱼"].Status)
	require.Equal(t, TopicRejected, statuses["谈谈政治人物的选举"].Status)
	require.Equal(t, "涉及政治", statuses["谈谈政治人物的选举"].RejectReason)

	ov, err := r.Overview(ctx)
	require.NoError(t, err)
	require.Len(t, ov.Queue, 1)
	require.Equal(t, "a***e", ov.Queue[0].Submitter, "public names are masked")

	addTarget(t, r.Store, "smart", "")
	round := runOnce(t, r)
	require.Equal(t, "一只猫在月亮上钓鱼", round.Topic, "approved submissions are used before the admin list")
	require.Equal(t, "user", round.TopicSource)
	ov, err = r.Overview(ctx)
	require.NoError(t, err)
	require.Equal(t, "a***e", ov.Wall.Submitter)
	require.Empty(t, ov.Queue)
}

func TestFailedReviewsRetryThenFreeQuota(t *testing.T) {
	r, _, _, c := newTestRuntime(t)
	ctx := context.Background()
	_, err := r.Submit(ctx, 8, "坏掉的审核服务测试")
	require.NoError(t, err)
	for i := 0; i < MaxReviewAttempts; i++ {
		require.True(t, r.reviewOne(ctx), "attempt %d", i+1)
		c.advance(10 * time.Minute)
	}
	require.False(t, r.reviewOne(ctx))
	mine, err := r.Submissions(ctx, 8)
	require.NoError(t, err)
	require.Equal(t, TopicFailed, mine.Items[0].Status)
	require.Equal(t, MaxReviewAttempts, mine.Items[0].Attempts)
	require.Equal(t, 0, mine.Used, "failed reviews do not use the daily quota")
}

func TestReviewLockIsExclusive(t *testing.T) {
	r, _, _, c := newTestRuntime(t)
	ctx := context.Background()
	_, err := r.Store.SubmitTopic(ctx, 7, "一只鹿在雪地里奔跑", "2026-11-24", 2)
	require.NoError(t, err)
	a, err := r.Store.ClaimReview(ctx, "node-a", reviewLock)
	require.NoError(t, err)
	require.NotNil(t, a)
	b, err := r.Store.ClaimReview(ctx, "node-b", reviewLock)
	require.NoError(t, err)
	require.Nil(t, b, "a locked submission is not reviewed twice")
	c.advance(reviewLock + time.Minute)
	b, err = r.Store.ClaimReview(ctx, "node-b", reviewLock)
	require.NoError(t, err)
	require.NotNil(t, b, "an expired lock is taken over")
	require.NoError(t, r.Store.FinishReview(ctx, a.ID, "node-a", true, ""))
	got, _ := r.Store.Topic(ctx, a.ID)
	require.Equal(t, TopicReviewing, got.Status, "the old holder cannot write a verdict")
}

func TestClearHistoryAndTargetLimits(t *testing.T) {
	r, _, _, _ := newTestRuntime(t)
	ctx := context.Background()
	addTarget(t, r.Store, "smart", "")
	addTarget(t, r.Store, "smart", "high")
	addTarget(t, r.Store, "dumb", "")
	runOnce(t, r)
	n, err := r.Store.ClearHistory(ctx, false, 10, "smart")
	require.NoError(t, err)
	require.EqualValues(t, 4, n, "both efforts, both kinds")
	ov, err := r.Overview(ctx)
	require.NoError(t, err)
	for _, m := range ov.Models {
		if m.Model == "smart" {
			require.Empty(t, m.Logic.Timeline)
		} else {
			require.Len(t, m.Logic.Timeline, 1)
		}
	}
	_, err = r.Submit(ctx, 7, "保留下来的投稿主题")
	require.NoError(t, err)
	_, err = r.Store.ClearHistory(ctx, true, 0, "")
	require.NoError(t, err)
	mine, _ := r.Submissions(ctx, 7)
	require.Len(t, mine.Items, 1, "submissions survive a full clear")

	for i := 3; i < MaxTargets; i++ {
		addTarget(t, r.Store, "m", "")
	}
	_, err = r.Store.CreateTarget(ctx, Target{GroupID: 10, Model: "over", APIKeyID: 5})
	require.Error(t, err)
}

func TestPruneKeepsQueuedTopics(t *testing.T) {
	r, _, _, c := newTestRuntime(t)
	ctx := context.Background()
	addTarget(t, r.Store, "smart", "")
	runOnce(t, r)
	_, err := r.Submit(ctx, 7, "排队中的投稿主题")
	require.NoError(t, err)
	c.advance(48 * time.Hour)
	require.NoError(t, r.Store.Prune(ctx, r.Store.now().Add(-24*time.Hour)))
	latest, err := r.Store.LatestRound(ctx)
	require.NoError(t, err)
	require.Nil(t, latest)
	n, err := r.Store.CountTopics(ctx, TopicPending)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

func TestEstimateUsesAverageTokens(t *testing.T) {
	r, _, _, _ := newTestRuntime(t)
	r.Host.Cost = func(context.Context, int64, string, time.Time) (UnitCost, error) {
		return UnitCost{InputPerToken: 0.000001, OutputPerToken: 0.00001, Samples: 3}, nil
	}
	addTarget(t, r.Store, "smart", "")
	runOnce(t, r)
	est, err := r.EstimateDaily(context.Background())
	require.NoError(t, err)
	require.Equal(t, 48.0, est.RoundsPerDay)
	require.True(t, est.Complete)
	require.Len(t, est.Targets, 1)
	// 100 in + 50 out per test, two tests, 48 rounds.
	require.InDelta(t, 48*2*(100*0.000001+50*0.00001), est.CostDaily, 1e-9)
}

func TestForbiddenRequestKeepsGatewayErrorAndIsNotRetried(t *testing.T) {
	r, _, _, _ := newTestRuntime(t)
	ctx := context.Background()
	addTarget(t, r.Store, "forbidden", "")
	addTarget(t, r.Store, "smart", "")
	round := runOnce(t, r)

	logic := resultsByModel(t, r.Store, round.ID, KindLogic)["forbidden"]
	require.Equal(t, StatusError, logic.Status)
	require.Equal(t, 1, logic.RequestCount, "403 is not retried")
	require.Contains(t, logic.Reason, "Claude Code clients")
	responses, err := r.Store.Responses(ctx, logic.ID)
	require.NoError(t, err)
	require.Len(t, responses, 1)
	require.Contains(t, responses[0].Error, "permission_error", "the gateway error is kept on the reply")

	problems, err := r.Store.RecentProblems(ctx, 30)
	require.NoError(t, err)
	require.Len(t, problems, 2, "both tests of the forbidden target, nothing from the passing one")
	for _, p := range problems {
		require.Equal(t, "forbidden", p.Model)
		require.Contains(t, p.Reason, "HTTP 403")
	}
}
