CREATE TABLE bot_notify_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    config JSONB NOT NULL DEFAULT '{"bot_token":"","receive_enabled":false,"push_enabled":false,"chat_ids":[],"interval_minutes":60,"push_template":"{channel_status}"}',
    update_offset BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO bot_notify_config (id) VALUES (1);

CREATE TABLE bot_reply_rules (
    id BIGSERIAL PRIMARY KEY,
    keyword TEXT NOT NULL,
    match_type VARCHAR(16) NOT NULL CHECK (match_type IN ('contains','exact','command')),
    reply_template TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    sort INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX bot_reply_rules_order_idx ON bot_reply_rules (sort, id);
-- One match type per rule: the default behavior needs two ordered entries.
INSERT INTO bot_reply_rules (keyword, match_type, reply_template, sort) VALUES
('渠道状态','contains','{channel_status}',0),
('/status','command','{channel_status}',1);

CREATE TABLE bot_messages (
    id BIGSERIAL PRIMARY KEY,
    platform VARCHAR(32) NOT NULL,
    bot_id BIGINT NOT NULL,
    chat_id TEXT NOT NULL,
    chat_type VARCHAR(32) NOT NULL,
    sender_id TEXT NOT NULL,
    sender_name TEXT NOT NULL,
    text TEXT NOT NULL,
    message_id BIGINT NOT NULL,
    update_id BIGINT NOT NULL,
    rule_id BIGINT REFERENCES bot_reply_rules(id) ON DELETE SET NULL,
    replied BOOLEAN NOT NULL DEFAULT false,
    reply_content TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(platform, bot_id, update_id)
);
CREATE INDEX bot_messages_created_idx ON bot_messages (created_at DESC, id DESC);
CREATE INDEX bot_messages_chat_idx ON bot_messages (chat_id, id DESC);
INSERT INTO settings (key, value, updated_at) VALUES
('custom_extensions.bot-notify.enabled','false',CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
