ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS batch_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS idx_redeem_codes_batch_id
    ON redeem_codes(batch_id);

CREATE TABLE IF NOT EXISTS redeem_code_batch_usages (
    id BIGSERIAL PRIMARY KEY,
    batch_id VARCHAR(64) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    redeem_code_id BIGINT NOT NULL REFERENCES redeem_codes(id) ON DELETE CASCADE,
    used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT redeem_code_batch_usages_batch_user_key UNIQUE(batch_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_redeem_code_batch_usages_batch_id
    ON redeem_code_batch_usages(batch_id);

CREATE INDEX IF NOT EXISTS idx_redeem_code_batch_usages_user_id
    ON redeem_code_batch_usages(user_id);
