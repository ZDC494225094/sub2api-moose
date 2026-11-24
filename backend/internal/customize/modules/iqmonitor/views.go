package iqmonitor

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const timelineLength = 48

// Cell is one round in a model's timeline.
type Cell struct {
	ResultID  int64     `json:"result_id"`
	RoundID   int64     `json:"round_id"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	Truncated bool      `json:"truncated,omitempty"`
	TotalMS   *int      `json:"total_ms,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type KindView struct {
	Stat     Stat   `json:"stat"`
	Timeline []Cell `json:"timeline"`
}

type ModelCard struct {
	TargetID        int64     `json:"target_id"`
	DisplayName     string    `json:"display_name"`
	Model           string    `json:"model"`
	GroupID         int64     `json:"group_id"`
	GroupName       string    `json:"group_name"`
	Platform        string    `json:"platform"`
	RateMultiplier  float64   `json:"rate_multiplier"`
	ReasoningEffort string    `json:"reasoning_effort"`
	SentEffort      string    `json:"sent_effort"`
	Health          string    `json:"health"`
	HealthRate      Stat      `json:"health_rate"`
	Logic           *KindView `json:"logic,omitempty"`
	Drawing         *KindView `json:"drawing,omitempty"`
	AvgFirstTokenMS *int      `json:"avg_first_token_ms"`
	AvgTotalMS      *int      `json:"avg_total_ms"`
}

type WallCard struct {
	ResultID        int64      `json:"result_id"`
	TargetID        int64      `json:"target_id"`
	DisplayName     string     `json:"display_name"`
	Model           string     `json:"model"`
	GroupName       string     `json:"group_name"`
	Platform        string     `json:"platform"`
	RateMultiplier  float64    `json:"rate_multiplier"`
	ReasoningEffort string     `json:"reasoning_effort"`
	Status          string     `json:"status"`
	Reason          string     `json:"reason,omitempty"`
	StartedAt       *time.Time `json:"started_at"`
	TotalMS         *int       `json:"total_ms"`
	OutputTokens    int64      `json:"output_tokens"`
	PreviewURL      string     `json:"preview_url,omitempty"`
}

