#!/usr/bin/env bash
# grpcurl smoke through Envoy's mesh listener (GRPC_ADDR, default localhost:18081).
set -euo pipefail
source "$(dirname "$0")/lib.sh"
ADDR="${GRPC_ADDR:-localhost:18081}"
PS="$ROOT/envoy/descriptor.binpb"
out=$(grpcurl -plaintext -protoset "$PS" -d '{"name":"smoke"}' "$ADDR" bank.v1.AccountsService/CreateCustomer)
echo "$out"
echo "$out" | grep -q customerId && echo "SMOKE OK"
