CREATE TABLE IF NOT EXISTS packages (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL,
    source_tracking_number TEXT NOT NULL UNIQUE,
    warehouse_code TEXT NOT NULL,
    weight_kg NUMERIC(10,3) NOT NULL CHECK (weight_kg > 0),
    length_cm NUMERIC(10,2) NOT NULL CHECK (length_cm > 0),
    width_cm NUMERIC(10,2) NOT NULL CHECK (width_cm > 0),
    height_cm NUMERIC(10,2) NOT NULL CHECK (height_cm > 0),
    status VARCHAR(64) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_packages_order_id ON packages(order_id);
CREATE INDEX IF NOT EXISTS idx_packages_warehouse_code ON packages(warehouse_code);
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
CREATE INDEX IF NOT EXISTS idx_warehouse_outbox_unpublished ON outbox_events(created_at) WHERE published_at IS NULL;
