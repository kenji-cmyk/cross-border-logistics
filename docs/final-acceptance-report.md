# Phase 8 final acceptance report

> Sequence-alignment update (2026-07-11): the current canonical flow now uses
> `POST /api/v1/quotations/extract`, idempotent quotation confirmation and Order
> creation, `POST /api/v1/payments/deposit`, an HMAC-signed replay-safe callback,
> Kafka-only Order mutation, and the Notification SSE stream. Current validation
> passed Go format/test/vet/build, frontend test/build, Compose config and image
> builds, all container health checks, a 180 ms controlled quotation request,
> duplicate Order/callback assertions, `WAITING_PURCHASE`, one outbox/timeline
> transition, and public-port SSE replay. The legacy report below documents the
> earlier warehouse slice and is retained as historical evidence.

## Sequence E2E regression suite

Run the complete repeatable suite from the repository root on Windows:

```powershell
.\scripts\sequence-e2e.ps1 -IncludeDependencyFailure
```

The suite covers 25 scenarios: public health, controlled quotation latency,
malformed and unknown fields, product restrictions, unsafe and unsupported URLs,
unsupported currency, request-ID propagation, Order validation/customer matching,
six-way concurrent Order/deposit/callback idempotency, callback signature/staleness,
transactional database invariants, duplicate Kafka delivery, SSE replay/reconnect,
warehouse compatibility, deprecated routes, and exchange-provider exhaustion with
automatic restoration. The 2026-07-11 run completed with **25 passed, 0 failed**.

## Scope

Phase 8 adds reproducible validation, an asserted end-to-end demo, complete architecture/API/event/deployment/troubleshooting documentation, safe reset commands, and Compose readiness. It introduces no new business service or workflow.

## Acceptance flow

`scripts/demo.sh` checks gateway/routing, creates a unique quotation and Order, verifies `WAITING_DEPOSIT`, creates and succeeds a deposit, polls for Kafka-driven `WAITING_PURCHASE`, receives/retrieves a unique package, polls for `ARRIVED_FOREIGN_WAREHOUSE`, validates all three timeline statuses/descriptions and Admin rate defaults, and exits non-zero on any mismatch.

## Verification commands

Run from repository root:

```bash
make fmt
make test
make vet
make build
docker compose config
docker compose up -d --build
make demo
```

Final verification on 2026-07-11 passed: formatting check, `go test ./...`, `go vet ./...`, `go build ./...`, `docker compose config --quiet`, all six application healthchecks, Nginx configuration validation, Docker image builds, and the complete demo through `ARRIVED_FOREIGN_WAREHOUSE`. The host did not have `jq`, so the unchanged demo script was executed from a disposable Alpine container on the Compose network with Bash, curl, and jq.

## Known demo limitations

The stack is intentionally single-node and unauthenticated; product/payment providers remain demo adapters; Admin uses cached Vietcombank reference rates by default with a fixed offline mode; Kafka UI is demo-only; local Compose has no TLS; and the warehouse event uses the documented direct demo state transition.
