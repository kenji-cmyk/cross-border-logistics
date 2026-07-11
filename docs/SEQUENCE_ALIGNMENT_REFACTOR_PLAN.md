# Sequence Alignment Audit and Refactor Plan

## 1. Purpose

This document is the execution contract for an AI Agent refactoring this repository to match `sequence.png` (Google Drive file ID `1WO_qtsaaNksiaWeMiOFIUiZ8070pFD4b`). It records the current gaps, resolves ambiguous architecture choices, and divides the work into independently verifiable phases.

The target sequence has three main slices:

1. Extract product information and create a quotation in under 2.5 seconds.
2. Confirm a quotation and create an Order.
3. Create and complete the first deposit payment, update the Order, and notify the client.

This is a refactor plan, not evidence that the target behavior is already implemented.

## 2. Rules for every AI Agent

- Read this file, `docs/architecture.md`, `docs/kafka-events.md`, `docs/api-examples.md`, and the relevant code before editing.
- Treat one phase as a hard scope boundary. Do not implement later phases early.
- Keep root-level files for orchestration and documentation; keep Go application code under `backend/`.
- Preserve the current Clean/Hexagonal structure: `domain`, `application`, `ports`, and `adapters`.
- Reuse `backend/pkg/config`, `logger`, `httpx`, `postgres`, `event`, and `kafka` instead of duplicating infrastructure.
- Each service may access only its own database. Never add cross-service foreign keys or direct database reads.
- State changes and outbox records that belong together must be committed in one database transaction.
- Kafka consumers must remain idempotent. Do not enable automatic offset commits.
- Do not log raw card data, gateway secrets, webhook secrets, tokens, or full sensitive payloads.
- Maintain backward compatibility only where a phase explicitly requires it. Mark compatibility endpoints deprecated and remove them only in the final cleanup phase.
- After each phase run formatting, unit/integration tests, vet, build, Compose validation, and the phase acceptance scenario.
- Update the API/event/architecture docs in the same phase as the contract change.

## 3. Current-state verdict

Overall status: **partially aligned**. The repository implements a reliable demo vertical slice, but it does not yet implement the sequence as drawn.

| Sequence area | Current implementation | Verdict |
|---|---|---|
| API Gateway routes public calls | Nginx routes quotation, order, payment, warehouse, and admin APIs | Aligned |
| `POST /api/v1/quotations/extract` with Product URL | Only `POST /api/v1/quotations`; caller must supply customer, URL, product name, price, currency, and quantity | Missing |
| Automatic product extraction | No extraction port/adapter; product data is trusted from the request | Missing |
| Product restriction rejection | Mock keyword checker returns `400` for restricted name/URL | Partially aligned |
| External exchange-rate call with 2-second timeout and 3 retries | In-process fixed rate map; no external HTTP call or retry policy | Missing |
| Quotation response latency below 2.5 seconds | No explicit end-to-end deadline, latency metric, or acceptance test | Missing |
| `POST /api/v1/orders` using `quotationId` | Implemented, but also requires `customerId` and `deliveryAddress` | Partially aligned |
| Confirm quotation while creating Order | Order accepts `PENDING_CONFIRMATION`; Quotation remains unchanged | Missing and creates ambiguous ownership |
| Create Order and OrderDetail atomically | Order and items are created in one transaction with timeline and outbox | Aligned |
| Initial Order status “waiting for payment” | `WAITING_DEPOSIT` plus initial tracking event | Semantically aligned |
| Publish `OrderCreated(OrderID)` | Transactional `order.created.v1` outbox event | Aligned |
| `POST /api/v1/payments/deposit` | Current endpoint is plural: `/api/v1/payments/deposits` | Contract mismatch |
| Tokenize/request transaction at payment gateway | Only a locally generated mock URL/reference | Missing |
| Redirect without raw card handling | Mock URL is returned and no card data is stored | Partially aligned |
| `POST /api/v1/payments/callback` webhook | Only `POST /api/v1/payments/{id}/mock-success` | Missing |
| Verify webhook signature | Not implemented | Missing |
| Mark payment paid and update Order | Payment becomes `SUCCEEDED`, then Kafka moves Order to `WAITING_PURCHASE` | Semantically aligned via async event |
| Publish Order status change | Transactional `order.status_changed.v1` outbox event | Aligned |
| Gateway consumes event and pushes SSE/WebSocket notification | Nginx is only a reverse proxy; no notification service or stream endpoint | Missing |
| Natural-language timeline | Timeline exists, but two descriptions are English while the diagram expects natural Vietnamese UI text | Partially aligned |

