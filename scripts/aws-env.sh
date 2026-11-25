# shellcheck shell=bash
# Sourced by deploy/destroy/verify scripts: pins the AWS profile and region.
export AWS_PROFILE=event-ledger-bank-deployer
export AWS_REGION=ap-southeast-2 AWS_DEFAULT_REGION=ap-southeast-2
export AWS_PAGER=""
export KUBECONFIG="$ROOT/.local/kubeconfig"
TF="terraform -chdir=$ROOT/infra"
