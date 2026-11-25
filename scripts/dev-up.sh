#!/usr/bin/env bash
# Start the whole local stack (Docker Compose) and seed demo data.
set -euo pipefail
source "$(dirname "$0")/lib.sh"
cd "$ROOT"
mkdir -p .local/sink
docker compose up -d --build
echo "waiting for Envoy + services..."
for i in $(seq 1 90); do
  if curl -fsS "$BASE_URL/healthz/accounts" >/dev/null 2>&1 && curl -fsS "$BASE_URL/healthz/ledger" >/dev/null 2>&1 \
     && curl -fsS "$BASE_URL/healthz/transfers" >/dev/null 2>&1 && curl -fsS "$BASE_URL/healthz/notifications" >/dev/null 2>&1; then
    echo "stack is up: $BASE_URL"; break
  fi
  sleep 3
done
[ "${SEED:-1}" = "1" ] && "$ROOT/scripts/seed.sh"
echo "web: $BASE_URL  jaeger: http://localhost:26686  prometheus: http://localhost:29090"
