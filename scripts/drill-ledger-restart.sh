#!/usr/bin/env bash
# Resilience drill (kind or EKS): delete one ledger pod while transfers run; Envoy retries/outlier detection keep them flowing.
set -euo pipefail
source "$(dirname "$0")/lib.sh"
NS="${NS:-bank}"; DURATION="${DURATION:-40}"; WORKERS="${WORKERS:-4}"
post() { curl -sS -m 10 -X POST "$BASE_URL$1" -H 'Content-Type: application/json' -d "$2"; }
mk() { c=$(post /v1/customers "{\"name\":\"$1\"}"); a=$(post /v1/accounts "{\"customer_id\":\"$(jfield "$c" customer_id)\"}"); id=$(jfield "$a" account_id); post /v1/deposits "{\"account_id\":\"$id\",\"amount_minor\":10000000}" >/dev/null; echo "$id"; }
A=$(mk drill-a); B=$(mk drill-b)

kubectl -n "$NS" port-forward svc/envoy 19901:9901 >/dev/null 2>&1 & PF=$!
trap 'kill $PF 2>/dev/null || true' EXIT
sleep 3
stat() { curl -s localhost:19901/stats | grep -E "^cluster\.ledger\.(upstream_rq_retry|outlier_detection\.ejections_total|upstream_rq_total):" | tr '\n' ' '; echo; }
echo "envoy ledger stats before: $(stat)"
RES=$(mktemp)
worker() { end=$((SECONDS + DURATION)); while [ $SECONDS -lt $end ]; do
  code=$(curl -s -o /dev/null -w '%{http_code}' -m 10 -X POST "$BASE_URL/v1/transfers" -H 'Content-Type: application/json' \
    -d "{\"idempotency_key\":\"drill-$RANDOM-$RANDOM-$SECONDS\",\"from_account_id\":\"$A\",\"to_account_id\":\"$B\",\"amount_minor\":100}" || true); echo "$code" >> "$RES"; done; }
PIDS=(); for _ in $(seq "$WORKERS"); do worker & PIDS+=($!); done
sleep 10
echo "$(date +%T) force-deleting one ledger pod (grace period 0: in-flight connections are reset)"
kubectl -n "$NS" delete "$(kubectl -n "$NS" get pod -l app=ledger -o name | head -1)" --grace-period=0 --force --wait=false
kubectl -n "$NS" get pods -l app=ledger -o wide
wait "${PIDS[@]}" || true
total=$(wc -l < "$RES"); ok=$(grep -c '^200$' "$RES" || true)
echo "envoy ledger stats after:  $(stat)"
echo "requests=$total success=$ok rate=$((100 * ok / total))%"
sort "$RES" | uniq -c
[ $((100 * ok / total)) -ge 95 ] && echo "DRILL OK (>=95%)"
