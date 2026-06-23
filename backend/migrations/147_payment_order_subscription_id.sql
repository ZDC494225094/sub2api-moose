-- Track the exact subscription instance created by each subscription order.

ALTER TABLE payment_orders
    ADD COLUMN IF NOT EXISTS subscription_id BIGINT REFERENCES user_subscriptions(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS payment_orders_subscription_id
    ON payment_orders(subscription_id);
