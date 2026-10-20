ALTER TABLE canvas_run_charges
    ADD COLUMN IF NOT EXISTS phase TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS upstream_task_id TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS error_message TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_canvas_run_charges_created_at
    ON canvas_run_charges (created_at DESC, id DESC);
