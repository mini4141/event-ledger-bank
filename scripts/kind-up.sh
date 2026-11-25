#!/usr/bin/env bash
# Deploy the same images to a local kind cluster with Kustomize (Cassandra + Kafka as StatefulSets).
set -euo pipefail
source "$(dirname "$0")/lib.sh"
cd "$ROOT"
docker compose down >/dev/null 2>&1 || true   # free ports/memory: compose and kind share 18080
for s in accounts ledger transfers notifications analytics-sink topic-init; do
  docker build -q --build-arg SERVICE=$s -t elb/$s:dev .
done
docker build -q -t elb/web:dev web
docker build -q -t elb/envoy:dev envoy
kind get clusters | grep -qx elb || kind create cluster --config deploy/kind-config.yaml
for i in accounts ledger transfers notifications analytics-sink topic-init web envoy; do kind load docker-image --name elb elb/$i:dev; done
kubectl create namespace bank --dry-run=client -o yaml | kubectl apply -f -
kubectl -n bank create configmap cql-schema --from-file=db/cql/schema.cql --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -k deploy/k8s/overlays/kind
kubectl -n bank rollout status deploy/envoy deploy/accounts deploy/ledger deploy/transfers deploy/notifications deploy/web --timeout=300s
wait_http "$BASE_URL/healthz/ledger" 60
echo "kind stack is up: $BASE_URL"
