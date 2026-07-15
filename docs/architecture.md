# Architecture

## System context

The broader domain includes Customer, Purchaser, Warehouse Staff, and Admin actors, plus external payment, exchange-rate, and domestic-delivery providers. The implemented sequence covers URL-led quotation extraction, explicit confirmation, 70% deposit and 30% balance payments through mock, direct SePay VietQR, or SePay hosted checkout, provider-verified callbacks, Kafka Order transitions, SSE notifications, warehouse receipt, and read-only Admin rates. Product extraction remains deterministic in demo mode; exchange rates can use either fixed offline values or Vietcombank's public reference feed.

## Container-level architecture

```mermaid
flowchart LR
    Client -->|HTTP/HTTPS| Caddy
    Caddy --> Frontend[Frontend Nginx]
    Frontend --> Gateway[API Gateway Nginx]
    Gateway --> Quotation
    Gateway --> Order
    Gateway --> Payment
    Gateway --> Warehouse
    Gateway --> Admin
    Gateway --> Notification
    Vietcombank --> Admin
    SePayBank[SePay bank webhook] -->|HTTPS| Caddy
    Payment --> SePayPG[SePay hosted checkout]
    SePayPG -->|HTTPS IPN| Caddy
    Quotation --> Admin
    Quotation --> QuotationDB[(quotation_db)]
    Order --> OrderDB[(order_db)]
    Payment --> PaymentDB[(payment_db)]
    Warehouse --> WarehouseDB[(warehouse_db)]
    Order --> Kafka
    Payment --> Kafka
    Warehouse --> Kafka
    Kafka --> Order
    Kafka --> Notification
    KafkaUI --> Kafka
```

Caddy is the only container publishing host ports 80 and 443. It terminates TLS, redirects public HTTP to HTTPS when a domain is configured, and proxies unchanged requests to frontend Nginx. Frontend Nginx serves React and forwards REST/SSE traffic to the API gateway; all other containers share the private Compose network.

## Service responsibilities

- Quotation owns allowlisted extraction, restriction checks, exchange-rate use, fee calculation, expiration, and idempotent Order confirmation.
- Order owns Order state, items, tracking timeline, idempotent consumers, and Order outbox events.
- Payment owns the shared 70% deposit and 30% balance lifecycle, provider selection, direct VietQR requests, hosted-checkout form generation, replay-safe webhook/IPN verification, and the resulting success outbox events.
- Notification consumes Order status events and exposes a bounded-replay SSE stream.
- Warehouse owns foreign-warehouse packages and their receipt outbox event.
- Admin exposes a validated rate snapshot and has no runtime database/Kafka dependency. In `vietcombank` mode it reads selling rates from the official XML feed, rounds them to the nearest VND, and caches the snapshot for at least five minutes. A stale successful snapshot is retained if a later refresh temporarily fails.

## Payment provider modes

The provider adapter changes only how a pending Payment is presented and how
success is authenticated. Persistence and downstream events stay provider
independent.

| Provider | Pending-payment presentation | Trusted completion path |
|---|---|---|
| `mock` | Local demo URL/action | Development-only mock-success request |
| `sepay` | Direct `vietqr.app` image using the configured bank account | HMAC-authenticated incoming-transfer webhook at `/api/v1/payments/sepay/webhook` |
| `sepay_pg` | Same-origin `GET /api/v1/payments/{paymentId}/checkout`, which returns an auto-submitting form for SePay | `X-Secret-Key`-authenticated IPN at `/api/v1/payments/sepay/pg/ipn` |

For `sepay_pg`, `SEPAY_PUBLIC_URL` is the public HTTPS origin used to construct
provider callback URLs. During local Sandbox development it can be a Cloudflare
Quick Tunnel pointing to Caddy on port 80. A server sets `PUBLIC_SITE_ADDRESS`
to its DNS name so Caddy obtains and renews the public certificate. Production
uses the same internal payment lifecycle with Production merchant credentials;
browser return URLs remain informational, while IPN is authoritative.

## Synchronous communication

```text
Order -> Quotation: GET /internal/quotations/{quotationId}, POST /internal/quotations/{quotationId}/confirm
Payment -> Order: GET /internal/orders/{orderId}/payment-summary
Warehouse -> Order: GET /internal/orders/{orderId}/warehouse-summary
Quotation -> Admin: GET /api/v1/admin/rates (private Compose network)
```

These internal endpoints are reachable inside the Docker network and are not routed by Nginx.

## Asynchronous communication

```mermaid
sequenceDiagram
    Payment->>PaymentDB: payment SUCCEEDED + outbox (one transaction)
    Payment->>Kafka: payment.deposit_succeeded.v1
    Kafka->>Order: order-service-payment-events
    Order->>OrderDB: WAITING_PURCHASE + timeline + processed event + outbox
    Payment->>Kafka: payment.remaining_balance_succeeded.v1
    Kafka->>Order: order-service-payment-events
    Order->>OrderDB: READY_FOR_DOMESTIC_DELIVERY + timeline + processed event + outbox
    Warehouse->>WarehouseDB: package + outbox (one transaction)
    Warehouse->>Kafka: package.received.v1
    Kafka->>Order: order-service-warehouse-events
    Order->>OrderDB: ARRIVED_FOREIGN_WAREHOUSE + timeline + processed event + outbox
```

Order also publishes `order.created.v1` and `order.status_changed.v1` from its outbox. Notification consumes status changes; Caddy and Nginx only route SSE and never consume Kafka.

## Data ownership

The single PostgreSQL container hosts `quotation_db`, `order_db`, `payment_db`, `warehouse_db`, and reserved `admin_db`. This is logical database-per-service ownership for a demo: each service receives credentials for and accesses only its own database; cross-service foreign keys do not exist.

## Reliability patterns

- Transactional Outbox avoids losing an event after committing business state.
- Delivery is at least once: publishing can occur again if marking an outbox row published fails.
- Order inserts event IDs in `processed_events` in the same transaction as status/timeline updates.
- Kafka auto-commit is disabled; consumers commit offsets after the database transaction succeeds.
- Transient handler failures retry. Duplicate events return without applying a second state change.
- HTTP servers shut down with a ten-second timeout; Kafka workers/clients and PostgreSQL pools close during coordinated service shutdown.

## Demo deployment

One EC2 instance runs Docker Compose: Caddy, two private Nginx layers, six Go services, one PostgreSQL container, one Kafka broker, and optional demo-only Kafka UI. Only Caddy TCP ports 80 and 443 are public. See [EC2 deployment](ec2-deployment.md).

## Target production architecture (future state only)

A production evolution would replace the single-node Caddy ingress with a managed load balancer, multiple service instances, managed multi-AZ PostgreSQL, a replicated Kafka cluster, secrets management, centralized logs/metrics/traces, object storage, and autoscaling. This repository does not implement or claim those properties.