## 4. Important architecture conflict

Step 27 in the diagram shows Payment Service calling Order Service using `gRPC / PUT`, while the current repository deliberately uses REST for synchronous calls, Kafka for cross-service state propagation, and a single source of Order state mutation.

The refactor must use this canonical rule:

```text
Payment callback
  -> Payment Service verifies signature
  -> Payment Service atomically marks Payment SUCCEEDED and writes payment.deposit_succeeded.v1 to outbox
  -> Kafka
  -> Order Service idempotently changes WAITING_DEPOSIT to WAITING_PURCHASE,
     appends TrackingEvent, and writes order.status_changed.v1 to outbox
```

Do **not** also call Order synchronously for the same state transition. A direct PUT plus Kafka would create two writers, races, and duplicate transition paths. If exact gRPC parity is a non-negotiable external requirement, create and approve an ADR before Phase 5; do not add gRPC opportunistically.

## 5. Target contracts

### 5.1 Quotation extraction

```http
POST /api/v1/quotations/extract
Content-Type: application/json

{
  "customerId": "customer-001",
  "productUrl": "https://shop.example/item/123",
  "quantity": 1
}
```

Success: `200 OK` with the persisted quotation and extracted product name, source price, currency, exchange rate, fees, total, status, and timestamps. Restricted product: `400 RESTRICTED_ITEM`. Unsupported/failed extraction: a stable `4xx` or `502` error contract, never a fabricated quotation.

Temporary compatibility: keep `POST /api/v1/quotations` during migration and document it as deprecated. It must call the same application use case after mapping its explicit product fields; do not maintain two calculation paths.

### 5.2 Order creation

```http
POST /api/v1/orders
Content-Type: application/json

{
  "quotationId": "<uuid>",
  "deliveryAddress": "Thu Duc City, Ho Chi Minh City"
}
```

Customer identity should eventually come from authenticated context. Until authentication exists, retain `customerId` as a temporary compatibility field and validate it against the quotation. The target transaction creates the Order, items, initial timeline, and `order.created.v1` outbox row. Quotation confirmation must be an explicit Quotation Service operation, not a direct Order database update.

### 5.3 Deposit and callback

Canonical public endpoint: `POST /api/v1/payments/deposit`. Keep `/deposits` as a temporary alias.

Payment gateway adapter contract:

- create/tokenize a transaction using a timeout and bounded retry policy;
- return a hosted payment URL;
- never accept or persist raw card data;
- map provider errors to stable domain/dependency errors.

Canonical webhook endpoint: `POST /api/v1/payments/callback`. It must verify the signature over the raw request body before parsing or mutating state. Provider event/reference identity must be unique and idempotent. A repeated valid callback returns success without emitting a duplicate business event.

### 5.4 Notification stream

Expose an authenticated-or-demo-scoped SSE endpoint first, because it is simpler than WebSocket for one-way status updates:

```http
GET /api/v1/notifications/orders/{orderId}/stream
```

A dedicated notification component consumes `order.status_changed.v1`; Nginx only routes the HTTP stream. Do not embed a Kafka consumer inside Nginx. The frontend must also recover via `GET /orders/{id}` and timeline polling after reconnect, because SSE delivery is not the system of record.

## 6. Refactor phases

### Phase 0 — Baseline freeze and executable sequence tests

Goal: protect working behavior before changing contracts.

Work:

