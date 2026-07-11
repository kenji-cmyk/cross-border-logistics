# Cross-Border Logistics

This repository is an academic microservices template. Phase 2 implements the Quotation Service; the other services remain Phase 1 health-check skeletons.

## Quotation Service

The service validates products, obtains a mock exchange rate, calculates fees without floating-point arithmetic, persists quotations in `quotation_db`, and exposes:

- `POST /api/v1/quotations`
- `GET /api/v1/quotations/{quotationId}`
- `GET /internal/quotations/{quotationId}` (Docker-network only; not routed by Nginx)

Mock rates in VND are USD 26,000, CNY 3,600, JPY 175, and KRW 19. Restricted keywords (case-insensitive, checked in product name and URL) are `weapon`, `gun`, `explosive`, `battery-liquid`, and `dangerous-chemical`.

Source prices are represented internally as fixed-point integers with six decimal places. Calculated VND amounts are rounded to the nearest integer, the service fee is 5%, and estimated shipping is 120,000 VND.

## Run locally

```bash
docker compose up -d --build postgres quotation-service nginx
curl http://localhost/health
```

Create and retrieve a quotation:

```bash
curl -X POST http://localhost/api/v1/quotations \
  -H "Content-Type: application/json" \
  -d '{"customerId":"customer-001","productUrl":"https://example.com/product/123","productName":"Wireless Keyboard","sourcePrice":50,"currency":"USD","quantity":1}'

curl http://localhost/api/v1/quotations/<quotation-id>
```

See [API examples](docs/api-examples.md) for success and error examples.

## Current Phase 2 limitations

Rates and restriction rules are in-memory mocks. There is no authentication, real exchange-rate integration, scraping, Kafka business flow, or business logic for Order, Payment, Warehouse, or Admin. The deployment is a local/demo single-node stack, not a production HA design. Kafka UI on port 8088 is for local/demo environments only.
