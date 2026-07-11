# AWS EC2 single-instance demo deployment

This guide prepares the existing Docker Compose stack for one Ubuntu EC2 instance. It is a demo topology, not production HA.

## Instance and network

Use an instance with enough memory for five Go services, PostgreSQL, Kafka, Nginx, and optional Kafka UI (for example, 4 GiB or more is a practical demo starting point). Allocate adequate EBS storage and a stable public IP/DNS name if needed.

Security-group inbound rules:

- TCP 22 only from your administrator IP.
- TCP 80 from the intended audience (`0.0.0.0/0` only when a public HTTP demo is intended).
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
chmod +x scripts/*.sh
docker compose up -d --build
docker compose ps
curl http://localhost/health
make demo
```

Change `POSTGRES_PASSWORD` in `.env` before an Internet-reachable deployment. Keep `.env` out of Git and never print it during a demo. This template does not include a production secrets manager or TLS termination.

## Update and operate

```bash
git pull
docker compose up -d --build --remove-orphans
docker compose ps
docker compose logs --tail=100
docker image prune -f
```

Stop without deleting data using `docker compose down`. `docker compose down -v` permanently deletes PostgreSQL data and all project volumes. Back up data before destructive operations.

## Verification

Confirm all containers are running/healthy, `curl http://localhost/health` succeeds, `make demo` reaches `ARRIVED_FOREIGN_WAREHOUSE`, and Kafka UI (when privately accessed) shows the four topics and two Order consumer groups.
