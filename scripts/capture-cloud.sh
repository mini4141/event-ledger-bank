#!/usr/bin/env bash
# Capture Phase 2 proof from the running AWS stack (reads .local/cloud-url).
set -euo pipefail
source "$(dirname "$0")/lib.sh"; source "$ROOT/scripts/aws-env.sh"
cd "$ROOT"
export BASE_URL="$(cat .local/cloud-url)"
P=proof/phase2-cloud; mkdir -p $P/{screenshots,logs,tests/playwright-eks}
out() { $TF output -raw "$1"; }
MSK_ARN=$(out msk_cluster_arn); WG=$(out athena_workgroup); DB=$(out glue_database)

echo "== screenshots + Playwright against the NLB"
(cd web && node capture/capture.mjs "../$P/screenshots" eks)
(cd web && BASE_URL="$BASE_URL" npx playwright test 2>&1) | tee $P/tests/playwright-eks/playwright.log
rm -rf $P/tests/playwright-eks/report; cp -r web/playwright-report $P/tests/playwright-eks/report; cp web/test-results/junit.xml $P/tests/playwright-eks/junit.xml

echo "== AWS describe logs"
if grep -q "keyspaces=true" .local/deploy-mode; then
  aws keyspaces get-keyspace --keyspace-name bank | redact > $P/logs/keyspaces-describe.json
  for t in accounts ledger_entries_by_account; do aws keyspaces get-table --keyspace-name bank --table-name $t | redact >> $P/logs/keyspaces-describe.json; done
else
  { echo "Amazon Keyspaces was NOT used: the deployer principal has no cassandra:* permissions."
    echo "Call result:"; aws keyspaces list-keyspaces 2>&1 || true
    echo; echo "Fallback: the same services ran against Cassandra 5 inside EKS (config only: CASSANDRA_AUTH=none)."
    kubectl -n bank exec cassandra-0 -- cqlsh -e "DESCRIBE KEYSPACE bank" 2>&1 | head -60
    kubectl -n bank exec cassandra-0 -- cqlsh -e "SELECT account_id, balance_minor, version FROM bank.accounts LIMIT 5" 2>&1; } | redact > $P/logs/keyspaces-describe.json
fi
aws kafka describe-cluster-v2 --cluster-arn "$MSK_ARN" | redact > $P/logs/msk-describe.json
{ kubectl -n bank get all,jobs -o wide; echo; kubectl get nodes -o wide; echo
  kubectl -n bank logs deploy/ledger --tail=5; kubectl -n bank logs deploy/notifications --tail=5; kubectl -n bank logs deploy/analytics-sink --tail=3; } 2>&1 | redact > $P/logs/kubectl-eks.txt

echo "== Athena"
ath() { # run a SQL file in Athena and print the rows
  qid=$(aws athena start-query-execution --work-group "$WG" --query-execution-context Database=$DB --query-string "$(cat "$1")" --query QueryExecutionId --output text)
  for _ in $(seq 1 60); do
    st=$(aws athena get-query-execution --query-execution-id "$qid" --query QueryExecution.Status.State --output text)
    if [ "$st" = SUCCEEDED ] || [ "$st" = FAILED ]; then break; fi; sleep 2
  done
  echo "state=$st"
  aws athena get-query-results --query-execution-id "$qid" --output json --query 'ResultSet.Rows[].Data[].VarCharValue' | tr -d '\n '
  echo
}
{ echo "-- daily_volume.sql"; ath analytics/athena/daily_volume.sql; echo "-- top_accounts.sql"; ath analytics/athena/top_accounts.sql; } 2>&1 | redact > $P/logs/athena-daily-volume.txt
cat $P/logs/athena-daily-volume.txt

echo "== resources.txt"
{ echo "# terraform outputs"; $TF output; echo; echo "# terraform state resources"; $TF state list; echo; echo "# tagged resources"
  aws resourcegroupstaggingapi get-resources --tag-filters Key=Project,Values=event-ledger-bank --query 'ResourceTagMappingList[].ResourceARN' --output text | tr '\t' '\n' | sort; } 2>&1 | redact > $P/resources.txt

# CloudWatch graph rendered from the CLI (no console login)
LBNAME=$(aws elbv2 describe-load-balancers --query "LoadBalancers[0].LoadBalancerArn" --output text | sed 's|.*:loadbalancer/||')
if [ -n "$LBNAME" ] && [ "$LBNAME" != None ]; then
  aws cloudwatch get-metric-widget-image --output text --query MetricWidgetImage \
    --metric-widget "{\"metrics\":[[\"AWS/NetworkELB\",\"NewFlowCount\",\"LoadBalancer\",\"$LBNAME\"]],\"period\":60,\"stat\":\"Sum\",\"title\":\"NLB new flows\",\"width\":800,\"height\":300,\"start\":\"-PT1H\"}" \
    | tr -d "
 " | base64 -d > $P/screenshots/04-cloudwatch-nlb-flows.png || true
fi
echo "proof captured in $P"
