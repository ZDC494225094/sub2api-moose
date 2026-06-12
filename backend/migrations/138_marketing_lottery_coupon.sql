-- Marketing coupon templates, user coupons, lottery activities, prizes, and order-level coupon reservations.

CREATE TABLE IF NOT EXISTS coupon_templates (
    id               BIGSERIAL PRIMARY KEY,
    name             VARCHAR(100) NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    scope            VARCHAR(20) NOT NULL,
    discount_amount  DECIMAL(20, 2) NOT NULL,
    threshold_amount DECIMAL(20, 2) NOT NULL DEFAULT 0,
    valid_days       INTEGER,
    valid_from       TIMESTAMPTZ,
    valid_until      TIMESTAMPTZ,
    status           VARCHAR(20) NOT NULL DEFAULT 'active',
    notes            TEXT NOT NULL DEFAULT '',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT coupon_templates_scope_check CHECK (scope IN ('balance', 'subscription', 'universal')),
    CONSTRAINT coupon_templates_status_check CHECK (status IN ('active', 'disabled')),
    CONSTRAINT coupon_templates_discount_amount_check CHECK (discount_amount > 0),
    CONSTRAINT coupon_templates_threshold_amount_check CHECK (threshold_amount >= 0),
    CONSTRAINT coupon_templates_valid_days_check CHECK (valid_days IS NULL OR valid_days > 0)
);

