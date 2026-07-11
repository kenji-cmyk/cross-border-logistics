#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${BASE_URL:-http://localhost}
WAIT_TIMEOUT_SECONDS=${WAIT_TIMEOUT_SECONDS:-120}

command -v curl >/dev/null 2>&1 || { echo "curl is required." >&2; exit 1; }

deadline=$((SECONDS + WAIT_TIMEOUT_SECONDS))
last_gateway=""
last_admin=""
while (( SECONDS < deadline )); do
  last_gateway=$(curl --silent --show-error --max-time 3 "$BASE_URL/health" 2>&1 || true)
  last_admin=$(curl --silent --show-error --max-time 3 "$BASE_URL/api/v1/admin/rates" 2>&1 || true)
  if [[ "$last_gateway" == *'"status":"UP"'* && "$last_admin" == *'"serviceFeePercent"'* ]]; then
    echo "Gateway and routed services are ready."
    exit 0
  fi
  sleep 2
done

echo "Timed out after ${WAIT_TIMEOUT_SECONDS}s waiting for the stack." >&2
echo "Last gateway response: $last_gateway" >&2
echo "Last Admin response: $last_admin" >&2
exit 1