type WallView struct {
	RoundID     int64      `json:"round_id"`
	Topic       string     `json:"topic"`
	TopicSource string     `json:"topic_source"`
	Submitter   string     `json:"submitter,omitempty"`
	Status      string     `json:"status"`
	StartedAt   time.Time  `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	PrevRoundID *int64     `json:"prev_round_id"`
	NextRoundID *int64     `json:"next_round_id"`
	Cards       []WallCard `json:"cards"`
}

type Summary struct {
	Models       int        `json:"models"`
	Rounds24h    int        `json:"rounds_24h"`
	Logic        Stat       `json:"logic"`
	Drawing      Stat       `json:"drawing"`
	Normal       int        `json:"normal"`
	Unstable     int        `json:"unstable"`
	Degraded     int        `json:"degraded"`
	QueuedTopics int        `json:"queued_topics"`
	Running      bool       `json:"running"`
	LastRoundAt  *time.Time `json:"last_round_at"`
	NextRoundAt  *time.Time `json:"next_round_at"`
}

type QueueItem struct {
	Topic     string    `json:"topic"`
	Submitter string    `json:"submitter"`
	CreatedAt time.Time `json:"created_at"`
}

type Overview struct {
	LogicEnabled    bool                 `json:"logic_enabled"`
	DrawingEnabled  bool                 `json:"drawing_enabled"`
	PrimaryKind     string               `json:"primary_kind"`
	IntervalMinutes int                  `json:"interval_minutes"`
	Thresholds      ThresholdConfig      `json:"thresholds"`
	Summary         Summary              `json:"summary"`
	Wall            *WallView            `json:"wall"`
	Models          []ModelCard          `json:"models"`
	Queue           []QueueItem          `json:"queue"`
	Recent          []RecentDrawingRound `json:"recent"`
	Timezone        string               `json:"timezone"`
	Now             time.Time            `json:"now"`
}

func (r *Runtime) groupMap(ctx context.Context) map[int64]GroupOption {
	out := map[int64]GroupOption{}
	if r.Host.Groups == nil {
		return out
	}
	if list, err := r.Host.Groups(ctx); err == nil {
		for _, g := range list {
			out[g.ID] = g
		}
	}
	return out
}

// Overview builds the public page.
func (r *Runtime) Overview(ctx context.Context) (Overview, error) {
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		return Overview{}, err
	}
	now := r.Store.now()
	since := now.Add(-24 * time.Hour)
	ov := Overview{LogicEnabled: cfg.Logic.Enabled, DrawingEnabled: cfg.Drawing.Enabled, PrimaryKind: cfg.PrimaryKind(),
		IntervalMinutes: cfg.IntervalMinutes, Thresholds: cfg.Thresholds, Timezone: timezone.Name(), Now: now, Models: []ModelCard{}, Queue: []QueueItem{}, Recent: []RecentDrawingRound{}}
	targets, err := r.enabledTargets(ctx)
	if err != nil {
		return ov, err
	}
	groups := r.groupMap(ctx)
	recent, err := r.Store.RecentResults(ctx, timelineLength, since)
	if err != nil {
		return ov, err
	}
	stats := statsSince(recent, since)
	timelines := map[historyKey]map[string][]Result{}
	for _, res := range recent {
		k := historyKey{GroupID: res.GroupID, Model: res.Model, Effort: res.ReasoningEffort}
		if timelines[k] == nil {
			timelines[k] = map[string][]Result{}
		}
		timelines[k][res.Kind] = append(timelines[k][res.Kind], res)
	}
	for _, t := range targets {
		k := keyOf(t)
		g := groups[t.GroupID]
		card := ModelCard{TargetID: t.ID, DisplayName: t.DisplayName, Model: t.Model, GroupID: t.GroupID, GroupName: g.Name, Platform: g.Platform,
			RateMultiplier: g.RateMultiplier, ReasoningEffort: t.ReasoningEffort}
		if t.ReasoningEffort != "" && r.Host.Effort != nil {
			card.SentEffort = r.Host.Effort(ctx, t.GroupID, t.Model, t.ReasoningEffort).Sent
		}
		card.Health, card.HealthRate = healthOf(cfg, stats[k])
		var ttftSum, ttftN, totalSum, totalN int
		for _, kind := range []string{KindLogic, KindDrawing} {
			list := timelines[k][kind]
			if len(list) == 0 && !((kind == KindLogic && cfg.Logic.Enabled) || (kind == KindDrawing && cfg.Drawing.Enabled)) {
				continue
			}
			if len(list) > timelineLength {
				list = list[len(list)-timelineLength:]
			}
			view := &KindView{Stat: stats[k].get(kind), Timeline: make([]Cell, 0, len(list))}
			for _, res := range list {
				view.Timeline = append(view.Timeline, Cell{ResultID: res.ID, RoundID: res.RoundID, Status: res.Status, Reason: res.Reason,
					Truncated: res.Truncated, TotalMS: res.TotalMS, CreatedAt: res.CreatedAt})
				if res.CreatedAt.Before(since) || !counted(res.Status) {
					continue
				}
				if res.FirstTokenMS != nil {
					ttftSum, ttftN = ttftSum+*res.FirstTokenMS, ttftN+1
				}
				if res.TotalMS != nil && res.Status != StatusError {
					totalSum, totalN = totalSum+*res.TotalMS, totalN+1
				}
			}
			if kind == KindLogic {
				card.Logic = view
			} else {
				card.Drawing = view
			}
		}
		if ttftN > 0 {
			v := ttftSum / ttftN
			card.AvgFirstTokenMS = &v
		}
		if totalN > 0 {
			v := totalSum / totalN
			card.AvgTotalMS = &v
		}
		switch card.Health {
		case HealthNormal:
			ov.Summary.Normal++
		case HealthUnstable:
			ov.Summary.Unstable++
		case HealthDegraded:
			ov.Summary.Degraded++
		}
		ov.Models = append(ov.Models, card)
	}
	ov.Summary.Models = len(ov.Models)
	for _, res := range recent {
		if res.CreatedAt.Before(since) {
			continue
		}
		if res.Kind == KindLogic {
			ov.Summary.Logic.add(res.Status)
		} else {
			ov.Summary.Drawing.add(res.Status)
		}
	}
	if ov.Summary.Rounds24h, err = r.Store.CountRoundsSince(ctx, since); err != nil {
		return ov, err
	}
	if ov.Summary.QueuedTopics, err = r.Store.CountTopics(ctx, TopicApproved); err != nil {
		return ov, err
	}
	latest, err := r.Store.LatestRound(ctx)
	if err != nil {
		return ov, err
	}
	if latest != nil {
		started := latest.StartedAt
		ov.Summary.LastRoundAt = &started
		ov.Summary.Running = latest.Status == RoundRunning
	}
	ov.Summary.NextRoundAt = nextRoundAt(cfg, now, latest)
	if round, err := r.Store.LatestDrawingRound(ctx); err != nil {
		return ov, err
	} else if round != nil {
		if ov.Wall, err = r.buildWall(ctx, *round, targets, groups); err != nil {
			return ov, err
		}
	}
	if ov.Recent, err = r.Store.RecentDrawingRounds(ctx, recentDrawingLimit); err != nil {
		return ov, err
	}
	queued, err := r.Store.QueuedTopics(ctx, 5)
	if err != nil {
		return ov, err
	}
	for _, t := range queued {
		ov.Queue = append(ov.Queue, QueueItem{Topic: t.Topic, Submitter: MaskName(displayName(t.Username, t.Email)), CreatedAt: t.CreatedAt})
	}
	return ov, nil
}

// recentDrawingLimit bounds the public "recently drawn" list.
const recentDrawingLimit = 30

// Wall is the drawing wall of one past or running round.
func (r *Runtime) Wall(ctx context.Context, roundID int64) (*WallView, error) {
	round, err := r.Store.DrawingRound(ctx, roundID)
	if err != nil {
		return nil, err
	}
	targets, err := r.enabledTargets(ctx)
	if err != nil {
		return nil, err
	}
	return r.buildWall(ctx, round, targets, r.groupMap(ctx))
}

func (r *Runtime) buildWall(ctx context.Context, round Round, targets []Target, groups map[int64]GroupOption) (*WallView, error) {
	results, err := r.Store.RoundResults(ctx, round.ID, KindDrawing)
	if err != nil {
		return nil, err
	}
	view := &WallView{RoundID: round.ID, Topic: round.Topic, TopicSource: round.TopicSource, Status: round.Status, StartedAt: round.StartedAt, FinishedAt: round.FinishedAt, Cards: []WallCard{}}
	if view.PrevRoundID, view.NextRoundID, err = r.Store.AdjacentDrawingRounds(ctx, round.ID); err != nil {
		return nil, err
	}
	if round.TopicID != nil {
		if t, err := r.Store.Topic(ctx, *round.TopicID); err == nil && t != nil {
			view.Submitter = MaskName(displayName(t.Username, t.Email))
		}
	}
	order := map[int64]int{}
	for i, t := range targets {
		order[t.ID] = i
	}
	sortByTarget(results, order)
	for _, res := range results {
		if res.Status == StatusSkipped {
			continue
		}
		g := groups[res.GroupID]
		name := res.GroupName
		if g.Name != "" {
			name = g.Name
		}
		view.Cards = append(view.Cards, WallCard{ResultID: res.ID, TargetID: res.TargetID, DisplayName: res.DisplayName, Model: res.Model, GroupName: name,
			Platform: g.Platform, RateMultiplier: g.RateMultiplier, ReasoningEffort: res.ReasoningEffort, Status: res.Status, Reason: res.Reason,
			StartedAt: res.StartedAt, TotalMS: res.TotalMS, OutputTokens: res.OutputTokens, PreviewURL: previewFor(ctx, r, res)})
	}
	return view, nil
}

// previewFor signs a preview only for finished drawings with a reply.
func previewFor(ctx context.Context, r *Runtime, res Result) string {
	if res.Status != StatusPass && res.Status != StatusFail {
		return ""
	}
	return r.PreviewURL(ctx, res.ID)
}

// sortByTarget orders results by current target order; results of removed
// targets follow in id order.
func sortByTarget(results []Result, order map[int64]int) {
	pos := func(r Result) int {
		if p, ok := order[r.TargetID]; ok {
			return p
		}
		return len(order) + int(r.ID%1_000_000)
	}
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && pos(results[j]) < pos(results[j-1]); j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}

// ResultDetail is a result with the shared round material and every reply.
type ResultDetail struct {
	Result    Result     `json:"result"`
	Question  string     `json:"question,omitempty"`
	Answers   []string   `json:"answers,omitempty"`
	Topic     string     `json:"topic,omitempty"`
	Responses []Response `json:"responses"`
	// PreviewHTML is the renderable drawing document; it is only served by
	// the signed preview URL, never inside API JSON.
	PreviewHTML string `json:"-"`
	PreviewURL  string `json:"preview_url,omitempty"`
}

func (r *Runtime) Detail(ctx context.Context, id int64) (ResultDetail, error) {
	res, err := r.Store.Result(ctx, id)
	if err != nil {
		return ResultDetail{}, err
	}
	round, err := r.Store.Round(ctx, res.RoundID)
	if err != nil {
		return ResultDetail{}, err
	}
	responses, err := r.Store.Responses(ctx, id)
	if err != nil {
		return ResultDetail{}, err
	}
	d := ResultDetail{Result: res, Responses: responses}
	if res.Kind == KindLogic {
		d.Question, d.Answers = round.Question, round.Answers
	} else {
		d.Topic = round.Topic
		if len(responses) > 0 {
			d.PreviewHTML = PreviewHTML(responses[len(responses)-1].Content)
		}
	}
	return d, nil
}

// MySubmissions is the submission box state for one user.
type MySubmissions struct {
	Enabled   bool    `json:"enabled"`
	Quota     int     `json:"quota"`
	Used      int     `json:"used"`
	MaxLength int     `json:"max_length"`
	Day       string  `json:"day"`
	Items     []Topic `json:"items"`
}

func quotaDay(now time.Time) string { return now.In(timezone.Location()).Format("2006-01-02") }

func (r *Runtime) Submissions(ctx context.Context, userID int64) (MySubmissions, error) {
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		return MySubmissions{}, err
	}
	day := quotaDay(r.Store.now())
	out := MySubmissions{Enabled: cfg.Submissions.Enabled && cfg.Drawing.Enabled, Quota: cfg.Submissions.DailyQuota, MaxLength: cfg.Submissions.MaxLength, Day: day}
	if out.Items, err = r.Store.UserTopics(ctx, userID, day); err != nil {
		return out, err
	}
	for i := range out.Items {
		// Users see their own text and verdict, not internal errors.
		out.Items[i].LastError, out.Items[i].Username, out.Items[i].Email = "", "", ""
	}
	out.Used, err = r.Store.UsedQuota(ctx, userID, day)
	return out, err
}

func (r *Runtime) Submit(ctx context.Context, userID int64, raw string) (Topic, error) {
	cfg, _, err := r.Store.Config(ctx)
	if err != nil {
		return Topic{}, err
	}
	if !cfg.Submissions.Enabled || !cfg.Drawing.Enabled {
		return Topic{}, errSubmissionsClosed
	}
	topic, err := CleanTopic(raw, cfg.Submissions.MaxLength)
	if err != nil {
		return Topic{}, err
	}
	t, err := r.Store.SubmitTopic(ctx, userID, topic, quotaDay(r.Store.now()), cfg.Submissions.DailyQuota)
	t.LastError, t.Username, t.Email = "", "", ""
	return t, err
}

// nextRoundAt is when the scheduler will start the next round: the current
// slot if no round has claimed it yet, otherwise the following slot.
func nextRoundAt(cfg Config, now time.Time, latest *Round) *time.Time {
	if !cfg.Logic.Enabled && !cfg.Drawing.Enabled {
		return nil
	}
	next := slotStart(now, cfg.IntervalMinutes)
	if latest != nil && !latest.StartedAt.Before(next) {
		next = next.Add(time.Duration(cfg.IntervalMinutes) * time.Minute)
	}
	if next.Before(now) {
		next = now
	}
	return &next
}
