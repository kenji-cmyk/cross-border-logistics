# AWS EC2 single-instance demo deployment

This guide deploys Caddy automatic HTTPS, the React frontend, API gateway, six Go services (including Notification SSE), PostgreSQL, and Kafka on one Ubuntu EC2 instance. It is a demo topology, not production HA.

## Instance and network

Use an instance with enough memory for six Go services, PostgreSQL, Kafka, and two Nginx containers. A `t3.large`/`t3a.large` (8 GiB) is the safer demo starting point; 4 GiB may require swap while images build. Use at least 20 GiB of EBS and attach an Elastic IP or DNS name if the address must remain stable.

Security-group inbound rules:

- TCP 22 only from your administrator IP.
- TCP 80 from `0.0.0.0/0` for ACME validation and HTTP-to-HTTPS redirects.
- TCP 443 from `0.0.0.0/0` for the public application and SePay IPN.
- Do **not** expose 5432, 9092, 8080, or Kafka UI 8088 publicly. If Kafka UI is needed, restrict 8088 to your IP or use `ssh -L 8088:localhost:8088 user@host`.

## Install and launch

```bash
sudo apt update
sudo apt install -y git docker.io docker-compose-plugin curl jq make
sudo usermod -aG docker "$USER"
newgrp docker

git clone <repository-url> # replace with the real URL
cd cross-border-logistics
cp .env.example .env
sed -i 's/^APP_ENV=.*/APP_ENV=production/' .env
chmod +x scripts/*.sh

# Replace this value with the public DNS name whose A record points at the EC2 IP.
sed -i 's/^PUBLIC_SITE_ADDRESS=.*/PUBLIC_SITE_ADDRESS=cross-border-logistics.duckdns.org/' .env
sed -i 's|^SEPAY_PUBLIC_URL=.*|SEPAY_PUBLIC_URL=https://cross-border-logistics.duckdns.org|' .env

docker compose config --quiet
docker compose run --rm --no-deps caddy \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
docker compose up -d --build
docker compose ps
docker compose logs --tail=100 caddy
curl --fail https://cross-border-logistics.duckdns.org/ui-health
curl --fail https://cross-border-logistics.duckdns.org/health
BASE_URL=https://cross-border-logistics.duckdns.org make demo
```

Before starting, replace `POSTGRES_PASSWORD` in `.env` with a long unique value and configure the SePay provider variables when needed. Keep `.env` out of Git and never print it during a demo. The frontend calls `/api/*` on its own public origin; Caddy terminates TLS and forwards to frontend Nginx, which forwards API requests to the internal gateway. No backend service or CORS port needs to be exposed.

Open `https://cross-border-logistics.duckdns.org/` after the health checks pass. A request to the HTTP origin must redirect to HTTPS. Kafka UI binds only to `127.0.0.1:8088`; access it through an SSH tunnel when needed:

```bash
ssh -L 8088:127.0.0.1:8088 ubuntu@<EC2-public-IP>
```

Then browse to `http://localhost:8088` on your own machine. Caddy stores certificates and private keys in the `caddy-data` volume and renews them automatically. Preserve that volume across redeployments and never commit certificate material to Git.

Configure SePay's Payment Gateway IPN URL as:

```text
https://cross-border-logistics.duckdns.org/api/v1/payments/sepay/pg/ipn
```

Use auth type `SECRET_KEY` with the same value as `SEPAY_PG_SECRET_KEY`. After an
`.env` change, recreate `payment-service`; a plain container restart does not
load new environment values.

## Update and operate

```bash
git pull
docker compose up -d --build --remove-orphans
docker compose ps
docker compose logs --tail=100 caddy
curl --fail https://cross-border-logistics.duckdns.org/ui-health
curl --fail https://cross-border-logistics.duckdns.org/health
```

Run `docker image prune -f` manually when disk cleanup is needed. Stop without deleting data using `docker compose down`. `docker compose down -v` permanently deletes PostgreSQL data, Caddy certificates, and all project volumes. Back up data before destructive operations.

If certificate issuance fails, verify the DNS A record, TCP 80/443 security-group
rules, Ubuntu firewall, and `docker compose logs caddy`. To restore temporary
HTTP access while diagnosing, set `PUBLIC_SITE_ADDRESS=:80` and recreate Caddy;
do not delete volumes.

## Verification

Confirm all containers are running/healthy, HTTP redirects to HTTPS, the certificate validates without `curl -k`, both health URLs succeed, and an unauthenticated HTTPS POST to `/api/v1/payments/sepay/pg/ipn` reaches the handler and returns `401 INVALID_IPN_SECRET`. Verify SSE returns `: connected`, a SePay Sandbox payment reaches Payment `SUCCEEDED` and Order `WAITING_PURCHASE`, `make demo` reaches `ARRIVED_FOREIGN_WAREHOUSE`, and Kafka UI (when privately accessed) shows the expected topics and consumer groups.
