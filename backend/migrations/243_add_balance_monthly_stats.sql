CREATE TABLE IF NOT EXISTS balance_monthly_stats (
    period VARCHAR(7) PRIMARY KEY CHECK (period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    total_balance DECIMAL(20,8) NOT NULL,
    total_subscription_remaining DECIMAL(20,8) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
