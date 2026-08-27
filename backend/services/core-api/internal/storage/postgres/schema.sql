-- Schema for core-api's Postgres storage backend.

CREATE TABLE IF NOT EXISTS tenants (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id                 TEXT PRIMARY KEY,
    tenant_id          TEXT NOT NULL REFERENCES tenants(id),
    rauthy_subject_id TEXT NOT NULL UNIQUE,
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
    pricing_type       TEXT NOT NULL DEFAULT 'per_kg',
    unit_price_cents   INTEGER NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS products_tenant_id_idx ON products(tenant_id);

CREATE TABLE IF NOT EXISTS transactions (
    id                 TEXT PRIMARY KEY,
    tenant_id          TEXT NOT NULL REFERENCES tenants(id),
    user_id            TEXT NOT NULL REFERENCES users(id),
    product_id         TEXT NOT NULL REFERENCES products(id),
    product_name       TEXT NOT NULL,
    pricing_type       TEXT NOT NULL DEFAULT 'per_kg',
    scale_id           TEXT NOT NULL,
    weight_grams       INTEGER NOT NULL,
    quantity           INTEGER NOT NULL DEFAULT 0,
    unit_price_cents   INTEGER NOT NULL,
    total_price_cents  INTEGER NOT NULL,
    scale_status_code  TEXT NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS transactions_tenant_id_idx ON transactions(tenant_id);

-- schema.sql is applied idempotently on every startup (see Store.Migrate)
-- rather than through a versioned migration tool, so a column added after a
-- table already existed needs an explicit ALTER here rather than relying on
-- CREATE TABLE IF NOT EXISTS, which is a no-op for it against installations
-- that were migrated before the column existed. This won't scale past a
-- handful of changes — a real migration tool (golang-migrate, goose, etc.)
-- is worth adopting before schema changes become frequent.
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS product_name TEXT NOT NULL DEFAULT '';
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS pricing_type TEXT NOT NULL DEFAULT 'per_kg';
ALTER TABLE transactions ADD COLUMN IF NOT EXISTS quantity INTEGER NOT NULL DEFAULT 0;

ALTER TABLE products ADD COLUMN IF NOT EXISTS pricing_type TEXT NOT NULL DEFAULT 'per_kg';
-- Renaming (rather than adding) a column can't use ADD COLUMN IF NOT
-- EXISTS, so it's guarded explicitly: only fires against an installation
-- that still has the old column name, and is a no-op on a fresh install
-- where CREATE TABLE above already created unit_price_cents directly.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'products' AND column_name = 'price_per_kg_cents'
    ) THEN
        ALTER TABLE products RENAME COLUMN price_per_kg_cents TO unit_price_cents;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS receipts (
    id           TEXT PRIMARY KEY,
    tenant_id    TEXT NOT NULL REFERENCES tenants(id),
    user_id      TEXT NOT NULL REFERENCES users(id),
    status       TEXT NOT NULL,
    number       INTEGER,
    created_at   TIMESTAMPTZ NOT NULL,
    finalized_at TIMESTAMPTZ,
    sent_at      TIMESTAMPTZ,
    sent_to      TEXT
);
CREATE INDEX IF NOT EXISTS receipts_tenant_id_idx ON receipts(tenant_id);
CREATE INDEX IF NOT EXISTS receipts_user_id_status_idx ON receipts(user_id, status);
ALTER TABLE receipts ADD COLUMN IF NOT EXISTS sent_at TIMESTAMPTZ;
ALTER TABLE receipts ADD COLUMN IF NOT EXISTS sent_to TEXT;

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

CREATE TABLE IF NOT EXISTS payments (
    id                        TEXT PRIMARY KEY,
    tenant_id                 TEXT NOT NULL REFERENCES tenants(id),
    receipt_id                TEXT NOT NULL REFERENCES receipts(id),
    stripe_payment_intent_id  TEXT NOT NULL UNIQUE,
    amount_cents              INTEGER NOT NULL,
    status                    TEXT NOT NULL,
    created_at                TIMESTAMPTZ NOT NULL,
    updated_at                TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS payments_tenant_id_idx ON payments(tenant_id);
CREATE INDEX IF NOT EXISTS payments_receipt_id_idx ON payments(receipt_id);
