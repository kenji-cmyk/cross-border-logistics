CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL,
    type VARCHAR(32) NOT NULL,
    amount_vnd BIGINT NOT NULL CHECK (amount_vnd > 0),
    currency VARCHAR(3) NOT NULL,
    status VARCHAR(24) NOT NULL,
    payment_url TEXT NOT NULL,
    provider_reference TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS payments_one_deposit_per_order_idx
    ON payments (order_id) WHERE type = 'DEPOSIT';

CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NULL
);

CREATE INDEX IF NOT EXISTS outbox_events_unpublished_idx
    ON outbox_events (created_at) WHERE published_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS payments_provider_reference_idx ON payments(provider_reference);
CREATE TABLE IF NOT EXISTS provider_callbacks (
    event_id TEXT PRIMARY KEY,
    payment_id UUID NOT NULL,
    received_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE payments ADD COLUMN IF NOT EXISTS provider TEXT NOT NULL DEFAULT 'mock';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS provider_request_id TEXT;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS provider_transaction_id TEXT;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS result_code INTEGER;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS result_message TEXT NOT NULL DEFAULT '';
ALTER TABLE payments ADD COLUMN IF NOT EXISTS succeeded_at TIMESTAMPTZ;
ALTER TABLE payments ADD COLUMN IF NOT EXISTS failed_at TIMESTAMPTZ;
UPDATE payments SET provider_request_id=id::text WHERE provider_request_id IS NULL;
ALTER TABLE payments ALTER COLUMN provider_request_id SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS payments_provider_request_id_idx ON payments(provider_request_id);
CREATE UNIQUE INDEX IF NOT EXISTS payments_one_type_per_order_idx ON payments(order_id,type) WHERE type IN ('DEPOSIT','REMAINING_BALANCE');

CREATE TABLE IF NOT EXISTS refunds (
    id UUID PRIMARY KEY,
    payment_id UUID NOT NULL REFERENCES payments(id),
    order_id UUID NOT NULL,
    amount_vnd BIGINT NOT NULL CHECK (amount_vnd > 0),
    status VARCHAR(24) NOT NULL,
    provider TEXT NOT NULL,
    provider_request_id TEXT NOT NULL UNIQUE,
    provider_transaction_id TEXT,
    result_code INTEGER,
    result_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    succeeded_at TIMESTAMPTZ,
    failed_at TIMESTAMPTZ,
    UNIQUE(payment_id)
);
