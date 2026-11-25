# Proof index

Every artifact is captured by a script from the running system (see `scripts/capture-proof.sh`, `scripts/capture-cloud.sh`). AWS account IDs, keys and tokens are redacted.

## Phase 1: local (Docker Compose + kind)

| Artifact | What it proves |
|---|---|
| [tests/go-unit-junit.xml](phase1-local/tests/go-unit-junit.xml), [go-unit.log](phase1-local/tests/go-unit.log) | Ledger posting balances to zero; idempotent replay returns the original transfer |
| [tests/go-integration-junit.xml](phase1-local/tests/go-integration-junit.xml) | gRPC through Envoy → Cassandra → Kafka: transfer, replay, balances 75.00 / 25.00, `transfer.completed` consumed |
| [tests/grpcurl-smoke.log](phase1-local/tests/grpcurl-smoke.log) | grpcurl call through Envoy's mesh listener |
| [tests/playwright-report/](phase1-local/tests/playwright-report/index.html), [playwright-junit.xml](phase1-local/tests/playwright-junit.xml) | Browser happy path: create → deposit → transfer → live feed |
| [tests/buf-lint.log](phase1-local/tests/buf-lint.log) | Protobuf contract lint |
| [screenshots/01-customers.png](phase1-local/screenshots/01-customers.png) | React Customers page with accounts and balances |
| [screenshots/02-account-entries.png](phase1-local/screenshots/02-account-entries.png) | Account detail with balanced double-entry rows |
| [screenshots/03-transfer.png](phase1-local/screenshots/03-transfer.png) | Transfer form and result (idempotency key shown) |
| [screenshots/04-live-feed.png](phase1-local/screenshots/04-live-feed.png) | Live event feed (SSE via Envoy) showing the transfer |
| [screenshots/05-system-health.png](phase1-local/screenshots/05-system-health.png) | Service health and Envoy cluster status through Envoy |
| [screenshots/06-jaeger-transfer-trace.png](phase1-local/screenshots/06-jaeger-transfer-trace.png) | OpenTelemetry trace: transfers → Envoy → ledger |
| [logs/cqlsh-ledger.txt](phase1-local/logs/cqlsh-ledger.txt) | Rows in Cassandra: accounts, ledger entries, idempotency keys |
| [logs/kafka-consume.txt](phase1-local/logs/kafka-consume.txt) | Topics, consumer group offsets and the notifications consumer log |
| [logs/envoy-clusters.txt](phase1-local/logs/envoy-clusters.txt) | Envoy admin cluster dump and retry/outlier stats |
| [logs/duckdb-daily-volume.txt](phase1-local/logs/duckdb-daily-volume.txt) | Analytics: DuckDB over the sink's NDJSON (daily volume, top accounts) |
| [logs/kind-get-all.txt](phase1-local/logs/kind-get-all.txt) | Same images running on Kubernetes (kind) |
| [logs/drill-ledger-restart.log](phase1-local/logs/drill-ledger-restart.log) | Ledger pod force-deleted under load: Envoy retries > 0, 100% success |

## Phase 2: AWS

See [phase2-cloud/](phase2-cloud/): screenshots of the app behind the NLB, Keyspaces/MSK describe output, the Athena result over S3, `kubectl` output, `resources.txt` and the Playwright run against EKS.

## Phase 3: destroy

[phase3-destroy/verify-destroyed.txt](phase3-destroy/verify-destroyed.txt) lists the checks that show nothing billable remains.
