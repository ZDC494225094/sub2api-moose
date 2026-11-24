// Package iqmonitor implements the 降智监测 extension: every round all
// enabled targets get the same logic question and the same drawing topic
// through the gateway's own relay API, so results are comparable across
// models. Pass rates over the last 24 hours drive a public status page.
package iqmonitor

import "time"

const ID = "iq-monitor"

// Test kinds.
const (
	KindLogic   = "logic"
	KindDrawing = "drawing"
)

// Result statuses. retry_pass counts as passed; interrupted is excluded
// from pass rates because the node, not the model, stopped the request.
const (
	StatusPending     = "pending"
	StatusRunning     = "running"
	StatusPass        = "pass"
	StatusRetryPass   = "retry_pass"
	StatusFail        = "fail"
	StatusError       = "error"
	StatusInterrupted = "interrupted"
	// StatusSkipped marks a probe that never reached the model because of
	// site configuration (missing key, no probe account); not counted.
	StatusSkipped = "skipped"
)

// Round statuses.
const (
	RoundRunning     = "running"
	RoundDone        = "done"
	RoundInterrupted = "interrupted"
)

// Topic submission statuses.
const (
	TopicPending   = "pending"
	TopicReviewing = "reviewing"
	TopicApproved  = "approved"
	TopicRejected  = "rejected"
	TopicFailed    = "failed"
	TopicUsed      = "used"
)

// Model health derived from the 24h pass rate.
const (
	HealthNormal    = "normal"
	HealthUnstable  = "unstable"
	HealthDegraded  = "degraded"
	HealthNoData    = "no_data"
	VisibilityAll   = "all"
	VisibilityUsers = "allowlist"
)

const MaxTargets = 30

type Target struct {
	ID              int64     `json:"id"`
	GroupID         int64     `json:"group_id"`
	Model           string    `json:"model"`
	DisplayName     string    `json:"display_name"`
	ReasoningEffort string    `json:"reasoning_effort"`
	APIKeyID        int64     `json:"api_key_id"`
	Enabled         bool      `json:"enabled"`
	SortOrder       int       `json:"sort_order"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Question struct {
	ID         int64      `json:"id"`
	Question   string     `json:"question"`
	Answers    []string   `json:"answers"`
	Enabled    bool       `json:"enabled"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Round struct {
	ID             int64      `json:"id"`
	SlotKey        string     `json:"-"`
	Trigger        string     `json:"trigger"`
	NodeID         string     `json:"-"`
	Status         string     `json:"status"`
	LogicEnabled   bool       `json:"logic_enabled"`
	DrawingEnabled bool       `json:"drawing_enabled"`
	QuestionID     *int64     `json:"question_id"`
	Question       string     `json:"question"`
	Answers        []string   `json:"answers"`
	Topic          string     `json:"topic"`
	TopicSource    string     `json:"topic_source"`
	TopicID        *int64     `json:"topic_id"`
	StartedAt      time.Time  `json:"started_at"`
	HeartbeatAt    time.Time  `json:"-"`
	FinishedAt     *time.Time `json:"finished_at"`
}

type Result struct {
	ID              int64      `json:"id"`
	RoundID         int64      `json:"round_id"`
	TargetID        int64      `json:"target_id"`
	Kind            string     `json:"kind"`
	GroupID         int64      `json:"group_id"`
	GroupName       string     `json:"group_name"`
	Model           string     `json:"model"`
	DisplayName     string     `json:"display_name"`
	ReasoningEffort string     `json:"reasoning_effort"`
	SentEffort      string     `json:"sent_effort"`
	Status          string     `json:"status"`
	Reason          string     `json:"reason"`
	Attempts        int        `json:"attempts"`
	RequestCount    int        `json:"request_count"`
	InputTokens     int64      `json:"input_tokens"`
	OutputTokens    int64      `json:"output_tokens"`
	FirstTokenMS    *int       `json:"first_token_ms"`
	TotalMS         *int       `json:"total_ms"`
	Truncated       bool       `json:"truncated"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Response struct {
	ID           int64     `json:"id"`
	ResultID     int64     `json:"result_id"`
	Attempt      int       `json:"attempt"`
	Content      string    `json:"content"`
	Extracted    string    `json:"extracted"`
	Correct      bool      `json:"correct"`
	FinishReason string    `json:"finish_reason"`
	Truncated    bool      `json:"truncated"`
	Error        string    `json:"error"`
	InputTokens  int64     `json:"input_tokens"`
	OutputTokens int64     `json:"output_tokens"`
	FirstTokenMS *int      `json:"first_token_ms"`
	TotalMS      *int      `json:"total_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

type Topic struct {
	ID           int64      `json:"id"`
	UserID       int64      `json:"user_id"`
	Username     string     `json:"username,omitempty"`
	Email        string     `json:"email,omitempty"`
	Topic        string     `json:"topic"`
	Status       string     `json:"status"`
	RejectReason string     `json:"reject_reason"`
	Attempts     int        `json:"attempts"`
	LastError    string     `json:"last_error,omitempty"`
	QuotaDay     string     `json:"quota_day"`
	RoundID      *int64     `json:"round_id"`
	CreatedAt    time.Time  `json:"created_at"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	UsedAt       *time.Time `json:"used_at"`
}

// historyKey identifies a model's history across target edits: deleting and
// re-adding the same group, model and effort keeps the timeline.
type historyKey struct {
	GroupID int64
	Model   string
	Effort  string
}

func keyOf(t Target) historyKey {
	return historyKey{GroupID: t.GroupID, Model: t.Model, Effort: t.ReasoningEffort}
}

func passed(status string) bool { return status == StatusPass || status == StatusRetryPass }

// counted reports whether a finished result enters pass-rate statistics.
func counted(status string) bool {
	switch status {
	case StatusPass, StatusRetryPass, StatusFail, StatusError:
		return true
	}
	return false
}
