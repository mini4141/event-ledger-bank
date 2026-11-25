# PROGRESS

Resume from here if the session restarts. Current phase: **4 done: all phases complete (see below)**

> **Phase 2 note:** Keyspaces is denied for the deployer principal, so AWS runs Cassandra in EKS (see docs/DECISIONS.md).

## Coverage matrix (SPECS §2)
- [x] Backend / distributed systems / resilience (5 Go services, idempotency, Envoy retries + outlier detection)
- [x] Go (all services)
- [x] Cassandra (cqlsh proof); Keyspaces blocked by IAM (see DECISIONS), code path is config-only
- [x] Kafka async (consumer logs) + MSK Serverless
- [x] Kubernetes + Docker (kind) + EKS
- [x] Envoy for RPC (admin /clusters dump)
- [x] AWS production infra (EKS, MSK, S3, Athena; resources.txt, Athena result)
- [x] React internal tools (`web/`)
- [x] Analytics sink (DuckDB) + Athena

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
- [x] screenshots 01-eks-feed, 02-eks-account, 03-eks-system (+ 04 CloudWatch NLB graph)
- [x] logs: keyspaces-describe.json (Keyspaces denied: in-cluster Cassandra fallback), msk-describe.json, athena-daily-volume.txt, kubectl-eks.txt, resources.txt
- [x] tests/playwright-eks/

## Phase 3 destroy (SPECS §11)
- [x] destroy.sh run (37 resources destroyed), verify-destroyed.txt clean

## Phase 4 write-up
- [x] docs/PORTFOLIO.md, README final
