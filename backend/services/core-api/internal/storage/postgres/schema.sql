-- Schema for core-api's Postgres storage backend.

CREATE TABLE IF NOT EXISTS tenants (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id                 TEXT PRIMARY KEY,
    tenant_id          TEXT NOT NULL REFERENCES tenants(id),
    zitadel_subject_id TEXT NOT NULL UNIQUE,
    display_name       TEXT NOT NULL,
    email              TEXT NOT NULL,
    role               TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS users_tenant_id_idx ON users(tenant_id);

CREATE TABLE IF NOT EXISTS products (
    id                 TEXT PRIMARY KEY,
    tenant_id          TEXT NOT NULL REFERENCES tenants(id),
    name               TEXT NOT NULL,
    price_per_kg_cents INTEGER NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS products_tenant_id_idx ON products(tenant_id);

CREATE TABLE IF NOT EXISTS transactions (
    id                 TEXT PRIMARY KEY,
    tenant_id          TEXT NOT NULL REFERENCES tenants(id),
    user_id            TEXT NOT NULL REFERENCES users(id),
    product_id         TEXT NOT NULL REFERENCES products(id),
    scale_id           TEXT NOT NULL,
    weight_grams       INTEGER NOT NULL,
    unit_price_cents   INTEGER NOT NULL,
    total_price_cents  INTEGER NOT NULL,
    scale_status_code  TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS transactions_tenant_id_idx ON transactions(tenant_id);

CREATE TABLE IF NOT EXISTS receipts (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL REFERENCES tenants(id),
    user_id      TEXT NOT NULL REFERENCES users(id),
    status       TEXT NOT NULL,
    number       INTEGER,
    created_at   TIMESTAMPTZ NOT NULL,
    finalized_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS receipts_tenant_id_idx ON receipts(tenant_id);
CREATE INDEX IF NOT EXISTS receipts_user_id_status_idx ON receipts(user_id, status);

-- Ordered line items on a receipt. A transaction can be removed from a draft
-- receipt (deleting its row here) without deleting the transaction itself.
CREATE TABLE IF NOT EXISTS receipt_lines (
    receipt_id     TEXT NOT NULL REFERENCES receipts(id),
    transaction_id TEXT NOT NULL REFERENCES transactions(id),
    position       INTEGER NOT NULL,
    PRIMARY KEY (receipt_id, transaction_id)
);

-- One row per tenant, incremented atomically to allocate sequential receipt
-- numbers.
CREATE TABLE IF NOT EXISTS receipt_number_sequences (
    tenant_id   TEXT PRIMARY KEY REFERENCES tenants(id),
    next_number INTEGER NOT NULL DEFAULT 1
);
