#!/usr/bin/env bash
# Load demo data through Envoy: 3 customers, funded accounts, N transfers (default 50).
set -euo pipefail
source "$(dirname "$0")/lib.sh"
N="${SEED_TRANSFERS:-50}"
post() { curl -fsS -X POST "$BASE_URL$1" -H 'Content-Type: application/json' -d "$2"; }

declare -a ACCTS
for name in "Alice" "Bob" "Carol"; do
  c=$(post /v1/customers "{\"name\":\"$name\"}")
  a=$(post /v1/accounts "{\"customer_id\":\"$(jfield "$c" customer_id)\",\"currency\":\"GBP\"}")
  id=$(jfield "$a" account_id)
  post /v1/deposits "{\"account_id\":\"$id\",\"amount_minor\":100000,\"idempotency_key\":\"seed-$id\"}" >/dev/null
  ACCTS+=("$id")
  echo "seeded $name $id"
done
for i in $(seq 1 "$N"); do
  from=${ACCTS[$((i % 3))]}; to=${ACCTS[$(((i + 1) % 3))]}
  post /v1/transfers "{\"idempotency_key\":\"seed-$(date +%s)-$i-$RANDOM\",\"from_account_id\":\"$from\",\"to_account_id\":\"$to\",\"amount_minor\":$((100 + i * 10))}" >/dev/null
done
echo "seeded $N transfers"
