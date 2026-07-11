# Cross-Border Shopping and Logistics Microservices Template

## Project overview

This is an academic, runnable source-code template for a cross-border assisted-shopping system. It demonstrates one event-driven microservice vertical slice; it is not a complete production system.

## Implemented business flow

```text
Product URL -> Extracted Quotation -> Confirmed Order (WAITING_DEPOSIT)
-> Hosted Deposit -> Signed Callback -> Kafka -> SSE -> Order (WAITING_PURCHASE)
-> Foreign Warehouse Package -> Kafka
-> Order (ARRIVED_FOREIGN_WAREHOUSE) -> Tracking Timeline
```

## Architecture

The public Nginx container serves the React frontend and forwards browser API requests to an internal Nginx API gateway. Six Go services use a logical database-per-service model in one PostgreSQL container. Transactional outboxes publish to one Kafka broker, Order consumers use `processed_events` for idempotency, and Notification streams status changes through SSE. Docker Compose runs the complete single-node demo. See [architecture](docs/architecture.md).

## Services

| Service | Responsibility / database | Public API | Kafka |
|---|---|---|---|
| Quotation | Extract allowlisted product metadata and calculate quotations / `quotation_db` | `/api/v1/quotations...` | None |
| Order | Create Orders, own status and timeline / `order_db` | `POST`, `GET /api/v1/orders...` | Produces `order.created.v1`, `order.status_changed.v1`; consumes payment/package events |
| Payment | Create hosted deposits and verify signed callbacks / `payment_db` | `/api/v1/payments...` | Produces `payment.deposit_succeeded.v1` |
| Notification | Stream Order status changes / no database | `/api/v1/notifications...` | Consumes `order.status_changed.v1` |
| Warehouse | Receive and retrieve foreign packages / `warehouse_db` | `/api/v1/warehouse/packages...` | Produces `package.received.v1` |
| Admin | Read configuration-backed demo rates / no runtime database | `GET /api/v1/admin/rates` | None |

## Technology stack

Go 1.25, PostgreSQL 17 Alpine, Apache Kafka 3.9.1 (KRaft), Nginx 1.27 Alpine, Kafka UI 0.7.2, Docker Compose, `pgx/v5`, and `franz-go`. Exact Go dependencies are in `backend/go.mod`.

## Repository structure

```text
cross-border-logistics/
|-- backend/                         Go microservices
|   |-- cmd/                         Service entry points
|   |   |-- admin-service/
|   |   |-- order-service/
|   |   |-- payment-service/
|   |   |-- quotation-service/
|   |   `-- warehouse-service/
|   |-- internal/                    Service-specific business code
|   |   |-- admin/
|   |   |-- order/
|   |   |-- payment/
|   |   |-- quotation/
|   |   `-- warehouse/
|   |       |-- adapters/            HTTP, Kafka, PostgreSQL, and service clients
|   |       |-- application/         Use cases and application services
|   |       |-- domain/              Domain models and rules
|   |       `-- ports/               Input and output contracts
|   |-- migrations/                  Per-service database migrations
|   |-- pkg/                         Shared config, event, HTTP, Kafka, and DB packages
|   |-- deploy/nginx/                Internal API gateway configuration
|   |-- scripts/init-databases.sql   PostgreSQL database bootstrap
|   |-- Dockerfile
|   |-- go.mod
|   `-- Makefile
|-- frontend/                        React and TypeScript web application
|   |-- deploy/nginx.conf            Production frontend Nginx configuration
|   |-- src/
|   |   |-- components/              Landing-page and shared UI components
|   |   |-- hooks/                   Reusable React hooks
|   |   |-- lib/                     API client and utilities
|   |   |-- pages/                   Page-level components
|   |   |-- styles/                  Fonts, global styles, and theme tokens
|   |   |-- test/                    Test setup
|   |   `-- types/                   Shared TypeScript types
|   |-- Dockerfile
|   |-- package.json
|   |-- tailwind.config.js
|   `-- vite.config.ts
|-- docs/                            Architecture, API, events, deployment, and guides
|-- scripts/                         Demo, readiness, and reset scripts
|-- .env.example                     Environment variable template
|-- compose.yaml                     Complete local demo stack
|-- Makefile                         Validation and Compose shortcuts
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

If executable bits were not preserved by the transfer, run `chmod +x scripts/*.sh`. The UI is available at `http://localhost/`, frontend health at `/ui-health`, and gateway health at `/health`. Individual backend containers are not published to the host.

## Manual API demo

Copyable requests for the full flow, response examples, and polling commands are in [API examples](docs/api-examples.md). The automated equivalent is:

```bash
BASE_URL=http://localhost make demo
```

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
```

## Kafka UI

Kafka UI is at `http://localhost:8088` and can show topics, records, and consumer groups. **Kafka UI is for local/demo environments only.** Do not expose it publicly on EC2; restrict it to your IP or use an SSH tunnel.

## EC2 deployment

See the [single-instance EC2 deployment guide](docs/ec2-deployment.md).

## Stopping and cleaning

```bash
docker compose down       # keeps PostgreSQL volume data
docker compose down -v    # permanently deletes project volumes and PostgreSQL data
```

The safer scripted form is `make reset-demo`; destructive reset requires `./scripts/reset-demo.sh --delete-data` and confirmation.

## Current limitations

- Demo customer identity remains request-scoped; production must derive it from authentication. Real provider certification, broad marketplace scraping, domestic shipping API, and local TLS remain out of scope.
- One Kafka broker, one PostgreSQL container, and one single-node EC2 deployment; no production HA and no claim of supporting 2,000 concurrent users.
- Admin rates are configuration-backed and do not dynamically update Quotation.
- The demo intentionally transitions directly from `WAITING_PURCHASE` to `ARRIVED_FOREIGN_WAREHOUSE`.

## Production evolution

A future production system could add a load balancer, autoscaling, TLS, authentication, a secrets manager, managed PostgreSQL, a Kafka cluster, observability, object storage, and the complete Order workflow. None of those capabilities are implemented here.

See [troubleshooting](docs/troubleshooting.md) and the [final acceptance report](docs/final-acceptance-report.md).
