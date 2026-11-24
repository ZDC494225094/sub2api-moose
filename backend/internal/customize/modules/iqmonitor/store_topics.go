package iqmonitor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const MaxReviewAttempts = 5

const topicColumns = `t.id, t.user_id, COALESCE(u.username, ''), COALESCE(u.email, ''), t.topic, t.status, t.reject_reason, t.attempts, t.last_error, t.quota_day, t.round_id, t.created_at, t.reviewed_at, t.used_at`

func scanTopic(row interface{ Scan(...any) error }) (Topic, error) {
	var t Topic
	var round sql.NullInt64
	var reviewed, used sql.NullTime
	err := row.Scan(&t.ID, &t.UserID, &t.Username, &t.Email, &t.Topic, &t.Status, &t.RejectReason, &t.Attempts, &t.LastError, &t.QuotaDay, &round, &t.CreatedAt, &reviewed, &used)
	if err != nil {
		return t, err
	}
	t.RoundID = nullInt64Ptr(round)
	t.ReviewedAt, t.UsedAt = nullTimePtr(reviewed), nullTimePtr(used)
	return t, nil
}

func (s *Store) queryTopics(ctx context.Context, where string, args ...any) ([]Topic, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+topicColumns+` FROM iq_monitor_topics t LEFT JOIN users u ON u.id = t.user_id `+where, args...)
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: list topics: %w", err)
	}
	defer rows.Close()
	out := []Topic{}
	for rows.Next() {
		t, err := scanTopic(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UserTopics lists one user's submissions of a quota day.
func (s *Store) UserTopics(ctx context.Context, userID int64, day string) ([]Topic, error) {
	return s.queryTopics(ctx, `WHERE t.user_id = $1 AND t.quota_day = $2 ORDER BY t.id DESC`, userID, day)
}

// AdminTopics lists submissions for the admin console, newest first.
func (s *Store) AdminTopics(ctx context.Context, status string, limit, offset int) ([]Topic, int, error) {
	where, args := "", []any{}
	if status != "" {
		where, args = `WHERE t.status = $1`, append(args, status)
	}
	var total int
	countQ := `SELECT COUNT(*) FROM iq_monitor_topics t ` + where
	if err := s.DB.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	n := len(args)
	items, err := s.queryTopics(ctx, where+fmt.Sprintf(` ORDER BY t.id DESC LIMIT $%d OFFSET $%d`, n+1, n+2), append(args, limit, offset)...)
	return items, total, err
}

// QueuedTopics returns approved, unused submissions oldest first.
func (s *Store) QueuedTopics(ctx context.Context, limit int) ([]Topic, error) {
	return s.queryTopics(ctx, `WHERE t.status = $1 ORDER BY t.id LIMIT $2`, TopicApproved, limit)
}

func (s *Store) CountTopics(ctx context.Context, status string) (int, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM iq_monitor_topics WHERE status = $1`, status).Scan(&n)
	return n, err
}

var ErrQuotaExceeded = errors.New("今日投稿次数已用完")

// SubmitTopic inserts a pending submission when the user has quota left
// today. Failed reviews do not count. The quota check and insert happen in
// one statement so parallel submissions cannot exceed the quota.
func (s *Store) SubmitTopic(ctx context.Context, userID int64, topic, day string, quota int) (Topic, error) {
	if err := s.ready(); err != nil {
		return Topic{}, err
	}
	now := s.now()
	var id int64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO iq_monitor_topics (user_id, topic, status, quota_day, created_at, next_attempt_at)
SELECT $1, $2, $3, $4, $5, $5
WHERE (SELECT COUNT(*) FROM iq_monitor_topics WHERE user_id = $1 AND quota_day = $4 AND status <> $6) < $7
RETURNING id`, userID, topic, TopicPending, day, now, TopicFailed, quota).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Topic{}, ErrQuotaExceeded
	}
	if err != nil {
		return Topic{}, fmt.Errorf("iq-monitor: submit topic: %w", err)
	}
	items, err := s.queryTopics(ctx, `WHERE t.id = $1`, id)
	if err != nil || len(items) == 0 {
		return Topic{ID: id, UserID: userID, Topic: topic, Status: TopicPending, QuotaDay: day, CreatedAt: now}, err
	}
	return items[0], nil
}

