# Troubleshooting

## Stack does not become ready

Run `docker compose ps` and `docker compose logs --tail=200 <service>`. PostgreSQL and Kafka must become healthy and `kafka-init` must exit successfully before dependent services start. On a small EC2 instance, check `free -h` and Docker disk usage.

## Port conflicts

If host port 80, 443, or 8088 is already used, inspect it with `ss -ltnp` (Linux) or the platform equivalent. Stop only a known conflicting service, or change `PUBLIC_HTTP_PORT`/`PUBLIC_HTTPS_PORT` and set `BASE_URL` accordingly. Do not kill an unknown process blindly.

## HTTPS certificate is not issued

Confirm `PUBLIC_SITE_ADDRESS` contains the public DNS name without a scheme, its
A record points to the server, and inbound TCP 80 and 443 are open in both the
cloud firewall and the host firewall. Then inspect:

```bash
docker compose logs --tail=200 caddy
curl -I http://cross-border-logistics.duckdns.org/health
curl -I https://cross-border-logistics.duckdns.org/health
```

Caddy needs port 80 for ACME validation and redirects even though application
traffic uses HTTPS. Do not use `curl -k` as an acceptance check. If temporary
HTTP recovery is needed, set `PUBLIC_SITE_ADDRESS=:80` and recreate Caddy.

## SePay returns successfully but payment remains pending

First inspect SePay's IPN log and Caddy access logs:

```bash
docker compose logs --since=30m caddy payment-service \
  | grep -E 'sepay/pg/ipn|payment request failed|outbox event published'
```

The configured IPN must be the full public HTTPS URL
`https://<domain>/api/v1/payments/sepay/pg/ipn`. A `401` means the dashboard
`X-Secret-Key` does not match `SEPAY_PG_SECRET_KEY`; `400` means the paid-order
payload or amount is invalid; `404` usually means the invoice number does not
match a stored payment reference. A browser return URL is informational and
does not mark a payment successful.

## SSE does not update through HTTPS

Test the public stream directly:

```bash
curl -N --max-time 25 \
  https://cross-border-logistics.duckdns.org/api/v1/notifications/orders/<order-id>/stream
```

It must immediately return `: connected`. Caddy preserves event-stream
responses; if the connection works but no status arrives, inspect
`notification-service`, the `notification-service-status-events` Kafka group,
and the authoritative Order API.

## Database does not exist

Database initialization runs only when the PostgreSQL volume is first created. For disposable demo data, use `./scripts/reset-demo.sh --delete-data`, confirm the warning, then start again. This permanently deletes the project volume.

## Demo timeout after payment or package receipt

Inspect `docker compose logs payment-service warehouse-service order-service kafka`. Confirm Kafka is healthy, the four topics exist in Kafka UI, outbox workers logged publication, and Order consumer groups are active. The demo prints the last Order response on timeout.

## `curl` or `jq` missing

Install both tools (`sudo apt install -y curl jq` on Ubuntu). Run scripts through Bash; PowerShell users can use Git Bash or WSL.

## Caddy or Nginx returns 502

Check `docker compose ps`, `docker compose logs caddy frontend api-gateway`, and the target service logs. Caddy waits for frontend health, and frontend waits for the API gateway, but a service can later restart. Internal endpoints are intentionally unavailable through the public ingress.

## Configuration changes are ignored

Edit `.env`, then run `docker compose up -d --force-recreate <affected-service>`. Caddy reads `PUBLIC_SITE_ADDRESS` at container creation and Payment reads `SEPAY_PUBLIC_URL` at startup. Fixed Admin settings are also read at startup; live exchange rates refresh after `EXCHANGE_RATE_CACHE_TTL`. Existing PostgreSQL and Caddy certificate data persist across ordinary `down`/`up` cycles.

## Live exchange rates are unavailable

The default Compose configuration reads Vietcombank's public selling-rate XML feed through Admin and caches it for five minutes. Check outbound HTTPS/DNS from `admin-service`, then inspect its logs. The first request returns an error if no live snapshot has ever loaded; after one successful load, a temporary refresh failure serves the last known snapshot. Set `EXCHANGE_RATE_PROVIDER=fixed` and recreate Admin and Quotation for deterministic offline development.
