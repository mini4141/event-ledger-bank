#!/usr/bin/env bash
# Phase 2: deploy to AWS (EKS, Keyspaces, MSK Serverless, S3/Athena) from this PC.
set -euo pipefail
source "$(dirname "$0")/lib.sh"; source "$ROOT/scripts/aws-env.sh"
cd "$ROOT"; mkdir -p .local
aws sts get-caller-identity >/dev/null

# Amazon Keyspaces needs cassandra:* on the deployer; fall back to in-cluster Cassandra when it is denied.
if aws keyspaces list-keyspaces >/dev/null 2>&1; then KS=true; OVERLAY=eks; else KS=false; OVERLAY=eks-cassandra; fi
export TF_VAR_enable_keyspaces=$KS
echo "keyspaces=$KS overlay=$OVERLAY" | tee .local/deploy-mode

echo "== terraform apply"
$TF init -input=false >/dev/null
$TF apply -input=false -auto-approve
out() { $TF output -raw "$1"; }
ECR=$(out ecr_registry); CLUSTER=$(out cluster_name); S3_BUCKET=$(out data_bucket); MSK_ARN=$(out msk_cluster_arn)
REGISTRY=${ECR%%/*}

echo "== build + push images to ECR"
aws ecr get-login-password | docker login --username AWS --password-stdin "$REGISTRY" >/dev/null
for s in accounts ledger transfers notifications analytics-sink topic-init; do
  docker build -q --build-arg SERVICE=$s -t "$ECR/$s:latest" . && docker push -q "$ECR/$s:latest"
done
docker build -q -t "$ECR/web:latest" web && docker push -q "$ECR/web:latest"
docker build -q -t "$ECR/envoy:latest" envoy && docker push -q "$ECR/envoy:latest"

echo "== kubeconfig + apply"
aws eks update-kubeconfig --name "$CLUSTER" --kubeconfig "$KUBECONFIG" >/dev/null
MSK_BOOTSTRAP=$(aws kafka get-bootstrap-brokers --cluster-arn "$MSK_ARN" --query BootstrapBrokerStringSaslIam --output text)
export ECR MSK_BOOTSTRAP S3_BUCKET REGION=$AWS_REGION
kubectl create namespace bank --dry-run=client -o yaml | kubectl apply -f -
if [ "$KS" = false ]; then kubectl -n bank create configmap cql-schema --from-file=db/cql/schema.cql --dry-run=client -o yaml | kubectl apply -f - ; fi
kubectl kustomize deploy/k8s/overlays/$OVERLAY | envsubst '${ECR} ${MSK_BOOTSTRAP} ${S3_BUCKET} ${REGION}' | kubectl apply -f -

echo "== wait for rollout"
if [ "$KS" = false ]; then kubectl -n bank wait --for=condition=complete job/cassandra-schema --timeout=600s; fi
kubectl -n bank wait --for=condition=complete job/topic-init --timeout=300s
kubectl -n bank rollout status deploy/envoy deploy/accounts deploy/ledger deploy/transfers deploy/notifications deploy/analytics-sink deploy/web --timeout=600s
echo "waiting for NLB hostname..."
for _ in $(seq 1 180); do
  LB=$(kubectl -n bank get svc envoy-public -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || true)
  [ -n "$LB" ] && break; sleep 5
done
export BASE_URL="http://$LB"; echo "$BASE_URL" > .local/cloud-url
wait_http "$BASE_URL/healthz/ledger" 240 || { echo "NLB not serving"; exit 1; }

echo "== seed + smoke + proof"
"$ROOT/scripts/seed.sh"
curl -fsS -X POST "$BASE_URL/v1/customers" -H 'Content-Type: application/json' -d '{"name":"cloud-smoke"}' | grep -q customer_id && echo "SMOKE OK"
sleep 15   # let analytics-sink flush to S3
"$ROOT/scripts/capture-cloud.sh"
