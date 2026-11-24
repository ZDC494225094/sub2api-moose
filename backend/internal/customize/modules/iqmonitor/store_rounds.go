package iqmonitor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const roundColumns = `id, slot_key, trigger_kind, node_id, status, logic_enabled, drawing_enabled, question_id, question, answers, topic, topic_source, topic_id, started_at, heartbeat_at, finished_at`

func scanRound(row interface{ Scan(...any) error }) (Round, error) {
	var r Round
	var qid, tid sql.NullInt64
	var answers string
	var finished sql.NullTime
	err := row.Scan(&r.ID, &r.SlotKey, &r.Trigger, &r.NodeID, &r.Status, &r.LogicEnabled, &r.DrawingEnabled, &qid, &r.Question, &answers, &r.Topic, &r.TopicSource, &tid, &r.StartedAt, &r.HeartbeatAt, &finished)
	if err != nil {
		return r, err
	}
	r.QuestionID, r.TopicID = nullInt64Ptr(qid), nullInt64Ptr(tid)
	r.Answers = decodeAnswers(answers)
	r.FinishedAt = nullTimePtr(finished)
	r.StartedAt, r.HeartbeatAt = r.StartedAt.UTC(), r.HeartbeatAt.UTC()
	return r, nil
}

// ClaimRound inserts the round for a slot. Only one node's insert succeeds
// on the unique slot_key; the others get ok=false and skip the slot.
func (s *Store) ClaimRound(ctx context.Context, r Round) (int64, bool, error) {
	if err := s.ready(); err != nil {
		return 0, false, err
	}
	now := s.now()
	var id int64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO iq_monitor_rounds (slot_key, trigger_kind, node_id, status, logic_enabled, drawing_enabled, started_at, heartbeat_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
ON CONFLICT (slot_key) DO NOTHING RETURNING id`, r.SlotKey, r.Trigger, r.NodeID, RoundRunning, r.LogicEnabled, r.DrawingEnabled, now).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("iq-monitor: claim round: %w", err)
	}
	return id, true, nil
}

// SetRoundMaterial records the shared question and topic of a claimed round.
func (s *Store) SetRoundMaterial(ctx context.Context, r Round) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_rounds SET logic_enabled = $1, drawing_enabled = $2, question_id = $3, question = $4, answers = $5, topic = $6, topic_source = $7, topic_id = $8 WHERE id = $9`,
		r.LogicEnabled, r.DrawingEnabled, r.QuestionID, r.Question, encodeAnswers(r.Answers), r.Topic, r.TopicSource, r.TopicID, r.ID)
	return err
}

func (s *Store) Heartbeat(ctx context.Context, roundID int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_rounds SET heartbeat_at = $1 WHERE id = $2 AND status = $3`, s.now(), roundID, RoundRunning)
	return err
}

// FinishRound closes a round; interrupted when the node stopped it.
func (s *Store) FinishRound(ctx context.Context, roundID int64, status string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_rounds SET status = $1, finished_at = $2 WHERE id = $3 AND status = $4`, status, s.now(), roundID, RoundRunning)
	return err
}

