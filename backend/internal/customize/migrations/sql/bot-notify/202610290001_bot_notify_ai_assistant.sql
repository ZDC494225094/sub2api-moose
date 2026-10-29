-- AI customer-service upgrade. Additive only: existing configs keep their
-- values, and disabling the module still leaves every row in place.
-- One column per statement keeps the file portable to the SQLite fixtures.
ALTER TABLE bot_ai_configs ADD COLUMN reasoning_effort TEXT NOT NULL DEFAULT 'low';
ALTER TABLE bot_ai_configs ADD COLUMN allow_images BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE bot_ai_configs ADD COLUMN allow_usage_stats BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE bot_ai_configs ADD COLUMN use_user_guide BOOLEAN NOT NULL DEFAULT true;
-- Empty means the built-in default knowledge base.
ALTER TABLE bot_ai_configs ADD COLUMN knowledge_base TEXT NOT NULL DEFAULT '';

-- A group chat is one conversation row; sender_id keeps each member's
-- history separate inside it. Images are never stored, only counted.
ALTER TABLE bot_ai_messages ADD COLUMN sender_id TEXT NOT NULL DEFAULT '';
ALTER TABLE bot_ai_messages ADD COLUMN image_count INTEGER NOT NULL DEFAULT 0;
CREATE INDEX bot_ai_messages_conversation_sender_idx ON bot_ai_messages (conversation_id, sender_id, id DESC);
ALTER TABLE bot_messages ADD COLUMN image_count INTEGER NOT NULL DEFAULT 0;
