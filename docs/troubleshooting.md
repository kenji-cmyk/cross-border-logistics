# Troubleshooting

## Stack does not become ready

Run `docker compose ps` and `docker compose logs --tail=200 <service>`. PostgreSQL and Kafka must become healthy and `kafka-init` must exit successfully before dependent services start. On a small EC2 instance, check `free -h` and Docker disk usage.

## Port conflicts

If host port 80 or 8088 is already used, stop the known conflicting service or change the published port in `compose.yaml` and set `BASE_URL` accordingly. Do not kill an unknown process blindly.

## Database does not exist

Database initialization runs only when the PostgreSQL volume is first created. For disposable demo data, use `./scripts/reset-demo.sh --delete-data`, confirm the warning, then start again. This permanently deletes the project volume.

## Demo timeout after payment or package receipt

Inspect `docker compose logs payment-service warehouse-service order-service kafka`. Confirm Kafka is healthy, the four topics exist in Kafka UI, outbox workers logged publication, and Order consumer groups are active. The demo prints the last Order response on timeout.

## `curl` or `jq` missing

Install both tools (`sudo apt install -y curl jq` on Ubuntu). Run scripts through Bash; PowerShell users can use Git Bash or WSL.

## Nginx returns 502

Check `docker compose ps` and the target service logs. Nginx starts only after service healthchecks pass, but a service can later restart. Internal endpoints are intentionally unavailable through Nginx.

## Configuration changes are ignored

Edit `.env`, then run `docker compose up -d --force-recreate`. Fixed Admin settings are read at startup; live exchange rates refresh after `EXCHANGE_RATE_CACHE_TTL`. Existing PostgreSQL data persists across ordinary `down`/`up` cycles.

## Live exchange rates are unavailable

The default Compose configuration reads Vietcombank's public selling-rate XML feed through Admin and caches it for five minutes. Check outbound HTTPS/DNS from `admin-service`, then inspect its logs. The first request returns an error if no live snapshot has ever loaded; after one successful load, a temporary refresh failure serves the last known snapshot. Set `EXCHANGE_RATE_PROVIDER=fixed` and recreate Admin and Quotation for deterministic offline development.
