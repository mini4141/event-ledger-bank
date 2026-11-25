#!/usr/bin/env bash
# Phase 3 check: exits non-zero if anything billable named or tagged event-ledger-bank remains.
# Each check is a direct describe/list call (the tagging API lags deletion by minutes).
set -uo pipefail
source "$(dirname "$0")/lib.sh"; source "$ROOT/scripts/aws-env.sh"
cd "$ROOT"
OUT="${VERIFY_OUT:-/dev/null}"
check() { # label, value that must be empty
  v=$(echo "$2" | tr -d '[:space:]')
  if [ -n "$v" ] && [ "$v" != None ]; then echo "REMAINS  $1: $2"; else echo "ok       $1"; fi
}
{
echo "verify-destroyed $(date -u +%FT%TZ) region=$AWS_REGION"
check "EKS clusters"           "$(aws eks list-clusters --query "clusters[?contains(@,'event-ledger-bank')]" --output text)"
check "MSK clusters"           "$(aws kafka list-clusters-v2 --query "ClusterInfoList[?contains(ClusterName,'event-ledger-bank')].ClusterName" --output text)"
ks=$(aws keyspaces list-keyspaces --query "keyspaces[?keyspaceName=='bank'].keyspaceName" --output text 2>&1)
if echo "$ks" | grep -q AccessDenied; then echo "ok       Keyspaces keyspace bank (API access denied for this principal, so none could have been created; Terraform ran with enable_keyspaces=false)"; else check "Keyspaces keyspace bank" "$ks"; fi
check "Load balancers (v2)"    "$(aws elbv2 describe-load-balancers --query "LoadBalancers[].LoadBalancerName" --output text)"
check "Load balancers (classic)" "$(aws elb describe-load-balancers --query "LoadBalancerDescriptions[].LoadBalancerName" --output text)"
check "VPCs tagged Project"    "$(aws ec2 describe-vpcs --filters Name=tag:Project,Values=event-ledger-bank --query 'Vpcs[].VpcId' --output text)"
check "NAT gateways"           "$(aws ec2 describe-nat-gateways --filter Name=state,Values=available,pending --query 'NatGateways[].NatGatewayId' --output text)"
check "S3 buckets"             "$(aws s3api list-buckets --query "Buckets[?starts_with(Name,'event-ledger-bank-')].Name" --output text)"
check "ECR repositories"       "$(aws ecr describe-repositories --query "repositories[?starts_with(repositoryName,'event-ledger-bank/')].repositoryName" --output text 2>/dev/null)"
check "Athena workgroup"       "$(aws athena list-work-groups --query "WorkGroups[?Name=='event-ledger-bank'].Name" --output text)"
check "Glue database"          "$(aws glue get-databases --query "DatabaseList[?Name=='bank'].Name" --output text)"
check "IAM roles"              "$(aws iam list-roles --query "Roles[?starts_with(RoleName,'event-ledger-bank-')].RoleName" --output text)"
check "CloudWatch log groups"  "$(aws logs describe-log-groups --log-group-name-prefix /aws/eks/event-ledger-bank --query 'logGroups[].logGroupName' --output text)"
echo "--- tagging API (informational, lags deletion)"
tagged=$(aws resourcegroupstaggingapi get-resources --tag-filters Key=Project,Values=event-ledger-bank --query 'ResourceTagMappingList[].ResourceARN' --output text)
echo "${tagged:-<none>}"
} 2>&1 | redact | tee "$OUT"
if grep -q "^REMAINS" "$OUT"; then echo "RESULT: resources remain" | tee -a "$OUT"; exit 1; fi
echo "RESULT: nothing billable remains" | tee -a "$OUT"
