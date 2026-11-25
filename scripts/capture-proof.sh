#!/usr/bin/env bash
# Capture proof for a phase: scripts/capture-proof.sh phase1 | phase1-kind | phase2
# phase1: compose stack up (screenshots + cassandra/kafka/envoy/duckdb logs)
# phase1-kind: kind stack up (kubectl output + ledger restart drill)
set -euo pipefail
source "$(dirname "$0")/lib.sh"
cd "$ROOT"
case "${1:-}" in
phase1)
  P=proof/phase1-local; mkdir -p $P/{screenshots,logs}
  (cd web && node capture/capture.mjs "../$P/screenshots" local)
  docker compose exec -T cassandra cqlsh -e "SELECT account_id, balance_minor, version FROM bank.accounts LIMIT 5; SELECT account_id, posting_id, direction, amount_minor, counterparty FROM bank.ledger_entries_by_account LIMIT 8; SELECT idempotency_key, transfer_id, status FROM bank.transfers_by_idempotency_key LIMIT 5;" > $P/logs/cqlsh-ledger.txt
  { docker compose exec -T kafka /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --describe
    docker compose exec -T kafka /opt/kafka/bin/kafka-consumer-groups.sh --bootstrap-server localhost:9092 --describe --all-groups
    echo "--- notifications consumer log"; docker compose logs notifications 2>&1 | grep consumed | tail -8; } > $P/logs/kafka-consume.txt 2>&1
  { curl -s localhost:19901/clusters | grep -E "::(cx_active|rq_total|health_flags|rq_success)" | grep -E "^(accounts|ledger|transfers)::"
    echo "--- retry/outlier stats"; curl -s localhost:19901/stats | grep -E "cluster\.ledger\.(upstream_rq_total|upstream_rq_retry|outlier_detection\.ejections_total)"; } > $P/logs/envoy-clusters.txt
  { echo "-- daily_volume"; "$(duckdb_bin)" -no-agent -c "$(cat analytics/duckdb/daily_volume.sql)"; echo "-- top_accounts"; "$(duckdb_bin)" -no-agent -c "$(cat analytics/duckdb/top_accounts.sql)"; } > $P/logs/duckdb-daily-volume.txt 2>&1
  ;;
phase1-kind)
  P=proof/phase1-local; mkdir -p $P/logs
  kubectl -n bank get all,statefulset,pvc,jobs -o wide > $P/logs/kind-get-all.txt
  scripts/drill-ledger-restart.sh 2>&1 | tee $P/logs/drill-ledger-restart.log
  ;;
phase2)
  exec scripts/capture-cloud.sh
  ;;
*) echo "usage: $0 phase1|phase1-kind|phase2"; exit 2;;
esac
