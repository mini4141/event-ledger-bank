#!/usr/bin/env bash
# Run every local test layer; saves reports to proof/phase1-local/tests/. Non-zero exit on failure.
set -euo pipefail
source "$(dirname "$0")/lib.sh"
cd "$ROOT"
OUT="${TEST_OUT:-proof/phase1-local/tests}"
mkdir -p "$OUT"

echo "== contract: buf lint + breaking"
(cd proto && buf lint) 2>&1 | tee "$OUT/buf-lint.log"
if git cat-file -e main:proto/buf.yaml 2>/dev/null; then
  buf breaking proto --against ".git#branch=main,subdir=proto" 2>&1 | tee "$OUT/buf-breaking.log"
else
  echo "no proto on main yet: nothing to compare against" | tee "$OUT/buf-breaking.log"
fi

echo "== unit"
go vet ./... 
go test ./internal/... -count=1 -v 2>&1 | tee "$OUT/go-unit.log" | go-junit-report -set-exit-code > "$OUT/go-unit-junit.xml"

if ! curl -fsS "$BASE_URL/healthz/ledger" >/dev/null 2>&1; then
  echo "== starting compose stack"; SEED=0 "$ROOT/scripts/dev-up.sh"
fi

echo "== integration (gRPC via Envoy + Cassandra + Kafka)"
go test -tags=integration ./tests/... -count=1 -v 2>&1 | tee "$OUT/go-integration.log" | go-junit-report -set-exit-code > "$OUT/go-integration-junit.xml"

echo "== grpcurl smoke through Envoy"
"$ROOT/scripts/smoke.sh" 2>&1 | tee "$OUT/grpcurl-smoke.log"

echo "== web build + Playwright e2e"
(cd web && pnpm build && BASE_URL="$BASE_URL" npx playwright test) 2>&1 | tee "$OUT/playwright.log"
rm -rf "$OUT/playwright-report"; cp -r web/playwright-report "$OUT/playwright-report"; cp web/test-results/junit.xml "$OUT/playwright-junit.xml"
echo "ALL TESTS PASSED"
