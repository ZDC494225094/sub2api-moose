package iqmonitor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// runRound executes a claimed round. Every target gets the same question
// and topic. Results are created up front as pending so the public page can
// show "drawing in progress" cards and a stopped node leaves rows that the
// stale sweep marks interrupted.
func (r *Runtime) runRound(ctx context.Context, cfg Config, round Round, targets []Target) {
	bg := context.WithoutCancel(ctx)
	if err := r.prepareMaterial(ctx, cfg, &round); err != nil {
		r.report(err)
	}
	if err := r.Store.SetRoundMaterial(ctx, round); err != nil {
		r.report(err)
	}
	groups := map[int64]GroupOption{}
	if r.Host.Groups != nil {
		if list, err := r.Host.Groups(ctx); err == nil {
			for _, g := range list {
				groups[g.ID] = g
			}
		}
	}
	type job struct {
		target Target
		result Result
	}
	var jobs []job
	for _, t := range targets {
		for _, kind := range []string{KindLogic, KindDrawing} {
			if (kind == KindLogic && !round.LogicEnabled) || (kind == KindDrawing && !round.DrawingEnabled) {
				continue
			}
			g := groups[t.GroupID]
			sent := ""
			if t.ReasoningEffort != "" && r.Host.Effort != nil {
				sent = r.Host.Effort(ctx, t.GroupID, t.Model, t.ReasoningEffort).Sent
			}
			res := Result{RoundID: round.ID, TargetID: t.ID, Kind: kind, GroupID: t.GroupID, GroupName: g.Name, Model: t.Model,
				DisplayName: t.DisplayName, ReasoningEffort: t.ReasoningEffort, SentEffort: sent}
			id, err := r.Store.CreateResult(ctx, res)
			if err != nil {
				r.report(err)
				continue
			}
			res.ID = id
			jobs = append(jobs, job{t, res})
		}
	}
	hbCtx, stopHB := context.WithCancel(ctx)
	go func() {
		t := time.NewTicker(heartbeatEvery)
		defer t.Stop()
		for {
			select {
			case <-hbCtx.Done():
				return
			case <-t.C:
				_ = r.Store.Heartbeat(hbCtx, round.ID)
			}
		}
	}()
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			res := j.result
			if j.target.APIKeyID == 0 || cfg.ProbeUserID == 0 {
				res.Status, res.Reason = StatusSkipped, "未配置探测账号或令牌"
				_ = r.Store.SaveResult(bg, res)
				return
			}
			switch res.Kind {
			case KindLogic:
				r.runLogic(ctx, cfg, round, j.target, &res)
			case KindDrawing:
				r.runDrawing(ctx, cfg, round, j.target, &res)
			}
			if ctx.Err() != nil {
				// Shutdown: the round is marked interrupted below.
				return
			}
			if err := r.Store.SaveResult(bg, res); err != nil {
				r.report(err)
			}
		}(j)
	}
	wg.Wait()
	stopHB()
	if ctx.Err() != nil {
		r.report(r.Store.InterruptRound(bg, round.ID, "节点停止，请求未完成"))
		return
	}
	r.report(r.Store.FinishRound(bg, round.ID, RoundDone))
	r.evaluateAlerts(bg, cfg, targets)
}

