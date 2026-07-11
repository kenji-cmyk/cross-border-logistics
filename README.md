# Cross-Border Shopping and Logistics Microservices Template

## Project overview

This is an academic, runnable source-code template for a cross-border assisted-shopping system. It demonstrates one event-driven microservice vertical slice; it is not a complete production system.

## Implemented business flow

```text
Quotation -> Order (WAITING_DEPOSIT) -> Deposit Payment -> Kafka
-> Order (WAITING_PURCHASE) -> Foreign Warehouse Package -> Kafka
-> Order (ARRIVED_FOREIGN_WAREHOUSE) -> Tracking Timeline
```

## Architecture

Nginx is the public API gateway. Five Go services use a logical database-per-service model in one PostgreSQL container. Transactional outboxes publish to one Kafka broker, and Order consumers use `processed_events` for idempotency. Docker Compose runs the complete single-node demo. See [architecture](docs/architecture.md).

## Services

| Service | Responsibility / database | Public API | Kafka |
|---|---|---|---|
| Quotation | Validate products and calculate mock-rate quotations / `quotation_db` | `POST`, `GET /api/v1/quotations...` | None |
| Order | Create Orders, own status and timeline / `order_db` | `POST`, `GET /api/v1/orders...` | Produces `order.created.v1`, `order.status_changed.v1`; consumes payment/package events |
| Payment | Create and mock-complete deposits / `payment_db` | `/api/v1/payments...` | Produces `payment.deposit_succeeded.v1` |
| Warehouse | Receive and retrieve foreign packages / `warehouse_db` | `/api/v1/warehouse/packages...` | Produces `package.received.v1` |
| Admin | Read configuration-backed demo rates / no runtime database | `GET /api/v1/admin/rates` | None |

## Technology stack

Go 1.25, PostgreSQL 17 Alpine, Apache Kafka 3.9.1 (KRaft), Nginx 1.27 Alpine, Kafka UI 0.7.2, Docker Compose, `pgx/v5`, and `franz-go`. Exact Go dependencies are in `backend/go.mod`.

## Repository structure

```text
backend/{cmd,internal,pkg,migrations,deploy}/
scripts/                 demo, wait, reset, database initialization
docs/                    architecture, API, events, deployment, troubleshooting
compose.yaml             complete demo stack
Makefile                 validation and Compose shortcuts
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

If executable bits were not preserved by the transfer, run `chmod +x scripts/*.sh`. Gateway health is available at `http://localhost/health`.

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

- No authentication, RBAC, real payment gateway, product scraping, exchange-rate integration, domestic shipping API, or TLS in local Compose.
- One Kafka broker, one PostgreSQL container, and one single-node EC2 deployment; no production HA and no claim of supporting 2,000 concurrent users.
- Admin rates are configuration-backed and do not dynamically update Quotation.
- The demo intentionally transitions directly from `WAITING_PURCHASE` to `ARRIVED_FOREIGN_WAREHOUSE`.

## Production evolution

A future production system could add a load balancer, autoscaling, TLS, authentication, a secrets manager, managed PostgreSQL, a Kafka cluster, observability, object storage, and the complete Order workflow. None of those capabilities are implemented here.

See [troubleshooting](docs/troubleshooting.md) and the [final acceptance report](docs/final-acceptance-report.md).
