# AWS EC2 single-instance demo deployment

This guide deploys the React frontend, API gateway, six Go services (including Notification SSE), PostgreSQL, and Kafka on one Ubuntu EC2 instance. It is a demo topology, not production HA.

## Instance and network

Use an instance with enough memory for six Go services, PostgreSQL, Kafka, and two Nginx containers. A `t3.large`/`t3a.large` (8 GiB) is the safer demo starting point; 4 GiB may require swap while images build. Use at least 20 GiB of EBS and attach an Elastic IP or DNS name if the address must remain stable.

Security-group inbound rules:

- TCP 22 only from your administrator IP.
- TCP 80 from the intended audience (`0.0.0.0/0` only when a public HTTP demo is intended). This is the only application port published by Compose.
- TCP 443 only after separately adding TLS.
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
docker compose up -d --build
docker compose ps
curl --fail http://localhost/ui-health
curl http://localhost/health
make demo
```

Before starting, replace `POSTGRES_PASSWORD` in `.env` with a long unique value. Keep `.env` out of Git and never print it during a demo. The frontend calls `/api/*` on its own public origin; its Nginx container forwards those requests to the internal API gateway, so no backend service or CORS port needs to be exposed.

Open `http://<EC2-public-IP>/` after the health checks pass. Kafka UI binds only to `127.0.0.1:8088`; access it through an SSH tunnel when needed:

```bash
ssh -L 8088:127.0.0.1:8088 ubuntu@<EC2-public-IP>
```

Then browse to `http://localhost:8088` on your own machine. This stack currently serves HTTP only. Add a domain and TLS termination before handling real customer data.

## Update and operate

```bash
git pull
docker compose up -d --build --remove-orphans
docker compose ps
docker compose logs --tail=100
curl --fail http://localhost/ui-health
curl --fail http://localhost/health
```

Run `docker image prune -f` manually when disk cleanup is needed. Stop without deleting data using `docker compose down`. `docker compose down -v` permanently deletes PostgreSQL data and all project volumes. Back up data before destructive operations.

## Verification

Confirm all containers are running/healthy, the landing page opens on port 80, both health URLs succeed, `make demo` reaches `ARRIVED_FOREIGN_WAREHOUSE`, and Kafka UI (when privately accessed) shows the four topics and two Order consumer groups.