// RunningRound reports the newest running round with a fresh heartbeat.
func (s *Store) RunningRound(ctx context.Context, staleBefore time.Time) (*Round, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	r, err := scanRound(s.DB.QueryRowContext(ctx, `SELECT `+roundColumns+` FROM iq_monitor_rounds WHERE status = $1 AND heartbeat_at >= $2 ORDER BY id DESC LIMIT 1`, RoundRunning, staleBefore.UTC()))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) Round(ctx context.Context, id int64) (Round, error) {
	if err := s.ready(); err != nil {
		return Round{}, err
	}
	r, err := scanRound(s.DB.QueryRowContext(ctx, `SELECT `+roundColumns+` FROM iq_monitor_rounds WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Round{}, ErrNotFound
	}
	return r, err
}

// LatestRound is the newest round, running or finished.
func (s *Store) LatestRound(ctx context.Context) (*Round, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	r, err := scanRound(s.DB.QueryRowContext(ctx, `SELECT `+roundColumns+` FROM iq_monitor_rounds ORDER BY id DESC LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// LatestDrawingRound is the newest round that ran the drawing test, for the
// public wall.
func (s *Store) LatestDrawingRound(ctx context.Context) (*Round, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	r, err := scanRound(s.DB.QueryRowContext(ctx, `SELECT `+roundColumns+` FROM iq_monitor_rounds WHERE drawing_enabled = true AND topic <> '' ORDER BY id DESC LIMIT 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

const drawingRoundFilter = `drawing_enabled = true AND topic <> ''`

// DrawingRound loads one round that ran the drawing test.
func (s *Store) DrawingRound(ctx context.Context, id int64) (Round, error) {
	r, err := s.Round(ctx, id)
	if err != nil {
		return Round{}, err
	}
	if !r.DrawingEnabled || r.Topic == "" {
		return Round{}, ErrNotFound
	}
	return r, nil
}

// AdjacentDrawingRounds returns the drawing rounds just before and after id,
// nil where there is none.
func (s *Store) AdjacentDrawingRounds(ctx context.Context, id int64) (prev, next *int64, err error) {
	if err := s.ready(); err != nil {
		return nil, nil, err
	}
	one := func(q string) (*int64, error) {
		var v int64
		err := s.DB.QueryRowContext(ctx, q, id).Scan(&v)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &v, nil
	}
	if prev, err = one(`SELECT id FROM iq_monitor_rounds WHERE ` + drawingRoundFilter + ` AND id < $1 ORDER BY id DESC LIMIT 1`); err != nil {
		return nil, nil, err
	}
	next, err = one(`SELECT id FROM iq_monitor_rounds WHERE ` + drawingRoundFilter + ` AND id > $1 ORDER BY id ASC LIMIT 1`)
	return prev, next, err
}

// RecentDrawingRound is one entry of the public "recently drawn" list.
type RecentDrawingRound struct {
	RoundID     int64     `json:"round_id"`
	Topic       string    `json:"topic"`
	TopicSource string    `json:"topic_source"`
	Submitter   string    `json:"submitter,omitempty"`
	StartedAt   time.Time `json:"started_at"`
}

// RecentDrawingRounds lists the newest drawing rounds with their submitter.
func (s *Store) RecentDrawingRounds(ctx context.Context, limit int) ([]RecentDrawingRound, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT r.id, r.topic, r.topic_source, r.started_at, t.id IS NOT NULL, COALESCE(u.username, ''), COALESCE(u.email, '')
FROM iq_monitor_rounds r
LEFT JOIN iq_monitor_topics t ON t.id = r.topic_id
LEFT JOIN users u ON u.id = t.user_id
WHERE r.drawing_enabled = true AND r.topic <> ''
ORDER BY r.id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: recent drawing rounds: %w", err)
	}
	defer rows.Close()
	out := []RecentDrawingRound{}
	for rows.Next() {
		var item RecentDrawingRound
		var hasTopic bool
		var username, email string
		if err := rows.Scan(&item.RoundID, &item.Topic, &item.TopicSource, &item.StartedAt, &hasTopic, &username, &email); err != nil {
			return nil, err
		}
		if hasTopic {
			item.Submitter = MaskName(displayName(username, email))
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// InterruptStale closes running rounds whose node stopped heart-beating and
// marks their unfinished results interrupted. Returns the rounds closed.
func (s *Store) InterruptStale(ctx context.Context, staleBefore time.Time) (int, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM iq_monitor_rounds WHERE status = $1 AND heartbeat_at < $2`, RoundRunning, staleBefore.UTC())
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	closed := 0
	for _, id := range ids {
		if err := s.InterruptRound(ctx, id, "执行节点已停止，请求未完成"); err != nil {
			return closed, err
		}
		closed++
	}
	return closed, nil
}

// InterruptRound marks a round and its unfinished results interrupted.
func (s *Store) InterruptRound(ctx context.Context, roundID int64, reason string) error {
	now := s.now()
	if _, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_results SET status = $1, reason = $2, finished_at = $3 WHERE round_id = $4 AND status IN ($5, $6)`,
		StatusInterrupted, reason, now, roundID, StatusPending, StatusRunning); err != nil {
		return fmt.Errorf("iq-monitor: interrupt results: %w", err)
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_rounds SET status = $1, finished_at = $2 WHERE id = $3 AND status = $4`, RoundInterrupted, now, roundID, RoundRunning)
	return err
}

const resultColumns = `id, round_id, target_id, kind, group_id, group_name, model, display_name, reasoning_effort, sent_effort, status, reason, attempts, request_count, input_tokens, output_tokens, first_token_ms, total_ms, truncated, started_at, finished_at, created_at`

func scanResult(row interface{ Scan(...any) error }) (Result, error) {
	var r Result
	var ttft, total sql.NullInt64
	var started, finished sql.NullTime
	err := row.Scan(&r.ID, &r.RoundID, &r.TargetID, &r.Kind, &r.GroupID, &r.GroupName, &r.Model, &r.DisplayName, &r.ReasoningEffort, &r.SentEffort, &r.Status, &r.Reason,
		&r.Attempts, &r.RequestCount, &r.InputTokens, &r.OutputTokens, &ttft, &total, &r.Truncated, &started, &finished, &r.CreatedAt)
	if err != nil {
		return r, err
	}
	r.FirstTokenMS, r.TotalMS = nullIntPtr(ttft), nullIntPtr(total)
	r.StartedAt, r.FinishedAt = nullTimePtr(started), nullTimePtr(finished)
	r.CreatedAt = r.CreatedAt.UTC()
	return r, nil
}

func (s *Store) CreateResult(ctx context.Context, r Result) (int64, error) {
	var id int64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO iq_monitor_results (round_id, target_id, kind, group_id, group_name, model, display_name, reasoning_effort, sent_effort, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`,
		r.RoundID, r.TargetID, r.Kind, r.GroupID, r.GroupName, r.Model, r.DisplayName, r.ReasoningEffort, r.SentEffort, StatusPending, s.now()).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("iq-monitor: create result: %w", err)
	}
	return id, nil
}

func (s *Store) StartResult(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_results SET status = $1, started_at = $2 WHERE id = $3 AND status = $4`, StatusRunning, s.now(), id, StatusPending)
	return err
}

// SaveResult stores the final verdict. A result already marked interrupted
// (e.g. by another node after a missed heartbeat) is left alone.
func (s *Store) SaveResult(ctx context.Context, r Result) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_results SET status = $1, reason = $2, attempts = $3, request_count = $4, input_tokens = $5, output_tokens = $6,
first_token_ms = $7, total_ms = $8, truncated = $9, finished_at = $10 WHERE id = $11 AND status IN ($12, $13)`,
		r.Status, truncateRunes(r.Reason, 480), r.Attempts, r.RequestCount, r.InputTokens, r.OutputTokens, intArg(r.FirstTokenMS), intArg(r.TotalMS), r.Truncated, s.now(), r.ID, StatusPending, StatusRunning)
	return err
}

func (s *Store) AddResponse(ctx context.Context, resp Response) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO iq_monitor_responses (result_id, attempt, content, extracted, correct, finish_reason, truncated, error, input_tokens, output_tokens, first_token_ms, total_ms, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		resp.ResultID, resp.Attempt, resp.Content, truncateRunes(resp.Extracted, 480), resp.Correct, truncateRunes(resp.FinishReason, 30), resp.Truncated, truncateRunes(resp.Error, 480),
		resp.InputTokens, resp.OutputTokens, intArg(resp.FirstTokenMS), intArg(resp.TotalMS), s.now())
	return err
}

func (s *Store) Result(ctx context.Context, id int64) (Result, error) {
	if err := s.ready(); err != nil {
		return Result{}, err
	}
	r, err := scanResult(s.DB.QueryRowContext(ctx, `SELECT `+resultColumns+` FROM iq_monitor_results WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Result{}, ErrNotFound
	}
	return r, err
}

func (s *Store) Responses(ctx context.Context, resultID int64) ([]Response, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, result_id, attempt, content, extracted, correct, finish_reason, truncated, error, input_tokens, output_tokens, first_token_ms, total_ms, created_at
FROM iq_monitor_responses WHERE result_id = $1 ORDER BY attempt, id`, resultID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Response{}
	for rows.Next() {
		var r Response
		var ttft, total sql.NullInt64
		if err := rows.Scan(&r.ID, &r.ResultID, &r.Attempt, &r.Content, &r.Extracted, &r.Correct, &r.FinishReason, &r.Truncated, &r.Error, &r.InputTokens, &r.OutputTokens, &ttft, &total, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.FirstTokenMS, r.TotalMS = nullIntPtr(ttft), nullIntPtr(total)
		out = append(out, r)
	}
	return out, rows.Err()
}

// RoundResults lists a round's results of one kind (all kinds when empty).
func (s *Store) RoundResults(ctx context.Context, roundID int64, kind string) ([]Result, error) {
	q := `SELECT ` + resultColumns + ` FROM iq_monitor_results WHERE round_id = $1`
	args := []any{roundID}
	if kind != "" {
		q += ` AND kind = $2`
		args = append(args, kind)
	}
	return s.queryResults(ctx, q+` ORDER BY id`, args...)
}

func (s *Store) queryResults(ctx context.Context, q string, args ...any) ([]Result, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: query results: %w", err)
	}
	defer rows.Close()
	out := []Result{}
	for rows.Next() {
		r, err := scanResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// RecentResults returns, per history key and kind, the newest `limit`
// finished-or-running results, plus everything created since `since` so
// 24h statistics are complete even past the timeline length.
func (s *Store) RecentResults(ctx context.Context, limit int, since time.Time) ([]Result, error) {
	return s.queryResults(ctx, `SELECT `+resultColumns+` FROM (
SELECT r.*, ROW_NUMBER() OVER (PARTITION BY group_id, model, reasoning_effort, kind ORDER BY id DESC) AS rn FROM iq_monitor_results r
) x WHERE rn <= $1 OR created_at >= $2 ORDER BY id`, limit, since.UTC())
}

// RecentProblems lists the newest results that did not pass, so admins can
// read gateway errors that never reached the usage log.
func (s *Store) RecentProblems(ctx context.Context, limit int) ([]Result, error) {
	return s.queryResults(ctx, `SELECT `+resultColumns+` FROM iq_monitor_results WHERE status IN ($1, $2, $3, $4) ORDER BY id DESC LIMIT $5`,
		StatusError, StatusFail, StatusSkipped, StatusInterrupted, limit)
}

// ResultsSince lists results created at or after since (statistics).
func (s *Store) ResultsSince(ctx context.Context, since time.Time) ([]Result, error) {
	return s.queryResults(ctx, `SELECT `+resultColumns+` FROM iq_monitor_results WHERE created_at >= $1 ORDER BY id`, since.UTC())
}

func (s *Store) CountRoundsSince(ctx context.Context, since time.Time) (int, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM iq_monitor_rounds WHERE started_at >= $1`, since.UTC()).Scan(&n)
	return n, err
}

