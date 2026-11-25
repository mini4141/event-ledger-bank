#!/usr/bin/env bash
# Phase 3: tear everything down. Idempotent; safe to re-run.
set -uo pipefail
source "$(dirname "$0")/lib.sh"; source "$ROOT/scripts/aws-env.sh"
cd "$ROOT"
aws sts get-caller-identity >/dev/null || { echo "no AWS access"; exit 1; }
out() { $TF output -raw "$1" 2>/dev/null; }
CLUSTER=$(out cluster_name); VPC=$(out vpc_id)

if [ -n "${CLUSTER:-}" ] && aws eks describe-cluster --name "$CLUSTER" >/dev/null 2>&1; then
  echo "== delete NLB service and namespaces"
  aws eks update-kubeconfig --name "$CLUSTER" --kubeconfig "$KUBECONFIG" >/dev/null
  kubectl -n bank delete svc envoy-public --ignore-not-found --wait=true --timeout=300s
  kubectl delete namespace bank --ignore-not-found --wait=true --timeout=300s
fi
if [ -n "${VPC:-}" ]; then
  echo "== wait for load balancers in the VPC to disappear"
  for _ in $(seq 1 60); do
    n=$(aws elbv2 describe-load-balancers --query "length(LoadBalancers[?VpcId=='$VPC'])" --output text 2>/dev/null || echo 0)
    c=$(aws elb describe-load-balancers --query "length(LoadBalancerDescriptions[?VPCId=='$VPC'])" --output text 2>/dev/null || echo 0)
    if [ "$n" = 0 ] && [ "$c" = 0 ]; then break; fi; sleep 10
  done
fi

echo "== empty S3 buckets, delete ECR images"
for b in $(aws s3api list-buckets --query "Buckets[?starts_with(Name,'event-ledger-bank-')].Name" --output text); do
  aws s3 rm "s3://$b" --recursive --quiet
done
for r in $(aws ecr describe-repositories --query "repositories[?starts_with(repositoryName,'event-ledger-bank/')].repositoryName" --output text 2>/dev/null); do
  ids=$(aws ecr list-images --repository-name "$r" --query 'imageIds[*]' --output json)
  if [ "$ids" != "[]" ]; then aws ecr batch-delete-image --repository-name "$r" --image-ids "$ids" >/dev/null; fi
done

echo "== terraform destroy"
for attempt in 1 2 3; do
  if [ -n "${VPC:-}" ]; then # security groups left by the in-tree load balancer controller block VPC deletion
    for sg in $(aws ec2 describe-security-groups --filters Name=vpc-id,Values="$VPC" --query "SecurityGroups[?starts_with(GroupName,'k8s-elb-')].GroupId" --output text); do
      aws ec2 delete-security-group --group-id "$sg" 2>/dev/null || true
    done
  fi
  if $TF destroy -input=false -auto-approve; then break; fi
  echo "destroy attempt $attempt failed, retrying"; sleep 30
done
echo "destroy finished"
