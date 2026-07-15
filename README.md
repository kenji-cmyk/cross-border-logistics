# Cross-Border Shopping and Logistics Microservices Template

## Project overview

This is an academic, runnable source-code template for a cross-border assisted-shopping system. It demonstrates one event-driven microservice vertical slice; it is not a complete production system.

## Implemented business flow

```text
Product URL -> Extracted Quotation -> Confirmed Order (WAITING_DEPOSIT)
-> Deposit (70% via mock, direct VietQR, or hosted SePay checkout)
-> Verified webhook/IPN -> Kafka -> SSE -> Order (WAITING_PURCHASE)
-> Foreign Warehouse Package -> Kafka
-> Order (ARRIVED_FOREIGN_WAREHOUSE) -> Tracking Timeline

When the Order later reaches `WAITING_REMAINING_PAYMENT`, the same pipeline creates
a SePay VietQR request for the remaining 30% and moves the Order to
`READY_FOR_DOMESTIC_DELIVERY` after verified payment.
```

## Architecture

Caddy is the only public ingress. It serves local HTTP by default and enables automatic HTTPS when configured with a public domain, then proxies every request to the private frontend Nginx container. Frontend Nginx serves React and forwards browser API requests to the internal Nginx API gateway. Six Go services use a logical database-per-service model in one PostgreSQL container. Transactional outboxes publish to one Kafka broker, Order consumers use `processed_events` for idempotency, and Notification streams status changes through SSE. Admin caches Vietcombank reference selling rates, while Quotation reads that same Admin snapshot before calculating a quotation. Docker Compose runs the complete single-node demo. See [architecture](docs/architecture.md).

## Services

| Service | Responsibility / database | Public API | Kafka |
|---|---|---|---|
| Quotation | Extract allowlisted product metadata and calculate quotations / `quotation_db` | `/api/v1/quotations...` | None |
| Order | Create Orders, own status and timeline / `order_db` | `POST`, `GET /api/v1/orders...` | Produces `order.created.v1`, `order.status_changed.v1`; consumes payment/package events |
| Payment | Create mock, direct SePay VietQR, or SePay hosted-checkout payments and verify provider callbacks / `payment_db` | `/api/v1/payments...` | Produces deposit and remaining-balance success events |
| Notification | Stream Order status changes / no database | `/api/v1/notifications...` | Consumes `order.status_changed.v1` |
| Warehouse | Receive and retrieve foreign packages / `warehouse_db` | `/api/v1/warehouse/packages...` | Produces `package.received.v1` |
| Admin | Cache Vietcombank selling rates or serve fixed offline rates / no runtime database | `GET /api/v1/admin/rates` | None |

## Technology stack

Go 1.25, PostgreSQL 17 Alpine, Apache Kafka 3.9.1 (KRaft), Caddy 2.11.4 Alpine, Nginx 1.27 Alpine, Kafka UI 0.7.2, Docker Compose, `pgx/v5`, and `franz-go`. Exact Go dependencies are in `backend/go.mod`.

## Repository structure

