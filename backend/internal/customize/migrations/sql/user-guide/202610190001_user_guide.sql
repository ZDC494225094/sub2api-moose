CREATE TABLE user_guide_config (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    title TEXT NOT NULL DEFAULT '',
    content_md TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO user_guide_config (id) VALUES (1);

-- The guide replaces the previously always-on docs center, so it starts
-- enabled; an admin can switch it off in the extension manager at any time.
INSERT INTO settings (key, value, updated_at) VALUES
('custom_extensions.user-guide.enabled','true',CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
