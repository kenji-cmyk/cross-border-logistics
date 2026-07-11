#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" == "--delete-data" ]]; then
  echo "WARNING: This deletes PostgreSQL data and all Docker volumes for this project." >&2
  if [[ "${CONFIRM_DELETE_DATA:-}" != "yes" ]]; then
    read -r -p "Type DELETE to continue: " confirmation
    [[ "$confirmation" == "DELETE" ]] || { echo "Reset cancelled."; exit 1; }
  fi
  docker compose down -v
  exit 0
fi

if [[ $# -gt 0 ]]; then
  echo "Usage: $0 [--delete-data]" >&2
  exit 2
fi

docker compose down
