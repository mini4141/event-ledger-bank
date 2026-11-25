# shellcheck shell=bash
# Sourced by deploy/destroy/verify scripts: pins the AWS profile and region.
export AWS_PROFILE=event-ledger-bank-deployer
export AWS_REGION=ap-southeast-2 AWS_DEFAULT_REGION=ap-southeast-2
export AWS_PAGER=""
WINROOT="$(cygpath -m "$ROOT" 2>/dev/null || echo "$ROOT")"   # native Windows tools need C:/ style paths
export KUBECONFIG="$WINROOT/.local/kubeconfig"
TF="terraform -chdir=$WINROOT/infra"