```text
cross-border-logistics/
|-- backend/                              Go microservices
|   |-- cmd/                              Service composition roots
|   |   |-- admin-service/
|   |   |-- notification-service/
|   |   |-- order-service/
|   |   |-- payment-service/
|   |   |-- quotation-service/
|   |   `-- warehouse-service/
|   |-- internal/                         Service-specific code
|   |   |-- admin/
|   |   |   |-- adapters/
|   |   |   |   |-- config/              Fixed/offline rate provider
|   |   |   |   |-- http/                Admin HTTP handler
|   |   |   |   `-- vietcombank/         Live XML provider and cache
|   |   |   |-- application/
|   |   |   |-- domain/
|   |   |   `-- ports/
|   |   |-- quotation/
|   |   |   |-- adapters/
|   |   |   |   |-- extraction/          Routing, metadata parsing, and SSRF safety
|   |   |   |   |-- http/                Public/internal quotation handlers
|   |   |   |   |-- postgres/            Quotation persistence
|   |   |   |   |-- admin_rates.go       Cached Admin snapshot client
|   |   |   |   `-- product_extractor.go Demo product extractor
|   |   |   |-- application/
|   |   |   |-- domain/
|   |   |   `-- ports/
|   |   |-- order/                        Order, timeline, Kafka consumers, outbox
|   |   |-- payment/                      Deposits, callbacks, and payment outbox
|   |   `-- warehouse/                    Foreign package receipt and outbox
|   |-- migrations/                       Per-service database migrations
|   |-- pkg/                              Shared config, event, HTTP, Kafka, logging, DB
|   |-- deploy/nginx/                     Internal API gateway configuration
|   |-- scripts/init-databases.sql        PostgreSQL database bootstrap
|   |-- Dockerfile
|   |-- go.mod
|   |-- go.sum
|   `-- Makefile
|-- frontend/                             React and TypeScript web application
|   |-- deploy/nginx.conf                 Production frontend Nginx configuration
|   |-- src/
|   |   |-- app/                          Providers and application layout
|   |   |-- components/                   Landing-page and shared UI components
|   |   |-- features/                     Frontend API feature boundary
|   |   |-- hooks/                        Reusable React hooks
|   |   |-- lib/                          API client, formatting, and storage
|   |   |-- pages/                        Page-level components
|   |   |-- styles/                       Fonts, globals, and theme tokens
|   |   |-- test/                         Test setup
|   |   `-- types/                        Runtime-validated API types
|   |-- Dockerfile
|   |-- package.json
|   |-- package-lock.json
|   |-- tailwind.config.js
|   `-- vite.config.ts
|-- deploy/caddy/                         Public HTTP/HTTPS ingress configuration
|-- docs/
|   |-- api/                              ApiDog/Postman manual-test collection
|   |-- api-examples.md
|   |-- architecture.md
|   |-- CODEX_IMPLEMENTATION_PLAN.md
|   |-- ec2-deployment.md
|   |-- final-acceptance-report.md
|   |-- FRONTEND_IMPLEMENTATION_PLAN.md
|   |-- kafka-events.md
|   |-- SEQUENCE_ALIGNMENT_REFACTOR_PLAN.md
|   `-- troubleshooting.md
|-- scripts/                              Demo, E2E, readiness, and reset scripts
|-- .env.example                          Environment variable template
|-- compose.yaml                          Complete local demo stack
|-- Makefile                              Validation and Compose shortcuts
`-- README.md
```

## Prerequisites

Docker with the Compose plugin and Git are enough to run the stack. The automated demo also needs `curl`, `jq`, and Bash. Local development additionally needs Go 1.25.

## Quick start

```bash
git clone <repository-url> # replace with the real repository URL
cd cross-border-logistics
cp .env.example .env
docker compose up -d --build
docker compose ps
make demo
```

If executable bits were not preserved by the transfer, run `chmod +x scripts/*.sh`. With the default `PUBLIC_SITE_ADDRESS=:80`, Caddy keeps the UI available at `http://localhost/`, frontend health at `/ui-health`, and gateway health at `/health`. Frontend Nginx and every backend container remain private to the Compose network.

Public ingress settings are `PUBLIC_SITE_ADDRESS`, `PUBLIC_HTTP_PORT`, and
`PUBLIC_HTTPS_PORT`. Set a real DNS name without a scheme as
`PUBLIC_SITE_ADDRESS` to enable Caddy-managed HTTPS; keep `:80` for local HTTP.

The default Compose configuration requires outbound HTTPS access from `admin-service` to load Vietcombank exchange rates. For a deterministic offline run, set `EXCHANGE_RATE_PROVIDER=fixed` in `.env` before starting the stack.

## Exchange rates

Docker Compose defaults to `EXCHANGE_RATE_PROVIDER=vietcombank`. Admin fetches the [official Vietcombank XML feed](https://portal.vietcombank.com.vn/Usercontrols/TVPortal.TyGia/pXML.aspx) and uses the `Sell` value because the quotation represents buying foreign currency to pay for imported goods. Decimal values are rounded to the nearest whole VND to match the existing quotation contract.

```text
Vietcombank XML feed
        |
        v
Admin cached snapshot ----> GET /api/v1/admin/rates
        |
        v
Quotation calculation ----> persisted exchangeRate and VND totals
```

The snapshot is cached for at least five minutes. If a refresh fails after a successful load, Admin returns the last successful snapshot; if the initial load fails, the rates endpoint returns an error and Quotation maps the dependency failure to `502 EXCHANGE_RATE_UNAVAILABLE`. This prevents silently calculating a quotation with invented or unexpectedly stale fixed values.

Important environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `EXCHANGE_RATE_PROVIDER` | `vietcombank` in Compose | `vietcombank` for external rates or `fixed` for offline mode |
| `EXCHANGE_RATE_FETCH_TIMEOUT` | `2s` | Total timeout for the Vietcombank request |
| `EXCHANGE_RATE_CACHE_TTL` | `5m` | Cache and failed-refresh backoff; values below five minutes are rejected |
| `VIETCOMBANK_EXCHANGE_RATE_URL` | Official XML endpoint | Override for controlled testing or provider endpoint changes |
| `ADMIN_RATES_URL` | `http://admin-service:8080/api/v1/admin/rates` | Internal snapshot URL used by Quotation |
| `ADMIN_EXCHANGE_RATE_*` | Currency-specific defaults | Values used only in `fixed` mode |

Inspect the active snapshot with:

```bash
curl -sS http://localhost/api/v1/admin/rates | jq
```

The supported source currencies remain controlled by `ADMIN_SUPPORTED_CURRENCIES` and default to `USD,CNY,JPY,KRW`.

## Manual API demo

Copyable requests for the full flow, response examples, and polling commands are in [API examples](docs/api-examples.md). The automated equivalent is:

```bash
BASE_URL=http://localhost make demo
```

## Payment providers

Payment provider selection is additive; the deposit/remaining-balance domain,
database, outbox, Kafka, and SSE behavior are shared by all three modes.

| `PAYMENT_PROVIDER` | Customer experience | Authoritative success signal |
|---|---|---|
| `mock` | Offline local demo | Development-only mock-success endpoint |
| `sepay` | Direct bank-transfer VietQR | Incoming-transaction HMAC webhook |
| `sepay_pg` | SePay-hosted Sandbox or Production checkout | Payment Gateway IPN |

The default development profile is `PAYMENT_PROVIDER=mock`, so the repository
can run without payment credentials.

### Direct SePay VietQR

Use `PAYMENT_PROVIDER=sepay` when the merchant owns the destination bank account
and wants the application to render a VietQR directly:

```dotenv
PAYMENT_PROVIDER=sepay
SEPAY_BANK_CODE=MBBank
SEPAY_ACCOUNT_NUMBER=0123456789
SEPAY_ACCOUNT_HOLDER=YOUR ACCOUNT NAME
SEPAY_PAYMENT_CODE_PREFIX=CBL
SEPAY_QR_BASE_URL=https://vietqr.app/img
SEPAY_WEBHOOK_SECRET=replace-with-a-long-random-secret
```

In the SePay dashboard:

1. Enable payment-code recognition with prefix `CBL`, a fixed 12-character
   alphanumeric suffix, matching `SEPAY_PAYMENT_CODE_PREFIX`.
2. Create an incoming-transaction webhook using HMAC-SHA256 and the same secret.
3. Point it to `https://your-domain/api/v1/payments/sepay/webhook`, select the
   configured bank account, and enable the payment-code filter.

The webhook handler validates the raw-body signature and timestamp, destination
account, incoming transfer type, unique SePay transaction ID, payment code, and
exact VND amount before changing state. A valid retry is idempotent and returns
the SePay response contract `{"success":true}`. See the official
[SePay webhook integration](https://developer.sepay.vn/vi/sepay-webhooks/tich-hop-webhook),
[HMAC authentication](https://developer.sepay.vn/vi/sepay-webhooks/xac-thuc), and
[payment-code configuration](https://developer.sepay.vn/vi/sepay-webhooks/cau-hinh-ma-thanh-toan).

### SePay Payment Gateway hosted checkout

Use `PAYMENT_PROVIDER=sepay_pg` to send the customer to SePay's hosted checkout.
This mode does not use `SEPAY_BANK_CODE`, `SEPAY_ACCOUNT_NUMBER`,
`SEPAY_ACCOUNT_HOLDER`, `SEPAY_QR_BASE_URL`, or `SEPAY_WEBHOOK_SECRET`.

```dotenv
PAYMENT_PROVIDER=sepay_pg
SEPAY_PG_ENV=sandbox
SEPAY_PG_MERCHANT_ID=your-sandbox-merchant-id
SEPAY_PG_SECRET_KEY=your-sandbox-secret-key
SEPAY_PUBLIC_URL=https://your-public-origin
```

For a pending payment, `paymentUrl` points to the same-origin route
`GET /api/v1/payments/{paymentId}/checkout`. That route returns an
auto-submitting HTML form which POSTs the signed checkout fields to SePay. The
browser return is informational; only a valid IPN can mark the payment
`SUCCEEDED`.

SePay must be able to reach this IPN URL:

```text
https://your-public-origin/api/v1/payments/sepay/pg/ipn
```

For local Sandbox testing, expose Caddy's local HTTP port with a quick Cloudflare Tunnel and keep
that terminal running:

```powershell
cloudflared tunnel --url http://localhost:80
```

Copy the generated `https://*.trycloudflare.com` origin into
`SEPAY_PUBLIC_URL` and the SePay Sandbox IPN configuration, then recreate
`payment-service`:

```powershell
docker compose up -d --force-recreate payment-service
```

Quick Tunnel URLs change whenever a new tunnel is created.

For a stable server deployment, configure Caddy and SePay with the same origin:

```dotenv
PUBLIC_SITE_ADDRESS=cross-border-logistics.duckdns.org
PUBLIC_HTTP_PORT=80
PUBLIC_HTTPS_PORT=443
SEPAY_PUBLIC_URL=https://cross-border-logistics.duckdns.org
```

Keep TCP 80 open for ACME validation and HTTP-to-HTTPS redirects, expose TCP
443, and configure SePay's IPN URL as
`https://cross-border-logistics.duckdns.org/api/v1/payments/sepay/pg/ipn`.
Certificates and private keys persist in the `caddy-data` volume.

To switch from Sandbox to Production, set `SEPAY_PG_ENV=production`, replace the
merchant ID and secret with Production credentials, use a stable public HTTPS
origin for `SEPAY_PUBLIC_URL`, and configure the matching Production IPN URL.
Never reuse or commit Sandbox/Production secrets. The payment domain and public
application routes do not otherwise change.

## Kafka event flow

Payment and Warehouse commit business state and an event envelope in one PostgreSQL transaction. Outbox workers publish pending records. Order consumers process at least once, update Order/timeline and insert the event ID into `processed_events` in one transaction, then commit the Kafka offset. Duplicate delivery therefore does not duplicate the state change. See [Kafka event contracts](docs/kafka-events.md).

## Database ownership

Each service connects only to its own logical database. Cross-service reads use internal REST endpoints; there are no cross-service foreign keys or direct database reads.

## Testing

```bash
make fmt
make test
make vet
make build
docker compose config
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/sequence-e2e.ps1 -IncludeDependencyFailure
```

The sequence E2E runner executes 25 public-API and infrastructure scenarios, including validation and SSRF rejection, quotation latency, concurrent idempotency, signed webhook replay, database/outbox invariants, Kafka duplicate delivery, SSE reconnect, warehouse regression, and exchange-rate dependency failure with automatic service restoration.

## Kafka UI

Kafka UI is at `http://localhost:8088` and can show topics, records, and consumer groups. **Kafka UI is for local/demo environments only.** Do not expose it publicly on EC2; restrict it to your IP or use an SSH tunnel.

## EC2 deployment

See the [single-instance EC2 deployment guide](docs/ec2-deployment.md).

## Stopping and cleaning

```bash
docker compose down       # keeps PostgreSQL volume data
docker compose down -v    # permanently deletes PostgreSQL data and Caddy certificates
```

The safer scripted form is `make reset-demo`; destructive reset requires `./scripts/reset-demo.sh --delete-data` and confirmation.

## Current limitations

- Demo customer identity remains request-scoped; production must derive it from authentication. Real provider certification, broad marketplace scraping, domestic shipping API, and locally trusted HTTPS remain out of scope.
- One Kafka broker, one PostgreSQL container, and one single-node EC2 deployment; no production HA and no claim of supporting 2,000 concurrent users.
- Vietcombank rates are reference selling rates cached for at least five minutes, not guaranteed executable foreign-exchange quotes. The provider has no SLA or secondary live-provider failover in this demo.
- The demo intentionally transitions directly from `WAITING_PURCHASE` to `ARRIVED_FOREIGN_WAREHOUSE`.

## Production evolution

A future production system could replace the single-node Caddy ingress with a managed load balancer, then add autoscaling, authentication, a secrets manager, managed PostgreSQL, a Kafka cluster, observability, object storage, and the complete Order workflow. None of those capabilities are implemented here.

See [troubleshooting](docs/troubleshooting.md) and the [final acceptance report](docs/final-acceptance-report.md).
