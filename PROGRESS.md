# PROGRESS

Resume from here if the session restarts. Current phase: **2 (AWS deploy started 2026-11-26 ~03:05 local; destroy within 2h)**.

## Coverage matrix (SPECS §2)
- [x] Backend / distributed systems / resilience (5 Go services, idempotency, Envoy retries + outlier detection)
- [x] Go (all services)
- [x] Cassandra (cqlsh proof) [ ] Keyspaces (describe)
- [x] Kafka async (consumer logs) [ ] MSK Serverless
- [x] Kubernetes + Docker (kind) [ ] EKS
- [x] Envoy for RPC (admin /clusters dump)
- [ ] AWS production infra (resources.txt, Athena result)
- [x] React internal tools (`web/`)
- [x] Analytics sink (DuckDB) [ ] Athena

## Phase 0 scaffold
- [x] Repo layout, README, PROGRESS, DECISIONS, .gitignore, .gitattributes, .env.example, CI workflow

## Phase 1 local (SPECS §9)
- [x] Features F1-F4, F6 (compose stack, seeded, tests green)
- [x] F5 Envoy drill (retries 3, 100% success)
- [x] kind deployment
- [x] tests/: go unit + integration JUnit, Playwright report
- [x] screenshots 01-06
- [x] logs: cqlsh-ledger, kafka-consume, envoy-clusters, duckdb-daily-volume, drill-ledger-restart, kind-get-all
- [x] CI green on main, proof index complete

## Phase 2 AWS (SPECS §10)
- [ ] screenshots 01-eks-feed, 02-eks-account, 03-eks-system
- [ ] logs: keyspaces-describe.json, msk-describe.json, athena-daily-volume.txt, kubectl-eks.txt, resources.txt
- [ ] tests/playwright-eks/

## Phase 3 destroy (SPECS §11)
- [ ] destroy.sh run, verify-destroyed.txt clean

## Phase 4 write-up
- [ ] docs/PORTFOLIO.md, README final
