# Kafka event contracts

## Envelope

Every record value is JSON with `eventId`, `eventType`, `aggregateId`, `producer`, `occurredAt`, and `data`. The Kafka message key is the aggregate Order ID for all current events.

| Topic / event type | Producer | Consumer | Aggregate ID and key | Data fields |
|---|---|---|---|---|
| `order.created.v1` | Order | None in template | Order ID | Order creation snapshot |
| `payment.deposit_succeeded.v1` | Payment | Order group `order-service-payment-events` | Order ID | `paymentId`, `orderId`, `amountVnd`, `currency` |
| `order.status_changed.v1` | Order | None in template | Order ID | `orderId`, `previousStatus`, `currentStatus`, `description` |
| `package.received.v1` | Warehouse | Order group `order-service-warehouse-events` | Order ID | `packageId`, `orderId`, tracking/warehouse codes and dimensions |

`order.created.v1`, payment success, package received, and Order status changes are first persisted in each producer's `outbox_events` table. Workers poll unpublished rows and mark them after synchronous Kafka acknowledgement.

Delivery is at least once, not exactly once. Order disables auto-commit and commits the Kafka offset only after its handler transaction succeeds. That transaction locks the Order, validates the transition, updates state, appends a timeline row, inserts the event ID in `processed_events`, and creates `order.status_changed.v1`. A duplicate `eventId` is detected and treated as already processed.

Malformed envelopes are logged and skipped. The warehouse consumer also skips an invalid-contract poison event; transient database/processing errors retry until shutdown.
