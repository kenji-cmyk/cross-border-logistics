CREATE TABLE IF NOT EXISTS quotations (
    id UUID PRIMARY KEY,
    customer_id TEXT NOT NULL,
    product_url TEXT NOT NULL,
    product_name TEXT NOT NULL,
    source_price NUMERIC(24, 6) NOT NULL CHECK (source_price > 0),
    currency VARCHAR(3) NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    exchange_rate BIGINT NOT NULL,
    product_amount_vnd BIGINT NOT NULL,
    service_fee_vnd BIGINT NOT NULL,
    estimated_shipping_fee_vnd BIGINT NOT NULL,
    total_amount_vnd BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
