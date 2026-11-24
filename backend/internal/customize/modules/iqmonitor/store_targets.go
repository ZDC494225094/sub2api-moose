package iqmonitor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

const targetColumns = `id, group_id, model, display_name, reasoning_effort, api_key_id, enabled, sort_order, created_at, updated_at`

func scanTarget(row interface{ Scan(...any) error }) (Target, error) {
	var t Target
	err := row.Scan(&t.ID, &t.GroupID, &t.Model, &t.DisplayName, &t.ReasoningEffort, &t.APIKeyID, &t.Enabled, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	return t, err
}

// Targets lists every target in display order.
func (s *Store) Targets(ctx context.Context) ([]Target, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+targetColumns+` FROM iq_monitor_targets ORDER BY sort_order, id`)
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: list targets: %w", err)
	}
	defer rows.Close()
	out := []Target{}
	for rows.Next() {
		t, err := scanTarget(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) Target(ctx context.Context, id int64) (Target, error) {
	if err := s.ready(); err != nil {
		return Target{}, err
	}
	t, err := scanTarget(s.DB.QueryRowContext(ctx, `SELECT `+targetColumns+` FROM iq_monitor_targets WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Target{}, ErrNotFound
	}
	return t, err
}

// CreateTarget appends a target; the count limit is checked in the same
// statement so concurrent saves cannot exceed MaxTargets.
func (s *Store) CreateTarget(ctx context.Context, t Target) (int64, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	now := s.now()
	var id int64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO iq_monitor_targets (group_id, model, display_name, reasoning_effort, api_key_id, enabled, sort_order, created_at, updated_at)
SELECT $1, $2, $3, $4, $5, $6, COALESCE((SELECT MAX(sort_order) FROM iq_monitor_targets), 0) + 1, $7, $7
WHERE (SELECT COUNT(*) FROM iq_monitor_targets) < $8
RETURNING id`, t.GroupID, t.Model, t.DisplayName, t.ReasoningEffort, t.APIKeyID, t.Enabled, now, MaxTargets).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("最多只能配置 %d 个被测目标", MaxTargets)
	}
	if err != nil {
		return 0, fmt.Errorf("iq-monitor: create target: %w", err)
	}
	return id, nil
}

func (s *Store) UpdateTarget(ctx context.Context, t Target) error {
	if err := s.ready(); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_targets SET group_id = $1, model = $2, display_name = $3, reasoning_effort = $4, api_key_id = $5, enabled = $6, updated_at = $7 WHERE id = $8`,
		t.GroupID, t.Model, t.DisplayName, t.ReasoningEffort, t.APIKeyID, t.Enabled, s.now(), t.ID)
	if err != nil {
		return fmt.Errorf("iq-monitor: update target: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteTarget removes the target only; its history stays under the
// (group, model, effort) key until retention or an explicit clear.
func (s *Store) DeleteTarget(ctx context.Context, id int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `DELETE FROM iq_monitor_targets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("iq-monitor: delete target: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderTargets assigns sort_order by position; ids must be exactly the
// current set of targets.
func (s *Store) ReorderTargets(ctx context.Context, ids []int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	current, err := s.Targets(ctx)
	if err != nil {
		return err
	}
	if len(current) != len(ids) {
		return errors.New("排序列表与当前目标不一致，请刷新后重试")
	}
	known := map[int64]bool{}
	for _, t := range current {
		known[t.ID] = true
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if !known[id] || seen[id] {
			return errors.New("排序列表与当前目标不一致，请刷新后重试")
		}
		seen[id] = true
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE iq_monitor_targets SET sort_order = $1 WHERE id = $2`, i+1, id); err != nil {
			return fmt.Errorf("iq-monitor: reorder targets: %w", err)
		}
	}
	return tx.Commit()
}

const questionColumns = `id, question, answers, enabled, last_used_at, created_at, updated_at`

func scanQuestion(row interface{ Scan(...any) error }) (Question, error) {
	var q Question
	var answers string
	var used sql.NullTime
	if err := row.Scan(&q.ID, &q.Question, &answers, &q.Enabled, &used, &q.CreatedAt, &q.UpdatedAt); err != nil {
		return q, err
	}
	q.Answers = decodeAnswers(answers)
	q.LastUsedAt = nullTimePtr(used)
	return q, nil
}

func (s *Store) Questions(ctx context.Context) ([]Question, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+questionColumns+` FROM iq_monitor_questions ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: list questions: %w", err)
	}
	defer rows.Close()
	out := []Question{}
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (s *Store) CountQuestions(ctx context.Context) (int, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM iq_monitor_questions`).Scan(&n)
	return n, err
}

func (s *Store) SaveQuestion(ctx context.Context, q Question) (int64, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	now := s.now()
	if q.ID == 0 {
		var id int64
		err := s.DB.QueryRowContext(ctx, `INSERT INTO iq_monitor_questions (question, answers, enabled, created_at, updated_at) VALUES ($1, $2, $3, $4, $4) RETURNING id`,
			q.Question, encodeAnswers(q.Answers), q.Enabled, now).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("iq-monitor: create question: %w", err)
		}
		return id, nil
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_questions SET question = $1, answers = $2, enabled = $3, updated_at = $4 WHERE id = $5`,
		q.Question, encodeAnswers(q.Answers), q.Enabled, now, q.ID)
	if err != nil {
		return 0, fmt.Errorf("iq-monitor: update question: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, ErrNotFound
	}
	return q.ID, nil
}

func (s *Store) DeleteQuestion(ctx context.Context, id int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `DELETE FROM iq_monitor_questions WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("iq-monitor: delete question: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// PickQuestion chooses an enabled question for a round, preferring those
// unused for longest (never-used first), with ties broken randomly by the
// caller-supplied index.
func (s *Store) PickQuestion(ctx context.Context, pick func(n int) int) (*Question, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+questionColumns+` FROM iq_monitor_questions WHERE enabled = true ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: pick question: %w", err)
	}
	defer rows.Close()
	var all []Question
	for rows.Next() {
		q, err := scanQuestion(rows)
		if err != nil {
			return nil, err
		}
		if len(q.Answers) > 0 {
			all = append(all, q)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(all) == 0 {
		return nil, nil
	}
	// Candidates are the least recently used half (at least one), so the
	// same question does not come back in consecutive rounds.
	sortQuestionsByUse(all)
	n := (len(all) + 1) / 2
	q := all[pick(n)]
	return &q, nil
}

func sortQuestionsByUse(qs []Question) {
	less := func(a, b Question) bool {
		switch {
		case a.LastUsedAt == nil && b.LastUsedAt == nil:
			return a.ID < b.ID
		case a.LastUsedAt == nil:
			return true
		case b.LastUsedAt == nil:
			return false
		}
		return a.LastUsedAt.Before(*b.LastUsedAt)
	}
	for i := 1; i < len(qs); i++ {
		for j := i; j > 0 && less(qs[j], qs[j-1]); j-- {
			qs[j], qs[j-1] = qs[j-1], qs[j]
		}
	}
}

func (s *Store) MarkQuestionUsed(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_questions SET last_used_at = $1 WHERE id = $2`, s.now(), id)
	return err
}

type Viewer struct {
	UserID   int64  `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Note     string `json:"note"`
}

func (s *Store) Viewers(ctx context.Context) ([]Viewer, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT v.user_id, COALESCE(u.email, ''), COALESCE(u.username, ''), v.note
FROM iq_monitor_viewers v LEFT JOIN users u ON u.id = v.user_id ORDER BY v.created_at DESC, v.user_id DESC`)
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: list viewers: %w", err)
	}
	defer rows.Close()
	out := []Viewer{}
	for rows.Next() {
		var v Viewer
		if err := rows.Scan(&v.UserID, &v.Email, &v.Username, &v.Note); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) AddViewer(ctx context.Context, userID int64, note string) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO iq_monitor_viewers (user_id, note, created_at) VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE SET note = excluded.note`, userID, note, s.now())
	return err
}

func (s *Store) RemoveViewer(ctx context.Context, userID int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM iq_monitor_viewers WHERE user_id = $1`, userID)
	return err
}

func (s *Store) IsViewer(ctx context.Context, userID int64) (bool, error) {
	if err := s.ready(); err != nil {
		return false, err
	}
	var one int
	err := s.DB.QueryRowContext(ctx, `SELECT 1 FROM iq_monitor_viewers WHERE user_id = $1`, userID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
