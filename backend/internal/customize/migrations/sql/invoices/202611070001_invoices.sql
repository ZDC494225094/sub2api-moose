-- Invoice applications for on-site paid orders. Users invoice their own
-- completed CNY payment orders (balance recharges and plan purchases), several
-- orders per application; administrators review, issue or reject them.

-- Single-row module settings. Access defaults to an allowlist so the feature
-- reaches nobody until an administrator names the users who may apply.
CREATE TABLE invoice_settings (
    id SMALLINT PRIMARY KEY CHECK (id = 1),
    min_amount NUMERIC(20,2) NOT NULL DEFAULT 0,
    allow_special BOOLEAN NOT NULL DEFAULT true,
    order_window_days INTEGER NOT NULL DEFAULT 365,
    access_mode VARCHAR(16) NOT NULL DEFAULT 'allowlist' CHECK (access_mode IN ('all', 'allowlist')),
    item_name VARCHAR(100) NOT NULL DEFAULT '',
    notice TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO invoice_settings (id) VALUES (1);

-- Users allowed to apply while access_mode is 'allowlist'.
CREATE TABLE invoice_user_access (
    user_id BIGINT PRIMARY KEY,
    note VARCHAR(200) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Saved invoice titles (抬头). Applications copy the title, so later edits or
-- deletions never change a submitted application.
CREATE TABLE invoice_titles (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    title_type VARCHAR(16) NOT NULL CHECK (title_type IN ('personal', 'company')),
    name VARCHAR(100) NOT NULL,
    tax_no VARCHAR(32) NOT NULL DEFAULT '',
    address VARCHAR(200) NOT NULL DEFAULT '',
    phone VARCHAR(32) NOT NULL DEFAULT '',
    bank_name VARCHAR(100) NOT NULL DEFAULT '',
    bank_account VARCHAR(64) NOT NULL DEFAULT '',
    email VARCHAR(255) NOT NULL DEFAULT '',
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX invoice_titles_user_idx ON invoice_titles (user_id, id);

CREATE TABLE invoice_applications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    invoice_type VARCHAR(16) NOT NULL CHECK (invoice_type IN ('normal', 'special')),
    title_type VARCHAR(16) NOT NULL CHECK (title_type IN ('personal', 'company')),
    title_name VARCHAR(100) NOT NULL,
    tax_no VARCHAR(32) NOT NULL DEFAULT '',
    address VARCHAR(200) NOT NULL DEFAULT '',
    phone VARCHAR(32) NOT NULL DEFAULT '',
    bank_name VARCHAR(100) NOT NULL DEFAULT '',
    bank_account VARCHAR(64) NOT NULL DEFAULT '',
    email VARCHAR(255) NOT NULL DEFAULT '',
    item_name VARCHAR(100) NOT NULL DEFAULT '',
    amount NUMERIC(20,2) NOT NULL,
    order_count INTEGER NOT NULL,
    remark VARCHAR(200) NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'issued', 'rejected', 'cancelled', 'voided')),
    invoice_no VARCHAR(64) NOT NULL DEFAULT '',
    reject_reason VARCHAR(500) NOT NULL DEFAULT '',
    admin_note VARCHAR(500) NOT NULL DEFAULT '',
    handled_by BIGINT,
    issued_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX invoice_applications_user_idx ON invoice_applications (user_id, id);
CREATE INDEX invoice_applications_status_idx ON invoice_applications (status, id);

-- Orders of an application. "active" is true while the application is pending,
-- processing or issued; the partial unique index then guarantees that an order
-- is part of at most one live application, even under concurrent submissions.
CREATE TABLE invoice_application_orders (
    application_id BIGINT NOT NULL REFERENCES invoice_applications (id) ON DELETE CASCADE,
    order_id BIGINT NOT NULL,
    out_trade_no VARCHAR(64) NOT NULL DEFAULT '',
    order_type VARCHAR(20) NOT NULL DEFAULT '',
    amount NUMERIC(20,2) NOT NULL,
    paid_at TIMESTAMPTZ,
    active BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (application_id, order_id)
);
CREATE UNIQUE INDEX invoice_application_orders_live_idx ON invoice_application_orders (order_id) WHERE active;

-- The issued electronic invoice (PDF), at most one per application.
CREATE TABLE invoice_files (
    application_id BIGINT PRIMARY KEY REFERENCES invoice_applications (id) ON DELETE CASCADE,
    file_name VARCHAR(200) NOT NULL DEFAULT '',
    size_bytes INTEGER NOT NULL,
    data BYTEA NOT NULL,
    uploaded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- New modules start switched off; an administrator enables it in the extension manager.
INSERT INTO settings (key, value, updated_at) VALUES
('custom_extensions.invoices.enabled', 'false', CURRENT_TIMESTAMP)
ON CONFLICT (key) DO NOTHING;