- Add a sequence traceability test matrix covering steps 1–34.
- Add black-box tests for the current quotation, order, deposit, Kafka transition, and timeline flow.
- Record current API payloads and event schemas as fixtures.
- Add latency measurement around quotation creation without claiming the 2.5-second target yet.
- Confirm the current full demo remains green.

Likely files: `scripts/`, `docs/`, handler/application tests; no domain behavior changes.

Acceptance:

- Existing demo reaches `WAITING_DEPOSIT -> WAITING_PURCHASE` through Kafka.
- Baseline tests fail clearly if an existing response or event changes unexpectedly.

### Phase 1 — Product extraction boundary

Goal: implement sequence steps 1–4 without changing pricing or Order behavior.

Work:

- Add `ProductExtractor` port and a deterministic demo adapter.
- Add `POST /api/v1/quotations/extract`.
- Accept URL-led input, normalize and validate supported URLs, extract name/price/currency/image metadata, then run restriction checks.
- Keep the legacy quotation endpoint as a deprecated adapter to the same use case.
- Add SSRF protections before any real HTTP fetch: allowlisted schemes, DNS/IP validation, redirect limits, response size/type limits, and timeouts.

Acceptance:

- A supported URL produces extracted product data without caller-supplied name/price/currency.
- A restricted extracted product returns `400 RESTRICTED_ITEM` and is not persisted.
- Private, loopback, metadata-service, and oversized targets are rejected.

### Phase 2 — Exchange-rate provider and 2.5-second budget

Goal: implement sequence steps 7–10 and make the latency requirement measurable.

Work:

- Add an HTTP exchange-rate adapter behind the existing port.
- Configure per-attempt timeout, maximum three attempts, exponential backoff with jitter, and a total request deadline below 2.5 seconds.
- Retry only transient network/`429`/`5xx` failures; never retry validation or ordinary `4xx` failures.
- Retain the deterministic mock provider for tests and offline demo mode.
- Add structured timing/error logs and latency integration tests.

Acceptance:

- Normal demo requests complete below 2.5 seconds in the controlled acceptance environment.
- A transient provider failure can recover within the total budget.
- Exhausted retries return `502 EXCHANGE_RATE_UNAVAILABLE`; no incomplete quotation is persisted.

### Phase 3 — Explicit quotation confirmation and Order contract

Goal: align sequence steps 12–17 and remove the hidden “pending quotation is orderable forever” rule.

Work:

- Add an idempotent internal confirmation operation owned by Quotation Service.
- Define confirmation/expiration rules and prevent two Orders for one quotation.
- Orchestrate confirmation and Order creation without a distributed transaction: use an explicit reservation/confirmation token or a compensatable idempotent workflow, documented with failure cases.
- Preserve atomic Order + items + timeline + outbox creation.
- Move the public request toward `quotationId` + delivery data; keep temporary customer compatibility until authentication exists.
- Ensure `201 Created` returns `orderId`, total, deposit, remaining balance, and `WAITING_DEPOSIT`.

Acceptance:

- Only a valid unexpired quotation can create an Order.
- Repeating the request cannot create a second Order.
- Partial failure is retryable and cannot leave an unusable confirmed quotation without a recoverable path.
- `order.created.v1` remains transactional.

### Phase 4 — Payment gateway port and singular deposit endpoint

Goal: align sequence steps 18–23 without implementing callback success yet.

Work:

- Add a `PaymentGateway` port and mock hosted-payment adapter.
- Add canonical `POST /api/v1/payments/deposit`; keep `/deposits` as deprecated alias.
- Create the provider transaction before returning the hosted URL, with safe timeout/retry/idempotency behavior.
- Persist only gateway-safe references and the hosted URL.
- Add correlation/request IDs across Gateway, Payment, and provider logs.

Acceptance:

- A `WAITING_DEPOSIT` Order receives one pending deposit and a hosted payment URL.
- Duplicate requests return the existing payment or a documented idempotent result.
- No raw card field exists in request, database, logs, or response.

### Phase 5 — Signed, idempotent payment callback

Goal: align sequence steps 24–31 while preserving Kafka as the single Order update path.

Work:

