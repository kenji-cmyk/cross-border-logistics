CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    customer_id TEXT NOT NULL,
    quotation_id UUID NOT NULL UNIQUE,
    delivery_address TEXT NOT NULL,
    total_amount_vnd BIGINT NOT NULL CHECK (total_amount_vnd > 0),
    deposit_amount_vnd BIGINT NOT NULL CHECK (deposit_amount_vnd >= 0),
    remaining_amount_vnd BIGINT NOT NULL CHECK (remaining_amount_vnd >= 0),
    status VARCHAR(48) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_name TEXT NOT NULL,
    product_url TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    unit_price_vnd BIGINT NOT NULL CHECK (unit_price_vnd >= 0),
    total_price_vnd BIGINT NOT NULL CHECK (total_price_vnd > 0),
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS tracking_events (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status VARCHAR(48) NOT NULL,
    description TEXT NOT NULL,
    source TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS tracking_events_order_timeline_idx
    ON tracking_events (order_id, occurred_at, created_at);

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

CREATE TABLE IF NOT EXISTS processed_events (
    event_id UUID PRIMARY KEY,
    event_type VARCHAR(100) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL
);