// prepareMaterial picks the round's shared question and topic. A test with
// nothing to ask is turned off for the round.
func (r *Runtime) prepareMaterial(ctx context.Context, cfg Config, round *Round) error {
	var errs []string
	if round.LogicEnabled {
		q, err := r.Store.PickQuestion(ctx, randIndex)
		switch {
		case err != nil:
			errs = append(errs, err.Error())
			round.LogicEnabled = false
		case q == nil:
			round.LogicEnabled = false
		default:
			round.QuestionID, round.Question, round.Answers = &q.ID, q.Question, q.Answers
			_ = r.Store.MarkQuestionUsed(ctx, q.ID)
		}
	}
	if round.DrawingEnabled {
		topic, err := r.Store.TakeQueuedTopic(ctx, round.ID)
		if err != nil {
			errs = append(errs, err.Error())
		}
		if topic != nil {
			round.Topic, round.TopicSource, round.TopicID = topic.Topic, "user", &topic.ID
		} else if list := cfg.TopicList(); len(list) > 0 {
			round.Topic, round.TopicSource = list[randIndex(len(list))], "admin"
		} else {
			round.DrawingEnabled = false
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (r *Runtime) acquire(ctx context.Context) bool {
	select {
	case r.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (r *Runtime) release() { <-r.sem }

// call sends one prompt with request-error retries, recording every
// request's tokens on the result. It returns the last outcome.
func (r *Runtime) call(ctx context.Context, cfg Config, key string, t Target, prompt string, res *Result) (CallResult, error) {
	var last CallResult
	var lastErr error
	for attempt := 0; attempt <= cfg.RequestRetries; attempt++ {
		if attempt > 0 && !r.sleep(ctx, time.Duration(attempt)*3*time.Second) {
			return last, ctx.Err()
		}
		if !r.acquire(ctx) {
			return last, ctx.Err()
		}
		out, err := r.Caller.Call(ctx, CallRequest{APIKey: key, Model: t.Model, Prompt: prompt, ReasoningEffort: t.ReasoningEffort,
			MaxOutputTokens: cfg.MaxOutputTokens, Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second})
		r.release()
		var callErr *CallError
		if errors.As(err, &callErr) {
			out = callErr.Result
		}
		res.RequestCount++
		res.InputTokens += out.InputTokens
		res.OutputTokens += out.OutputTokens
		last, lastErr = out, err
		if err == nil || ctx.Err() != nil {
			return out, err
		}
		if callErr == nil || !callErr.Retryable {
			return out, err
		}
	}
	return last, lastErr
}

func (r *Runtime) probeKey(ctx context.Context, cfg Config, t Target) (string, error) {
	key, err := r.Host.APIKey(ctx, t.APIKeyID, cfg.ProbeUserID)
	if err != nil {
		return "", fmt.Errorf("令牌不可用：%v", err)
	}
	return key, nil
}

func setTiming(res *Result, out CallResult) {
	res.FirstTokenMS = out.FirstTokenMS
	ms := out.TotalMS
	res.TotalMS = &ms
	res.Truncated = res.Truncated || out.Truncated
}

func (r *Runtime) runLogic(ctx context.Context, cfg Config, round Round, t Target, res *Result) {
	_ = r.Store.StartResult(ctx, res.ID)
	key, err := r.probeKey(ctx, cfg, t)
	if err != nil {
		res.Status, res.Reason = StatusSkipped, err.Error()
		return
	}
	prompt := renderPrompt(cfg.Logic.PromptTemplate, "{question}", round.Question)
	for attempt := 1; attempt <= 1+cfg.Logic.RetryWrong; attempt++ {
		res.Attempts = attempt
		out, err := r.call(ctx, cfg, key, t, prompt, res)
		if ctx.Err() != nil {
			return
		}
		setTiming(res, out)
		resp := Response{ResultID: res.ID, Attempt: attempt, Content: out.Content, FinishReason: out.FinishReason, Truncated: out.Truncated,
			InputTokens: out.InputTokens, OutputTokens: out.OutputTokens, FirstTokenMS: out.FirstTokenMS, TotalMS: &out.TotalMS}
		if err != nil {
			resp.Error = err.Error()
			_ = r.Store.AddResponse(ctx, resp)
			res.Status, res.Reason = StatusError, err.Error()
			return
		}
		extracted, ok := JudgeLogic(out.Content, round.Answers)
		resp.Extracted, resp.Correct = extracted, ok
		_ = r.Store.AddResponse(ctx, resp)
		if ok {
			res.Status, res.Reason = StatusPass, ""
			if attempt > 1 {
				res.Status = StatusRetryPass
			}
			return
		}
		res.Status = StatusFail
		switch {
		case out.Truncated:
			res.Reason = "回复达到输出上限被截断"
		case extracted == "":
			res.Reason = "没有找到「答案：」行或 \\boxed{}"
		default:
			res.Reason = "答案错误：" + truncateRunes(extracted, 100)
		}
	}
}

func (r *Runtime) runDrawing(ctx context.Context, cfg Config, round Round, t Target, res *Result) {
	_ = r.Store.StartResult(ctx, res.ID)
	key, err := r.probeKey(ctx, cfg, t)
	if err != nil {
		res.Status, res.Reason = StatusSkipped, err.Error()
		return
	}
	res.Attempts = 1
	out, err := r.call(ctx, cfg, key, t, renderPrompt(cfg.Drawing.PromptTemplate, "{topic}", round.Topic), res)
	if ctx.Err() != nil {
		return
	}
	setTiming(res, out)
	resp := Response{ResultID: res.ID, Attempt: 1, Content: out.Content, FinishReason: out.FinishReason, Truncated: out.Truncated,
		InputTokens: out.InputTokens, OutputTokens: out.OutputTokens, FirstTokenMS: out.FirstTokenMS, TotalMS: &out.TotalMS}
	if err != nil {
		resp.Error = err.Error()
		_ = r.Store.AddResponse(ctx, resp)
		res.Status, res.Reason = StatusError, err.Error()
		return
	}
	ok, reason := JudgeDrawing(out.Content, out.Truncated)
	resp.Correct = ok
	_ = r.Store.AddResponse(ctx, resp)
	if ok {
		res.Status, res.Reason = StatusPass, ""
	} else {
		res.Status, res.Reason = StatusFail, reason
	}
}

// evaluateAlerts recomputes 24h health for the round's targets and pushes
// one bot message listing models that just entered degraded.
func (r *Runtime) evaluateAlerts(ctx context.Context, cfg Config, targets []Target) {
	now := r.Store.now()
	results, err := r.Store.ResultsSince(ctx, now.Add(-24*time.Hour))
	if err != nil {
		r.report(err)
		return
	}
	stats := statsSince(results, now.Add(-24*time.Hour))
	seen := map[historyKey]bool{}
	var lines []string
	for _, t := range targets {
		k := keyOf(t)
		if seen[k] {
			continue
		}
		seen[k] = true
		health, s := healthOf(cfg, stats[k])
		entered, err := r.Store.TransitionAlert(ctx, k, health)
		if err != nil {
			r.report(err)
			continue
		}
		if entered {
			lines = append(lines, fmt.Sprintf("• %s：24 小时%s通过率 %.1f%%（%d/%d）", targetLabel(t), kindLabel(cfg.PrimaryKind()), s.Rate, s.Passed, s.Total))
		}
	}
	if len(lines) == 0 || !cfg.Notify.Enabled || len(cfg.Notify.BotIDs) == 0 || r.Host.Notify == nil {
		return
	}
	sort.Strings(lines)
	text := "【降智监测】以下模型进入「疑似降智」状态：\n" + strings.Join(lines, "\n")
	if err := r.Host.Notify(ctx, cfg.Notify.BotIDs, text); err != nil {
		r.report(fmt.Errorf("推送降智通知失败：%w", err))
	}
}

func targetLabel(t Target) string {
	label := t.DisplayName
	if label == "" {
		label = t.Model
	}
	if t.ReasoningEffort != "" {
		label += "（" + t.ReasoningEffort + "）"
	}
	return label
}

func kindLabel(kind string) string {
	if kind == KindDrawing {
		return "画图测试"
	}
	return "逻辑测试"
}
