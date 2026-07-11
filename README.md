# Cross-Border Logistics

This repository is an academic microservices template. Phase 6 implements the Quotation, Order, Payment, and foreign Warehouse Services with transactional outboxes and Kafka-driven Order updates. Admin remains a health-check skeleton.

## Quotation Service

The service validates products, obtains a mock exchange rate, calculates fees without floating-point arithmetic, persists quotations in `quotation_db`, and exposes:

- `POST /api/v1/quotations`
- `GET /api/v1/quotations/{quotationId}`
- `GET /internal/quotations/{quotationId}` (Docker-network only; not routed by Nginx)

Mock rates in VND are USD 26,000, CNY 3,600, JPY 175, and KRW 19. Restricted keywords (case-insensitive, checked in product name and URL) are `weapon`, `gun`, `explosive`, `battery-liquid`, and `dangerous-chemical`.

Source prices are represented internally as fixed-point integers with six decimal places. Calculated VND amounts are rounded to the nearest integer, the service fee is 5%, and estimated shipping is 120,000 VND.

## Order Service

The Order Service creates an order from a `PENDING_CONFIRMATION` quotation owned by the requesting customer. It calculates a 70% deposit, persists the order and item, creates the initial tracking event, and stores an `order.created.v1` envelope in its transactional outbox. Its outbox worker publishes pending events to Kafka.

It exposes:

- `POST /api/v1/orders`
- `GET /api/v1/orders/{orderId}`
- `GET /api/v1/orders/{orderId}/timeline`
- `GET /internal/orders/{orderId}/payment-summary` (Docker-network only; not routed by Nginx)
- `GET /internal/orders/{orderId}/warehouse-summary` (Docker-network only; not routed by Nginx)

Creating a second order for the same quotation returns `409 CONFLICT`. The Order Service reads quotation data only through `GET /internal/quotations/{quotationId}` and never accesses `quotation_db` directly.

## Payment Service

The Payment Service reads the required deposit and current order status through the Order Service internal payment-summary API, then stores one pending deposit payment per order in `payment_db`. It exposes:

- `POST /api/v1/payments/deposits`
- `GET /api/v1/payments/{paymentId}`
- `POST /api/v1/payments/{paymentId}/mock-success`

Mock success atomically changes a payment from `PENDING` to `SUCCEEDED` and inserts a full `payment.deposit_succeeded.v1` envelope into the Payment transactional outbox. Repeating mock success is idempotent and does not create another outbox event. The outbox publishes asynchronously; Order Service consumes idempotently and advances the Order to `WAITING_PURCHASE`.

## Warehouse Service

`POST /api/v1/warehouse/packages/receive` validates an Order through the internal warehouse-summary API, accepts only `WAITING_PURCHASE`, and atomically stores a Package plus `package.received.v1` in `warehouse_db`. The request returns after that database commit and does not wait for Kafka processing. A warehouse outbox worker publishes with the Order ID as the Kafka key. Order Service consumes through group `order-service-warehouse-events`, then atomically updates the Order, adds the Vietnamese tracking description, records `processed_events`, and creates `order.status_changed.v1` in its own outbox.

For the source-code template, `package.received.v1` advances an Order directly from `WAITING_PURCHASE` to `ARRIVED_FOREIGN_WAREHOUSE`. In a production workflow, `PURCHASED` and `IN_TRANSIT_TO_FOREIGN_WAREHOUSE` would normally occur first.

Source tracking numbers are trimmed, uppercased, and globally unique in this simplified template. Physical measurements use `float64` in Go and PostgreSQL `NUMERIC(10,3)` for kilograms / `NUMERIC(10,2)` for centimetres; all must be positive and no greater than 500.

## Run locally

```bash
docker compose up -d --build postgres quotation-service order-service payment-service nginx
curl http://localhost/health
```

Create and retrieve a quotation:

```bash
curl -X POST http://localhost/api/v1/quotations \
  -H "Content-Type: application/json" \
  -d '{"customerId":"customer-001","productUrl":"https://example.com/product/123","productName":"Wireless Keyboard","sourcePrice":50,"currency":"USD","quantity":1}'

curl http://localhost/api/v1/quotations/<quotation-id>
```

Create an order and read its timeline:

```bash
curl -X POST http://localhost/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{"quotationId":"<quotation-id>","customerId":"customer-001","deliveryAddress":"Thu Duc City, Ho Chi Minh City"}'

curl http://localhost/api/v1/orders/<order-id>/timeline
```

Create, retrieve, and simulate a successful deposit payment:

```bash
curl -X POST http://localhost/api/v1/payments/deposits \
  -H "Content-Type: application/json" \
  -d '{"orderId":"<order-id>"}'

curl http://localhost/api/v1/payments/<payment-id>
curl -X POST http://localhost/api/v1/payments/<payment-id>/mock-success
```

After the payment event advances the Order, receive a package:

```bash
curl -X POST http://localhost/api/v1/warehouse/packages/receive \
  -H "Content-Type: application/json" \
  -d '{"orderId":"<order-id>","sourceTrackingNumber":"CN123456789","warehouseCode":"CN-GZ-01","weightKg":1.4,"lengthCm":30,"widthCm":20,"heightCm":15}'
```

See [API examples](docs/api-examples.md) for success and error examples.

## Current Phase 6 limitations

Rates, restriction rules, and payment processing are mocks. There is no authentication, real exchange-rate integration, scraping, shipment consolidation, domestic warehouse flow, remaining-payment flow, delivery integration, or Admin business logic. The deployment uses one Kafka broker and one PostgreSQL container and is a local/demo single-node stack, not production HA. Local Compose has no TLS and does not demonstrate 2,000 concurrent users. Kafka UI on port 8088 is for local/demo environments only.
