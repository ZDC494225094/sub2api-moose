-- Optional AI support for bot-notify.  This migration is additive: disabling
-- the module leaves the configuration, keyword rules and audit records intact.
CREATE TABLE bot_ai_configs (
    bot_id BIGINT PRIMARY KEY REFERENCES bot_notify_bots(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    api_key_id BIGINT,
    model TEXT NOT NULL DEFAULT '',
    max_input_chars INTEGER NOT NULL DEFAULT 4000 CHECK (max_input_chars BETWEEN 1 AND 8000),
    max_output_tokens INTEGER NOT NULL DEFAULT 800 CHECK (max_output_tokens BETWEEN 1 AND 4000),
    cooldown_seconds INTEGER NOT NULL DEFAULT 10 CHECK (cooldown_seconds BETWEEN 0 AND 3600),
    daily_limit INTEGER NOT NULL DEFAULT 100 CHECK (daily_limit BETWEEN 0 AND 100000),
    allow_recharge BOOLEAN NOT NULL DEFAULT true,
    allow_technical BOOLEAN NOT NULL DEFAULT true,
    knowledge_version TEXT NOT NULL DEFAULT 'public-v1',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE bot_ai_conversations (
    id BIGSERIAL PRIMARY KEY,
    bot_id BIGINT NOT NULL REFERENCES bot_notify_bots(id) ON DELETE CASCADE,
    platform VARCHAR(32) NOT NULL,
    chat_id TEXT NOT NULL,
    external_user_id TEXT NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'active',
    last_message_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(bot_id, platform, chat_id)
);
CREATE INDEX bot_ai_conversations_bot_chat_idx ON bot_ai_conversations(bot_id, chat_id);

CREATE TABLE bot_ai_messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES bot_ai_conversations(id) ON DELETE CASCADE,
    direction VARCHAR(16) NOT NULL CHECK (direction IN ('user','assistant','system')),
    external_message_id TEXT NOT NULL DEFAULT '',
    question_redacted TEXT NOT NULL DEFAULT '',
    sender_name TEXT NOT NULL DEFAULT '',
    content_redacted TEXT NOT NULL DEFAULT '',
    blocked BOOLEAN NOT NULL DEFAULT false,
    blocked_reason TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    api_key_id BIGINT,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    error_code TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX bot_ai_messages_created_idx ON bot_ai_messages(created_at DESC, id DESC);
CREATE INDEX bot_ai_messages_conversation_idx ON bot_ai_messages(conversation_id, id DESC);
