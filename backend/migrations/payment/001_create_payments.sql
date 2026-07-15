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

CREATE UNIQUE INDEX IF NOT EXISTS payments_one_remaining_balance_per_order_idx
    ON payments (order_id) WHERE type = 'REMAINING_BALANCE';

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

-- Older payment-service versions required provider_request_id for MoMo.
-- SePay uses provider_reference instead, so keep the legacy data but allow
-- new payments to omit the obsolete column when an existing volume is reused.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'payments'
          AND column_name = 'provider_request_id'
          AND is_nullable = 'NO'
    ) THEN
        ALTER TABLE payments ALTER COLUMN provider_request_id DROP NOT NULL;
    END IF;
END $$;
