# PROGRESS

Resume from here if the session restarts. Current phase: **1 (local)**.

## Coverage matrix (SPECS §2)
- [x] Backend / distributed systems / resilience (5 Go services, idempotency, Envoy retries + outlier detection)
- [x] Go (all services)
- [ ] Cassandra (cqlsh proof) / Keyspaces (describe)
- [ ] Kafka async (consumer logs) / MSK Serverless
- [ ] Kubernetes + Docker (kind and EKS, Kustomize)
- [ ] Envoy for RPC (admin /clusters dump)
- [ ] AWS production infra (resources.txt, Athena result)
- [x] React internal tools (`web/`)
- [ ] Analytics sink + Athena queries

## Phase 0 scaffold
- [x] Repo layout, README, PROGRESS, DECISIONS, .gitignore, .gitattributes, .env.example, CI workflow

## Phase 1 local (SPECS §9)
- [x] Features F1-F4, F6 (compose stack, seeded, tests green)
- [ ] F5 Envoy drill
- [ ] kind deployment
- [x] tests/: go unit + integration JUnit, Playwright report
- [ ] screenshots 01-customers, 02-account-entries, 03-transfer, 04-live-feed, 05-system-health, 06-jaeger-transfer-trace
- [ ] logs: cqlsh-ledger, kafka-consume, envoy-clusters, duckdb-daily-volume, drill-ledger-restart, kind-get-all
- [ ] CI green on main, proof index complete

## Phase 2 AWS (SPECS §10)
- [ ] screenshots 01-eks-feed, 02-eks-account, 03-eks-system
- [ ] logs: keyspaces-describe.json, msk-describe.json, athena-daily-volume.txt, kubectl-eks.txt, resources.txt
- [ ] tests/playwright-eks/

## Phase 3 destroy (SPECS §11)
- [ ] destroy.sh run, verify-destroyed.txt clean

## Phase 4 write-up
- [ ] docs/PORTFOLIO.md, README final
