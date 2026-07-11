# Cross-Border Logistics

This repository is an academic microservices template. Phase 3 implements the Quotation and Order Services; Payment, Warehouse, and Admin remain Phase 1 health-check skeletons.

## Quotation Service

The service validates products, obtains a mock exchange rate, calculates fees without floating-point arithmetic, persists quotations in `quotation_db`, and exposes:

- `POST /api/v1/quotations`
- `GET /api/v1/quotations/{quotationId}`
- `GET /internal/quotations/{quotationId}` (Docker-network only; not routed by Nginx)

Mock rates in VND are USD 26,000, CNY 3,600, JPY 175, and KRW 19. Restricted keywords (case-insensitive, checked in product name and URL) are `weapon`, `gun`, `explosive`, `battery-liquid`, and `dangerous-chemical`.

Source prices are represented internally as fixed-point integers with six decimal places. Calculated VND amounts are rounded to the nearest integer, the service fee is 5%, and estimated shipping is 120,000 VND.

## Order Service

The Order Service creates an order from a `PENDING_CONFIRMATION` quotation owned by the requesting customer. It calculates a 70% deposit, persists the order and item, creates the initial tracking event, and stores an `order.created.v1` envelope in its transactional outbox. Phase 3 intentionally does not publish that outbox event to Kafka.

It exposes:

- `POST /api/v1/orders`
- `GET /api/v1/orders/{orderId}`
- `GET /api/v1/orders/{orderId}/timeline`
- `GET /internal/orders/{orderId}/payment-summary` (Docker-network only; not routed by Nginx)

Creating a second order for the same quotation returns `409 CONFLICT`. The Order Service reads quotation data only through `GET /internal/quotations/{quotationId}` and never accesses `quotation_db` directly.

## Run locally

```bash
docker compose up -d --build postgres quotation-service order-service nginx
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

See [API examples](docs/api-examples.md) for success and error examples.

## Current Phase 3 limitations

Rates and restriction rules are in-memory mocks. There is no authentication, real exchange-rate integration, scraping, Payment/Warehouse/Admin business logic, Kafka producer or consumer, or outbox publishing worker. Order status changes driven by payment and warehouse events begin in Phase 5 and Phase 6. The deployment is a local/demo single-node stack, not a production HA design. Kafka UI on port 8088 is for local/demo environments only.