// Prune deletes rounds (with their results and responses) and spent topic
// submissions older than the cutoff. Queued submissions are kept.
func (s *Store) Prune(ctx context.Context, before time.Time) error {
	if err := s.ready(); err != nil {
		return err
	}
	before = before.UTC()
	for _, stmt := range []string{
		`DELETE FROM iq_monitor_responses WHERE result_id IN (SELECT r.id FROM iq_monitor_results r JOIN iq_monitor_rounds o ON o.id = r.round_id WHERE o.started_at < $1 AND o.status <> 'running')`,
		`DELETE FROM iq_monitor_results WHERE round_id IN (SELECT id FROM iq_monitor_rounds WHERE started_at < $1 AND status <> 'running')`,
		`DELETE FROM iq_monitor_rounds WHERE started_at < $1 AND status <> 'running'`,
		`DELETE FROM iq_monitor_topics WHERE created_at < $1 AND status IN ('used', 'rejected', 'failed')`,
	} {
		if _, err := s.DB.ExecContext(ctx, stmt, before); err != nil {
			return fmt.Errorf("iq-monitor: prune: %w", err)
		}
	}
	return nil
}

// ClearHistory deletes the history of one group+model (every effort) or,
// with all, of every model. Rounds that are still running are kept so the
// executing node can finish; topic submissions are never touched.
func (s *Store) ClearHistory(ctx context.Context, all bool, groupID int64, model string) (int64, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	var deleted int64
	if all {
		for _, stmt := range []string{
			`DELETE FROM iq_monitor_responses WHERE result_id IN (SELECT r.id FROM iq_monitor_results r JOIN iq_monitor_rounds o ON o.id = r.round_id WHERE o.status <> 'running')`,
			`DELETE FROM iq_monitor_results WHERE round_id IN (SELECT id FROM iq_monitor_rounds WHERE status <> 'running')`,
			`DELETE FROM iq_monitor_rounds WHERE status <> 'running'`,
			`DELETE FROM iq_monitor_alert_state`,
		} {
			res, err := tx.ExecContext(ctx, stmt)
			if err != nil {
				return 0, fmt.Errorf("iq-monitor: clear history: %w", err)
			}
			if strings.Contains(stmt, "iq_monitor_results WHERE") {
				deleted, _ = res.RowsAffected()
			}
		}
	} else {
		if _, err := tx.ExecContext(ctx, `DELETE FROM iq_monitor_responses WHERE result_id IN (SELECT r.id FROM iq_monitor_results r JOIN iq_monitor_rounds o ON o.id = r.round_id WHERE r.group_id = $1 AND r.model = $2 AND o.status <> 'running')`, groupID, model); err != nil {
			return 0, fmt.Errorf("iq-monitor: clear history: %w", err)
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM iq_monitor_results WHERE group_id = $1 AND model = $2 AND round_id IN (SELECT id FROM iq_monitor_rounds WHERE status <> 'running')`, groupID, model)
		if err != nil {
			return 0, fmt.Errorf("iq-monitor: clear history: %w", err)
		}
		deleted, _ = res.RowsAffected()
		if _, err := tx.ExecContext(ctx, `DELETE FROM iq_monitor_alert_state WHERE group_id = $1 AND model = $2`, groupID, model); err != nil {
			return 0, err
		}
	}
	return deleted, tx.Commit()
}

// TransitionAlert records a model's latest health. It returns true exactly
// once when the model enters degraded from any other state; the
// conditional update makes concurrent evaluators agree on a single winner.
func (s *Store) TransitionAlert(ctx context.Context, key historyKey, health string) (bool, error) {
	now := s.now()
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO iq_monitor_alert_state (group_id, model, reasoning_effort, status, updated_at) VALUES ($1, $2, $3, '', $4)
ON CONFLICT (group_id, model, reasoning_effort) DO NOTHING`, key.GroupID, key.Model, key.Effort, now); err != nil {
		return false, err
	}
	if health == HealthNoData {
		return false, nil
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_alert_state SET status = $1, updated_at = $2 WHERE group_id = $3 AND model = $4 AND reasoning_effort = $5 AND status <> $1`,
		health, now, key.GroupID, key.Model, key.Effort)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1 && health == HealthDegraded, nil
}