func (s *Store) UsedQuota(ctx context.Context, userID int64, day string) (int, error) {
	if err := s.ready(); err != nil {
		return 0, err
	}
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM iq_monitor_topics WHERE user_id = $1 AND quota_day = $2 AND status <> $3`, userID, day, TopicFailed).Scan(&n)
	return n, err
}

// ClaimReview locks the oldest submission due for review: pending ones whose
// retry time has come, or reviewing ones whose lock expired (the reviewing
// node died). The WHERE clause is re-checked on the locked row, so exactly
// one node wins each submission.
func (s *Store) ClaimReview(ctx context.Context, node string, lockFor time.Duration) (*Topic, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	now := s.now()
	const due = `((status = 'pending' AND (next_attempt_at IS NULL OR next_attempt_at <= $1)) OR (status = 'reviewing' AND lock_until < $1))`
	var id int64
	err := s.DB.QueryRowContext(ctx, `UPDATE iq_monitor_topics SET status = 'reviewing', locked_by = $2, lock_until = $3, attempts = attempts + 1
WHERE id = (SELECT id FROM iq_monitor_topics WHERE `+due+` ORDER BY id LIMIT 1) AND `+due+`
RETURNING id`, now, node, now.Add(lockFor)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: claim review: %w", err)
	}
	items, err := s.queryTopics(ctx, `WHERE t.id = $1`, id)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

// FinishReview stores a verdict for a submission this node still holds.
func (s *Store) FinishReview(ctx context.Context, id int64, node string, approved bool, reason string) error {
	status := TopicApproved
	if !approved {
		status = TopicRejected
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_topics SET status = $1, reject_reason = $2, reviewed_at = $3, locked_by = '', lock_until = NULL, last_error = ''
WHERE id = $4 AND status = 'reviewing' AND locked_by = $5`, status, truncateRunes(reason, 480), s.now(), id, node)
	return err
}

// FailReview records a failed review attempt: back to pending with a
// growing delay, or failed after MaxReviewAttempts (then off the quota).
func (s *Store) FailReview(ctx context.Context, t Topic, node, errText string) error {
	now := s.now()
	if t.Attempts >= MaxReviewAttempts {
		_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_topics SET status = $1, last_error = $2, reject_reason = $3, reviewed_at = $4, locked_by = '', lock_until = NULL
WHERE id = $5 AND status = 'reviewing' AND locked_by = $6`, TopicFailed, truncateRunes(errText, 480), "审核服务暂时不可用，本次投稿不占用今日配额", now, t.ID, node)
		return err
	}
	next := now.Add(time.Duration(t.Attempts) * time.Minute)
	_, err := s.DB.ExecContext(ctx, `UPDATE iq_monitor_topics SET status = $1, last_error = $2, next_attempt_at = $3, locked_by = '', lock_until = NULL
WHERE id = $4 AND status = 'reviewing' AND locked_by = $5`, TopicPending, truncateRunes(errText, 480), next, t.ID, node)
	return err
}

// TakeQueuedTopic marks the longest-queued approved submission as used by a
// round. Only the node holding the round calls it.
func (s *Store) TakeQueuedTopic(ctx context.Context, roundID int64) (*Topic, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	var id int64
	err := s.DB.QueryRowContext(ctx, `UPDATE iq_monitor_topics SET status = $1, round_id = $2, used_at = $3
WHERE id = (SELECT id FROM iq_monitor_topics WHERE status = $4 ORDER BY id LIMIT 1) AND status = $4 RETURNING id`, TopicUsed, roundID, s.now(), TopicApproved).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("iq-monitor: take topic: %w", err)
	}
	items, err := s.queryTopics(ctx, `WHERE t.id = $1`, id)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func (s *Store) Topic(ctx context.Context, id int64) (*Topic, error) {
	items, err := s.queryTopics(ctx, `WHERE t.id = $1`, id)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func (s *Store) DeleteTopic(ctx context.Context, id int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	res, err := s.DB.ExecContext(ctx, `DELETE FROM iq_monitor_topics WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
