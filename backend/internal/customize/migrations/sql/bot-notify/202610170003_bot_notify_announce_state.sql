-- Announcement sync watermark per bot: {"seeded": bool, "hashes": {"<announcement id>": "<content hash>"}}.
-- Seeded starts false so enabling sync never blasts the announcement history;
-- the first watcher pass records the baseline, later passes push only new or
-- edited announcements.
ALTER TABLE bot_notify_bots ADD COLUMN announce_state JSONB NOT NULL DEFAULT '{}';
