# Phase 8 final acceptance report

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

Final verification on 2026-07-11 passed: formatting check, `go test ./...`, `go vet ./...`, `go build ./...`, `docker compose config --quiet`, all five application healthchecks, Nginx configuration validation, Docker image builds, and the complete demo through `ARRIVED_FOREIGN_WAREHOUSE`. The host did not have `jq`, so the unchanged demo script was executed from a disposable Alpine container on the Compose network with Bash, curl, and jq.

## Known demo limitations

The stack is intentionally single-node and unauthenticated; providers and Admin rates are mocks/configuration; Kafka UI is demo-only; local Compose has no TLS; and the warehouse event uses the documented direct demo state transition.
