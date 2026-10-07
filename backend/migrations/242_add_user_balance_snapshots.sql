CREATE TABLE IF NOT EXISTS user_balance_snapshots (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL CHECK (period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    balance DECIMAL(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS userbalancesnapshot_user_id_period
    ON user_balance_snapshots (user_id, period);

CREATE OR REPLACE FUNCTION capture_month_end_user_balance()
RETURNS TRIGGER AS $$
DECLARE
    captured_at TIMESTAMPTZ := statement_timestamp();
    month_start TIMESTAMPTZ := date_trunc('month', captured_at);
BEGIN
    IF captured_at < month_start + INTERVAL '1 day'
       AND OLD.created_at < month_start
       AND OLD.deleted_at IS NULL
       AND OLD.balance IS DISTINCT FROM NEW.balance THEN
        INSERT INTO user_balance_snapshots (user_id, period, balance, created_at)
        VALUES (OLD.id, to_char(month_start - INTERVAL '1 month', 'YYYY-MM'), OLD.balance, captured_at)
        ON CONFLICT (user_id, period) DO UPDATE
        SET balance = EXCLUDED.balance, created_at = EXCLUDED.created_at
        WHERE user_balance_snapshots.created_at < month_start;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS capture_month_end_user_balance ON users;
CREATE TRIGGER capture_month_end_user_balance
    BEFORE UPDATE OF balance ON users
    FOR EACH ROW EXECUTE FUNCTION capture_month_end_user_balance();