- Add `POST /api/v1/payments/callback` and preserve the raw body for signature verification.
- Add webhook secret configuration, timestamp tolerance, constant-time signature comparison, and replay protection.
- Store provider callback/event identity with a unique constraint.
- In one Payment DB transaction, mark the payment succeeded and insert `payment.deposit_succeeded.v1` in outbox.
- Keep `/mock-success` only in an explicit demo profile; disable or remove it from normal deployment.
- Keep the existing Order idempotent consumer and validate amount/order/payment identity before transition.

Acceptance:

- Invalid or stale signatures cause no state change.
- Replayed valid callbacks do not duplicate outbox events or Order timeline entries.
- A valid callback eventually yields Payment `SUCCEEDED`, Order `WAITING_PURCHASE`, and `order.status_changed.v1`.
- There is no synchronous Payment-to-Order state mutation.

### Phase 6 — Notification stream and frontend timeline update

Goal: implement sequence steps 32–34.

Work:

- Add a small notification service/component that consumes `order.status_changed.v1`.
- Add SSE routing through Nginx, including buffering/timeout headers suitable for streams.
- Add reconnect support with event IDs and a bounded replay strategy, or document that reconnect performs an Order/timeline refresh.
- Update the frontend to subscribe after payment redirect/callback completion and render natural Vietnamese timeline text.
- Preserve polling/read-model fallback when the stream is unavailable.

Acceptance:

- A status event reaches a connected client without page reload.
- Reconnect does not leave the UI permanently stale.
- Kafka or SSE duplicates do not duplicate visible timeline entries.

### Phase 7 — Contract cleanup and full sequence acceptance

Goal: remove migration scaffolding and prove the complete target flow.

Work:

- Remove deprecated endpoints only after all callers and docs use canonical paths.
- Decide and document the authentication/customer-identity boundary; do not silently trust arbitrary customer IDs in a production profile.
- Normalize Vietnamese tracking descriptions and error codes.
- Update README, architecture, API examples, event docs, environment example, Compose, demo/reset scripts, and EC2 guide.
- Add a single executable acceptance scenario matching sequence steps 1–34.

Acceptance:

- Product URL -> extracted quotation -> confirmed Order -> hosted deposit -> signed callback -> Kafka Order transition -> live UI notification succeeds.
- Restricted-product, exchange-provider failure, duplicate Order, invalid webhook, replayed webhook, Kafka duplicate, and SSE reconnect cases are asserted.
- All required validation commands pass.

## 7. Required validation after every phase

From `backend/`:

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./...
```

From repository root:

```bash
docker compose config --quiet
docker compose build <touched-services>
docker compose up -d <dependencies-and-touched-services>
```

Then run the phase-specific black-box acceptance checks through Nginx. For Kafka phases, verify both the final API state and the outbox/processed-event invariants; seeing a Kafka message alone is insufficient.

## 8. Definition of done for sequence alignment

The refactor is complete only when all of the following are true:

- The canonical endpoints and payloads in section 5 are implemented and documented.
- Quotation extraction is URL-led, restricted items are rejected, and the controlled latency test meets 2.5 seconds.
- Quotation confirmation and Order creation are idempotent and recoverable.
- Deposit creation returns a hosted URL without handling raw card data.
- Payment callbacks are signed, replay-safe, and idempotent.
- Kafka is the sole cross-service path that moves Order from `WAITING_DEPOSIT` to `WAITING_PURCHASE`.
- Order transition, timeline append, processed-event insert, and status outbox insert remain transactional.
- A client receives the status change through SSE and can recover after disconnect.
- The complete sequence and all listed negative cases pass in Docker Compose.
- Documentation reflects shipped behavior, not future intent.

## 9. Out of scope unless separately approved

- Real marketplace scraping beyond explicitly supported provider adapters.
- Real card collection or PCI-scoped storage.
- Full production payment-provider certification.
- gRPC added solely to mimic one arrow in the diagram.
- Saga framework, event sourcing, full CQRS, Kubernetes, service mesh, or a second Order-state writer.
- Later logistics states beyond the existing warehouse demo slice.

