package iqmonitor

import (
	"context"
	"time"
)

type KindEstimate struct {
	Samples         int     `json:"samples"`
	AvgRequests     float64 `json:"avg_requests"`
	AvgInputTokens  float64 `json:"avg_input_tokens"`
	AvgOutputTokens float64 `json:"avg_output_tokens"`
	// CostPerRun is nil when no usage log prices this key and model yet.
	CostPerRun *float64 `json:"cost_per_run"`
}

type TargetEstimate struct {
	TargetID    int64         `json:"target_id"`
	DisplayName string        `json:"display_name"`
	Model       string        `json:"model"`
	Logic       *KindEstimate `json:"logic,omitempty"`
	Drawing     *KindEstimate `json:"drawing,omitempty"`
	TokensDaily float64       `json:"tokens_daily"`
	CostDaily   *float64      `json:"cost_daily"`
}

type Estimate struct {
	RoundsPerDay float64          `json:"rounds_per_day"`
	WindowDays   int              `json:"window_days"`
	Targets      []TargetEstimate `json:"targets"`
	TokensDaily  float64          `json:"tokens_daily"`
	CostDaily    float64          `json:"cost_daily"`
	// Complete is false when some enabled target has no price sample yet.
	Complete bool `json:"complete"`
}

const estimateWindowDays = 7

// EstimateDaily projects daily spend from each target's average token use
// per test over the last week and the probe key's observed unit prices.
func (r *Runtime) EstimateDaily(ctx context.Context) (Estimate, error) {
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		return Estimate{}, err
	}
	now := r.Store.now()
	since := now.AddDate(0, 0, -estimateWindowDays)
	out := Estimate{RoundsPerDay: 1440 / float64(cfg.IntervalMinutes), WindowDays: estimateWindowDays, Targets: []TargetEstimate{}, Complete: true}
	targets, err := r.enabledTargets(ctx)
	if err != nil {
		return out, err
	}
	results, err := r.Store.ResultsSince(ctx, since)
	if err != nil {
		return out, err
	}
	type acc struct{ n, req, in, outTok float64 }
	sums := map[historyKey]map[string]*acc{}
	for _, res := range results {
		if res.RequestCount == 0 {
			continue
		}
		k := historyKey{GroupID: res.GroupID, Model: res.Model, Effort: res.ReasoningEffort}
		if sums[k] == nil {
			sums[k] = map[string]*acc{}
		}
		a := sums[k][res.Kind]
		if a == nil {
			a = &acc{}
			sums[k][res.Kind] = a
		}
		a.n++
		a.req += float64(res.RequestCount)
		a.in += float64(res.InputTokens)
		a.outTok += float64(res.OutputTokens)
	}
	for _, t := range targets {
		te := TargetEstimate{TargetID: t.ID, DisplayName: t.DisplayName, Model: t.Model}
		var unit *UnitCost
		if r.Host.Cost != nil {
			if u, err := r.Host.Cost(ctx, t.APIKeyID, t.Model, since); err == nil && u.Samples > 0 {
				unit = &u
			}
		}
		daily, priced := 0.0, true
		for _, kind := range []string{KindLogic, KindDrawing} {
			if (kind == KindLogic && !cfg.Logic.Enabled) || (kind == KindDrawing && !cfg.Drawing.Enabled) {
				continue
			}
			ke := &KindEstimate{}
			if a := sums[keyOf(t)][kind]; a != nil {
				ke.Samples = int(a.n)
				ke.AvgRequests, ke.AvgInputTokens, ke.AvgOutputTokens = a.req/a.n, a.in/a.n, a.outTok/a.n
			}
			te.TokensDaily += (ke.AvgInputTokens + ke.AvgOutputTokens) * out.RoundsPerDay
			if unit != nil && ke.Samples > 0 {
				c := ke.AvgInputTokens*unit.InputPerToken + ke.AvgOutputTokens*unit.OutputPerToken
				ke.CostPerRun = &c
				daily += c * out.RoundsPerDay
			} else {
				priced = false
			}
			if kind == KindLogic {
				te.Logic = ke
			} else {
				te.Drawing = ke
			}
		}
		if priced {
			te.CostDaily = &daily
			out.CostDaily += daily
		} else {
			out.Complete = false
		}
		out.TokensDaily += te.TokensDaily
		out.Targets = append(out.Targets, te)
	}
	return out, nil
}

type RuntimeStatus struct {
	NodeID       string     `json:"node_id"`
	LocalRunning bool       `json:"local_running"`
	LastError    string     `json:"last_error"`
	Running      *Round     `json:"running"`
	Latest       *Round     `json:"latest"`
	PendingTopic int        `json:"pending_topics"`
	QueuedTopic  int        `json:"queued_topics"`
	Now          time.Time  `json:"now"`
	NextRoundAt  *time.Time `json:"next_round_at"`
}

func (r *Runtime) Status(ctx context.Context) (RuntimeStatus, error) {
	r.mu.Lock()
	st := RuntimeStatus{NodeID: r.NodeID, LocalRunning: r.running, LastError: r.lastErr, Now: r.Store.now()}
	r.mu.Unlock()
	var err error
	if st.Running, err = r.Store.RunningRound(ctx, st.Now.Add(-staleAfter)); err != nil {
		return st, err
	}
	if st.Latest, err = r.Store.LatestRound(ctx); err != nil {
		return st, err
	}
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		return st, err
	}
	st.NextRoundAt = nextRoundAt(cfg, st.Now, st.Latest)
	if st.PendingTopic, err = r.Store.CountTopics(ctx, TopicPending); err != nil {
		return st, err
	}
	n, err := r.Store.CountTopics(ctx, TopicReviewing)
	if err != nil {
		return st, err
	}
	st.PendingTopic += n
	if st.QueuedTopic, err = r.Store.CountTopics(ctx, TopicApproved); err != nil {
		return st, err
	}
	return st, nil
}
