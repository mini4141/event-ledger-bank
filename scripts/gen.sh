#!/usr/bin/env bash
# Regenerate Go code and the Envoy transcoder descriptor from proto/.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$PATH:$(go env GOPATH)/bin"
(cd proto && buf dep update)
buf generate
buf build proto -o envoy/descriptor.binpb
