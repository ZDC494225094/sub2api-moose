-- Scheduled push watermark: the latest schedule slot (interval boundary or
-- daily HH:MM point) a bot has claimed. Claiming is a conditional UPDATE, so
-- each slot is sent once across restarts and instances sharing the database.
-- NULL means never claimed; such bots do not catch up a missed slot.
ALTER TABLE bot_notify_bots ADD COLUMN last_push_at TIMESTAMPTZ;
