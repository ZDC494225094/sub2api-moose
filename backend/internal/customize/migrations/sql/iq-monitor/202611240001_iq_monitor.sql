-- 降智监测 (iq-monitor): periodic same-prompt probes of configured models
-- through the gateway's own relay API, with public pass-rate timelines and
-- user-submitted drawing topics. Times are written by the application in UTC.

-- Single-row module configuration (JSON document, validated in Go).
CREATE TABLE iq_monitor_settings (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    config TEXT NOT NULL DEFAULT '{}',
    -- HMAC key for short-lived drawing preview URLs (generated on first use,
    -- shared by all nodes, never sent to clients).
    preview_secret VARCHAR(128) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO iq_monitor_settings (id) VALUES (1);

-- Probed targets, shown in sort_order. reasoning_effort '' sends nothing.
CREATE TABLE iq_monitor_targets (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL,
    model VARCHAR(200) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    reasoning_effort VARCHAR(16) NOT NULL DEFAULT '',
    api_key_id BIGINT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX iq_monitor_targets_sort_idx ON iq_monitor_targets (sort_order, id);

-- Logic question bank. answers is a JSON array of accepted answers.
CREATE TABLE iq_monitor_questions (
    id BIGSERIAL PRIMARY KEY,
    question TEXT NOT NULL,
    answers TEXT NOT NULL DEFAULT '[]',
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One row per round. slot_key is the cluster-wide claim: the node whose
-- insert succeeds runs the round. heartbeat_at lets other nodes detect a
-- round whose node stopped mid-way and mark its unfinished work interrupted.
-- Question and topic are snapshots so later edits never rewrite history.
CREATE TABLE iq_monitor_rounds (
    id BIGSERIAL PRIMARY KEY,
    slot_key VARCHAR(64) NOT NULL UNIQUE,
    trigger_kind VARCHAR(16) NOT NULL DEFAULT 'schedule',
    node_id VARCHAR(128) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'running',
    logic_enabled BOOLEAN NOT NULL DEFAULT false,
    drawing_enabled BOOLEAN NOT NULL DEFAULT false,
    question_id BIGINT,
    question TEXT NOT NULL DEFAULT '',
    answers TEXT NOT NULL DEFAULT '[]',
    topic TEXT NOT NULL DEFAULT '',
    topic_source VARCHAR(16) NOT NULL DEFAULT '',
    topic_id BIGINT,
    started_at TIMESTAMPTZ NOT NULL,
    heartbeat_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ
);
CREATE INDEX iq_monitor_rounds_started_idx ON iq_monitor_rounds (started_at);
CREATE INDEX iq_monitor_rounds_status_idx ON iq_monitor_rounds (status);

-- One row per target and test kind in a round. Target identity fields are
-- snapshots; history is keyed by (group_id, model, reasoning_effort).
CREATE TABLE iq_monitor_results (
    id BIGSERIAL PRIMARY KEY,
    round_id BIGINT NOT NULL REFERENCES iq_monitor_rounds(id) ON DELETE CASCADE,
    target_id BIGINT NOT NULL,
    kind VARCHAR(16) NOT NULL,
    group_id BIGINT NOT NULL,
    group_name VARCHAR(200) NOT NULL DEFAULT '',
    model VARCHAR(200) NOT NULL,
    display_name VARCHAR(100) NOT NULL DEFAULT '',
    reasoning_effort VARCHAR(16) NOT NULL DEFAULT '',
    sent_effort VARCHAR(16) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    reason VARCHAR(500) NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    request_count INTEGER NOT NULL DEFAULT 0,
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    first_token_ms INTEGER,
    total_ms INTEGER,
    truncated BOOLEAN NOT NULL DEFAULT false,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX iq_monitor_results_round_idx ON iq_monitor_results (round_id);
CREATE INDEX iq_monitor_results_key_idx ON iq_monitor_results (group_id, model, reasoning_effort, kind, id);
CREATE INDEX iq_monitor_results_created_idx ON iq_monitor_results (created_at);

-- Every answered attempt (logic re-asks keep earlier wrong answers).
CREATE TABLE iq_monitor_responses (
    id BIGSERIAL PRIMARY KEY,
    result_id BIGINT NOT NULL REFERENCES iq_monitor_results(id) ON DELETE CASCADE,
    attempt INTEGER NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    extracted VARCHAR(500) NOT NULL DEFAULT '',
    correct BOOLEAN NOT NULL DEFAULT false,
    finish_reason VARCHAR(32) NOT NULL DEFAULT '',
    truncated BOOLEAN NOT NULL DEFAULT false,
    error VARCHAR(500) NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    first_token_ms INTEGER,
    total_ms INTEGER,
    created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX iq_monitor_responses_result_idx ON iq_monitor_responses (result_id, attempt);

-- User topic submissions. quota_day is the submission day in the site
-- timezone; failed reviews do not count toward the daily quota.
CREATE TABLE iq_monitor_topics (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    topic VARCHAR(400) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    reject_reason VARCHAR(500) NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error VARCHAR(500) NOT NULL DEFAULT '',
    locked_by VARCHAR(128) NOT NULL DEFAULT '',
    lock_until TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ,
    quota_day VARCHAR(10) NOT NULL,
    round_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL,
    reviewed_at TIMESTAMPTZ,
    used_at TIMESTAMPTZ
);
CREATE INDEX iq_monitor_topics_user_day_idx ON iq_monitor_topics (user_id, quota_day);
CREATE INDEX iq_monitor_topics_status_idx ON iq_monitor_topics (status, id);

-- Users allowed to view the public page while visibility is 'allowlist'.
CREATE TABLE iq_monitor_viewers (
    user_id BIGINT PRIMARY KEY,
    note VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Last evaluated status per history key, so a degradation alert is pushed
-- once per transition instead of every round.
CREATE TABLE iq_monitor_alert_state (
    group_id BIGINT NOT NULL,
    model VARCHAR(200) NOT NULL,
    reasoning_effort VARCHAR(16) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (group_id, model, reasoning_effort)
);

-- New modules start switched off; an administrator enables it in the extension manager.
INSERT INTO settings (key, value, updated_at) VALUES
('custom_extensions.iq-monitor.enabled', 'false', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
