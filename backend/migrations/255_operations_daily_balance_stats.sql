CREATE TABLE IF NOT EXISTS balance_daily_stats (
    day DATE PRIMARY KEY,
    captured_at TIMESTAMPTZ NOT NULL,
    total_balance DECIMAL(20,8) NOT NULL,
    total_subscription_remaining DECIMAL(20,8) NOT NULL
);
