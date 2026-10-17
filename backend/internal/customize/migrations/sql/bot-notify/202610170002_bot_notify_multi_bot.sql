-- Multi-bot: one row per configured bot. The historical single-bot config
-- (bot_notify_config id=1) becomes the first bot row; its rules and stored
-- messages are re-parented to it. bot_notify_config is kept for history only.
CREATE TABLE bot_notify_bots (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    platform VARCHAR(32) NOT NULL DEFAULT 'telegram',
    config JSONB NOT NULL DEFAULT '{"bot_token":"","receive_enabled":false,"push_enabled":false,"chat_ids":[],"interval_minutes":60,"push_template":"{channel_status}"}',
    update_offset BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO bot_notify_bots (name, platform, config, update_offset)
SELECT '默认机器人', 'telegram', config, update_offset FROM bot_notify_config WHERE id = 1;

ALTER TABLE bot_reply_rules ADD COLUMN bot_id BIGINT REFERENCES bot_notify_bots(id) ON DELETE CASCADE;
UPDATE bot_reply_rules SET bot_id = (SELECT id FROM bot_notify_bots ORDER BY id LIMIT 1) WHERE bot_id IS NULL;
CREATE INDEX bot_reply_rules_bot_idx ON bot_reply_rules (bot_id, sort, id);

ALTER TABLE bot_messages ADD COLUMN bot_row_id BIGINT REFERENCES bot_notify_bots(id) ON DELETE CASCADE;
UPDATE bot_messages SET bot_row_id = (SELECT id FROM bot_notify_bots ORDER BY id LIMIT 1) WHERE bot_row_id IS NULL;
CREATE INDEX bot_messages_bot_idx ON bot_messages (bot_row_id, id DESC);