CREATE INDEX IF NOT EXISTS idx_coupon_templates_status_created_at
    ON coupon_templates (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_coupon_templates_scope_status
    ON coupon_templates (scope, status);

CREATE TABLE IF NOT EXISTS user_coupons (
    id               BIGSERIAL PRIMARY KEY,
    template_id      BIGINT NOT NULL REFERENCES coupon_templates(id) ON DELETE CASCADE,
    user_id          BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    coupon_code      VARCHAR(64) NOT NULL UNIQUE,
    source_type      VARCHAR(32) NOT NULL,
    source_ref_id    BIGINT,
    scope            VARCHAR(20) NOT NULL,
    discount_amount  DECIMAL(20, 2) NOT NULL,
    threshold_amount DECIMAL(20, 2) NOT NULL DEFAULT 0,
    valid_from       TIMESTAMPTZ,
    valid_until      TIMESTAMPTZ,
    status           VARCHAR(20) NOT NULL DEFAULT 'unused',
    reserved_order_id BIGINT REFERENCES payment_orders(id) ON DELETE SET NULL,
    reserved_at      TIMESTAMPTZ,
    used_order_id    BIGINT REFERENCES payment_orders(id) ON DELETE SET NULL,
    used_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_coupons_scope_check CHECK (scope IN ('balance', 'subscription', 'universal')),
    CONSTRAINT user_coupons_status_check CHECK (status IN ('unused', 'reserved', 'used', 'expired', 'disabled')),
    CONSTRAINT user_coupons_discount_amount_check CHECK (discount_amount > 0),
    CONSTRAINT user_coupons_threshold_amount_check CHECK (threshold_amount >= 0)
);

CREATE INDEX IF NOT EXISTS idx_user_coupons_user_status
    ON user_coupons (user_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_coupons_reserved_order
    ON user_coupons (reserved_order_id);

CREATE INDEX IF NOT EXISTS idx_user_coupons_used_order
    ON user_coupons (used_order_id);

CREATE TABLE IF NOT EXISTS payment_order_discounts (
    id                    BIGSERIAL PRIMARY KEY,
    order_id              BIGINT NOT NULL UNIQUE REFERENCES payment_orders(id) ON DELETE CASCADE,
    user_coupon_id        BIGINT REFERENCES user_coupons(id) ON DELETE SET NULL,
    coupon_template_id    BIGINT REFERENCES coupon_templates(id) ON DELETE SET NULL,
    coupon_code           VARCHAR(64) NOT NULL,
    scope                 VARCHAR(20) NOT NULL,
    discount_amount       DECIMAL(20, 2) NOT NULL,
    threshold_amount      DECIMAL(20, 2) NOT NULL DEFAULT 0,
    original_amount       DECIMAL(20, 2) NOT NULL,
    discounted_amount     DECIMAL(20, 2) NOT NULL,
    status                VARCHAR(20) NOT NULL DEFAULT 'reserved',
    reserved_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    released_at           TIMESTAMPTZ,
    used_at               TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT payment_order_discounts_scope_check CHECK (scope IN ('balance', 'subscription', 'universal')),
    CONSTRAINT payment_order_discounts_status_check CHECK (status IN ('reserved', 'released', 'used')),
    CONSTRAINT payment_order_discounts_discount_amount_check CHECK (discount_amount > 0),
    CONSTRAINT payment_order_discounts_threshold_amount_check CHECK (threshold_amount >= 0),
    CONSTRAINT payment_order_discounts_amount_check CHECK (original_amount > 0 AND discounted_amount >= 0.01)
);

CREATE INDEX IF NOT EXISTS idx_payment_order_discounts_coupon
    ON payment_order_discounts (user_coupon_id, status);

CREATE TABLE IF NOT EXISTS lottery_activities (
    id                        BIGSERIAL PRIMARY KEY,
    name                      VARCHAR(100) NOT NULL,
    description               TEXT NOT NULL DEFAULT '',
    status                    VARCHAR(20) NOT NULL DEFAULT 'draft',
    default_draw_times        INTEGER NOT NULL DEFAULT 0,
    consume_threshold_amount  DECIMAL(20, 2) NOT NULL DEFAULT 0,
    wallet_cost_per_draw      DECIMAL(20, 2) NOT NULL DEFAULT 0,
    starts_at                 TIMESTAMPTZ,
    ends_at                   TIMESTAMPTZ,
    sort_order                INTEGER NOT NULL DEFAULT 0,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_activities_status_check CHECK (status IN ('draft', 'active', 'inactive', 'ended')),
    CONSTRAINT lottery_activities_default_draw_times_check CHECK (default_draw_times >= 0),
    CONSTRAINT lottery_activities_consume_threshold_amount_check CHECK (consume_threshold_amount >= 0),
    CONSTRAINT lottery_activities_wallet_cost_per_draw_check CHECK (wallet_cost_per_draw >= 0)
);

CREATE INDEX IF NOT EXISTS idx_lottery_activities_status_sort
    ON lottery_activities (status, sort_order ASC, id ASC);

CREATE TABLE IF NOT EXISTS lottery_prizes (
    id                BIGSERIAL PRIMARY KEY,
    activity_id       BIGINT NOT NULL REFERENCES lottery_activities(id) ON DELETE CASCADE,
    name              VARCHAR(100) NOT NULL,
    prize_type        VARCHAR(20) NOT NULL,
    stock             INTEGER NOT NULL DEFAULT 0,
    remaining_stock   INTEGER NOT NULL DEFAULT 0,
    balance_amount    DECIMAL(20, 2),
    coupon_template_id BIGINT REFERENCES coupon_templates(id) ON DELETE SET NULL,
    display_order     INTEGER NOT NULL DEFAULT 0,
    status            VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_prizes_prize_type_check CHECK (prize_type IN ('balance_redeem', 'coupon', 'thanks')),
    CONSTRAINT lottery_prizes_status_check CHECK (status IN ('active', 'inactive')),
    CONSTRAINT lottery_prizes_stock_check CHECK (stock >= 0 AND remaining_stock >= 0 AND remaining_stock <= stock),
    CONSTRAINT lottery_prizes_balance_amount_check CHECK (balance_amount IS NULL OR balance_amount > 0)
);

CREATE INDEX IF NOT EXISTS idx_lottery_prizes_activity_status
    ON lottery_prizes (activity_id, status, display_order ASC, id ASC);

CREATE TABLE IF NOT EXISTS lottery_user_states (
    activity_id               BIGINT NOT NULL REFERENCES lottery_activities(id) ON DELETE CASCADE,
    user_id                   BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    default_granted           BOOLEAN NOT NULL DEFAULT FALSE,
    available_draw_times      INTEGER NOT NULL DEFAULT 0,
    total_granted_times       INTEGER NOT NULL DEFAULT 0,
    total_drawn_times         INTEGER NOT NULL DEFAULT 0,
    total_wallet_paid_amount  DECIMAL(20, 2) NOT NULL DEFAULT 0,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (activity_id, user_id),
    CONSTRAINT lottery_user_states_available_draw_times_check CHECK (available_draw_times >= 0),
    CONSTRAINT lottery_user_states_total_granted_times_check CHECK (total_granted_times >= 0),
    CONSTRAINT lottery_user_states_total_drawn_times_check CHECK (total_drawn_times >= 0),
    CONSTRAINT lottery_user_states_total_wallet_paid_amount_check CHECK (total_wallet_paid_amount >= 0)
);

CREATE TABLE IF NOT EXISTS lottery_chance_logs (
    id             BIGSERIAL PRIMARY KEY,
    activity_id    BIGINT NOT NULL REFERENCES lottery_activities(id) ON DELETE CASCADE,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    change_amount  INTEGER NOT NULL,
    balance_after  INTEGER NOT NULL,
    source_type    VARCHAR(32) NOT NULL,
    source_ref_id  BIGINT,
    notes          TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_lottery_chance_logs_user_activity_created
    ON lottery_chance_logs (user_id, activity_id, created_at DESC);

CREATE TABLE IF NOT EXISTS lottery_draw_records (
    id                BIGSERIAL PRIMARY KEY,
    activity_id       BIGINT NOT NULL REFERENCES lottery_activities(id) ON DELETE CASCADE,
    user_id           BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prize_id          BIGINT REFERENCES lottery_prizes(id) ON DELETE SET NULL,
    prize_name        VARCHAR(100) NOT NULL,
    prize_type        VARCHAR(20) NOT NULL,
    result_code       VARCHAR(20) NOT NULL DEFAULT 'win',
    chance_source     VARCHAR(20) NOT NULL,
    wallet_amount     DECIMAL(20, 2) NOT NULL DEFAULT 0,
    user_coupon_id    BIGINT REFERENCES user_coupons(id) ON DELETE SET NULL,
    reward_reference  VARCHAR(128) NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_draw_records_prize_type_check CHECK (prize_type IN ('balance_redeem', 'coupon', 'thanks')),
    CONSTRAINT lottery_draw_records_result_code_check CHECK (result_code IN ('win', 'thanks')),
    CONSTRAINT lottery_draw_records_chance_source_check CHECK (chance_source IN ('default', 'wallet'))
);

CREATE INDEX IF NOT EXISTS idx_lottery_draw_records_user_activity_created
    ON lottery_draw_records (user_id, activity_id, created_at DESC);
