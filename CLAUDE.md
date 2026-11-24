# CLAUDE.md — event-ledger-bank

These are the engineering rules for Claude Code on this repository. You work **fully autonomously**. Don't stop to ask the user questions. When something is ambiguous, pick the simplest option that keeps every stack item in §2 covered. Record it in `docs/DECISIONS.md` (one line: date, decision, reason) and keep going. Read `SPECS.md` end to end before writing any code. It is the source of truth for features, architecture and proof.

## 1. Project at a glance

| | |
|---|---|
| Project | Event Ledger Bank: a Go microservices bank on Cassandra, Kafka, Envoy and Kubernetes |
| Repo | public GitHub repo `event-ledger-bank`. `origin` is already configured and the default branch is `main` |
| Target role | Backend Engineer (Go, distributed systems, banking platform) |
| Pitch | A small but real banking backend: Go services talk gRPC through Envoy, persist a double-entry ledger in Cassandra, publish every movement to Kafka, run on Kubernetes, and expose a React internal-tools app. The cloud version swaps in Amazon Keyspaces and MSK Serverless. |
| Priority | **Breadth of the tech stack beats depth of business logic.** Every technology in §2 must appear in running code and be visible in the proof. Full validation, edge cases and complete domain rules are out of scope. |

## 2. Mandatory tech stack

| Area | Technology |
|---|---|
| Language | Go 1.25 (one module per service, or a `go.work` workspace) |
| RPC | gRPC + Protobuf managed with `buf` (lint, breaking, generate), **Envoy Proxy** as the front proxy (gRPC routing, retries, timeouts, outlier detection, gRPC-JSON transcoding for the web app) |
| Storage | Apache Cassandra 5 (local, gocql) → **Amazon Keyspaces** in AWS (gocql + `aws-sigv4-auth-cassandra-gocql-driver-plugin`, TLS, LOCAL_QUORUM); lightweight transactions for idempotency |
| Messaging | Apache Kafka 4 (official `apache/kafka` image, KRaft) locally → **Amazon MSK Serverless** (IAM auth) in AWS; Go client `franz-go` (with its AWS MSK IAM SASL mechanism) |
| Containers / orchestration | Docker, Kubernetes: kind locally, **Amazon EKS** in AWS; Kustomize base + overlays |
| Web | React 19 + TypeScript + Vite internal tools app (customers, accounts, transfer, live event feed) |
| Data / analytics | Kafka → `analytics-sink` (Go) → NDJSON in S3 → **Amazon Athena** + Glue table (the AWS equivalent of the post's GCP data infrastructure); locally the sink writes to a folder and DuckDB queries it |
| Observability | OpenTelemetry tracing (Jaeger locally), Prometheus metrics, structured JSON logs (`slog`) |
| Tests | Go `testing` + `testify`, integration tests against compose Cassandra/Kafka, Playwright (web) |
| AWS | EKS (spot), ECR, Keyspaces, MSK Serverless, S3, Athena, Glue, IAM (EKS Pod Identity), CloudWatch |
| IaC / CI | Terraform ≥ 1.9 · GitHub Actions (buf lint, go test, web build) |

Don't swap an item for an alternative. If a library is unavailable or broken, use the closest equivalent and log the swap in `docs/DECISIONS.md`.

## 3. Environment

- The host is a **Windows 11 PC**. Claude Code's shell is Git Bash, and the container runtime is Docker Desktop (WSL2 backend). Write automation as POSIX `bash` scripts under `scripts/` and use forward-slash paths. Add a `.gitattributes` with `* text=auto eol=lf` so scripts keep LF line endings.
- Prerequisites: Docker Desktop, kind, kubectl, Go 1.25, buf, Node.js 24 + pnpm, Terraform, AWS CLI v2, DuckDB CLI (optional). If one is missing, install it with `winget` or the official installer and record the version in `docs/DECISIONS.md`.
- The `gh` CLI is logged in to the owner's GitHub account, and AWS CLI v2 is installed.
- Don't use Bitnami charts or images (their free catalogue has been retired). Use official upstream images and charts.

## 4. AWS account rules

- Profile: **`event-ledger-bank-deployer`**. Always pass it explicitly, either as `--profile event-ledger-bank-deployer` or by exporting `AWS_PROFILE=event-ledger-bank-deployer` inside the scripts. Never use the default profile or any other profile.
- Region: **`ap-southeast-2`** for everything except global services such as CloudFront and IAM.
- The profile has `PowerUserAccess` + `IAMFullAccess`. Start Phase 2 with `aws sts get-caller-identity --profile event-ledger-bank-deployer`. If that fails, stop and write the reason at the top of `PROGRESS.md`.
- Use AWS only. Where a requirement names Azure, GCP or another hyperscaler, use the AWS equivalent stated in `SPECS.md`.
- IaC: Terraform (`infra/`): VPC public subnets, EKS + spot node group, ECR, Keyspaces keyspace + tables, MSK Serverless cluster, S3 + Glue + Athena workgroup, Pod Identity roles. Don't create resources by hand in the console: every cloud resource comes from code in `infra/`. Keep IaC state local and gitignored.
- Tag every resource with `Project=event-ledger-bank`, `Ephemeral=true`, `ManagedBy=<tool>` and `Owner=alvieirvansyah`.
- Deployment runs **from this PC via scripts** (`scripts/deploy.sh`, `scripts/destroy.sh`). GitHub Actions only builds and tests, and never holds AWS credentials.

## 5. Phases and gates

Work strictly in order. A phase is done only when its proof exists (§8) and its items are ticked in `PROGRESS.md`. Commit and push at the end of every phase.

1. **Phase 0, scaffold.** Set up the repo layout from `SPECS.md` §4. Create `README.md`, `PROGRESS.md` (copy every checklist item from `SPECS.md` §2 and §9–§11 into it), `docs/DECISIONS.md`, `.gitignore`, `.gitattributes`, `.env.example` and the CI workflow.
2. **Phase 1, local.** Build every feature in `SPECS.md` §5. Everything runs on this PC with Docker and local tooling, and makes **no AWS calls**: use the local stand-ins listed in `SPECS.md`. Run the full test suite, then capture proof into `proof/phase1-local/`. Phase 1 counts as "ready for next" only when `scripts/test-all.sh` is green, the CI run on `main` is green, and the proof index is complete.
3. **Phase 2, AWS showcase.** Deploy with `scripts/deploy.sh`, run the smoke/e2e tests against the cloud endpoints and capture proof into `proof/phase2-cloud/`. Keep the stack alive only as long as capturing takes, with a target of **under 2 hours**.
4. **Phase 3, destroy.** Right after capturing, run `scripts/destroy.sh` and then `scripts/verify-destroyed.sh`, and save the output to `proof/phase3-destroy/`. This phase is **mandatory even if Phase 2 partly failed**. Never end a session with cloud resources running.
5. **Phase 4, write-up.** Write `docs/PORTFOLIO.md`. It needs the problem, an architecture diagram (Mermaid), a stack → requirement table, the key screenshots (relative links into `proof/`), what was hard, the approximate AWS cost and how to reproduce. This file becomes a technical page on alvieirvansyah.com.

## 6. Engineering rules

- Keep it simple and conventional: idiomatic layout for each language, small modules, no premature abstraction, no dead code.
- Configure through environment variables, with a committed `.env.example`. Real `.env` files are gitignored.
- **Never commit secrets.** That includes credentials, IaC state, `.terraform/`, `cdk.out/`, kubeconfigs, private keys and certificates. Generate local dev keys and certs into a gitignored `.local/` folder.
- Every HTTP service exposes a health endpoint (`/healthz` or the framework idiom) and logs structured JSON to stdout.
- Pin versions: commit lockfiles and pin container images to at least a minor tag.
- Use one command for each job:
  - `scripts/dev-up.sh` starts everything locally and `scripts/dev-down.sh` stops it.
  - `scripts/seed.sh` loads demo data. Start-up may run it automatically so that demos never start empty.
  - `scripts/test-all.sh` runs every test layer.
  - `scripts/capture-proof.sh <phase>` captures the proof for a phase.
- Keep `README.md` current. It covers what the project is, the architecture diagram, a quickstart, deploy/destroy, a link to `proof/README.md` and `docs/PORTFOLIO.md`, and the CI badge.
- Don't put the hiring company's name anywhere in the repo. Describe the target role generically.
- `docs/AI_USAGE.md`: briefly note how AI coding tools were used and how their output was reviewed and tested.

## 7. Testing rules

- Tests are **shallow by design**: one happy-path test per feature or flow is enough. Don't write exhaustive validation or edge-case tests.
- Required layers: Go unit (ledger posting balances to zero, idempotency key logic), Go integration (transfer through gRPC against compose Cassandra + Kafka, event consumed), Playwright (web: create two accounts → transfer → feed shows event), plus a grpcurl smoke through Envoy.
- `scripts/test-all.sh` runs every local layer and exits non-zero on failure. CI runs every layer that doesn't need the cloud.
- Save test output as files (JUnit XML, an HTML report or a plain log) under the current phase's `proof/.../tests/`.

## 8. Proof rules

Proof is the deliverable. Use this folder layout:

```
proof/
  README.md                 # index: each artifact on one line, with what it proves
  phase1-local/
    screenshots/            # PNG, named NN-what-it-shows.png
    logs/                   # captured service logs and CLI output
    tests/                  # test reports
  phase2-cloud/
    screenshots/  logs/  tests/
    resources.txt           # deployed AWS resources (from IaC outputs / describe calls)
  phase3-destroy/
    verify-destroyed.txt
```

- Capture screenshots with a script (`scripts/capture-proof.sh`, driving Playwright headless), never by hand, so they can be reproduced. The screenshots listed in `SPECS.md` are the minimum.
- There is no AWS console login, so render AWS-side visuals from the CLI. For example, `aws cloudwatch get-metric-widget-image` renders a metrics graph as a PNG. Otherwise save CLI `describe`/`get` output as logs.
- Before saving logs or CLI output, redact 12-digit AWS account IDs (`sed -E 's/[0-9]{12}/<ACCOUNT_ID>/g'`), access keys, tokens, passwords and the deployer's public IP.
- Proof must show the system **working**, with real data flowing through it, and not just "deployed".

## 9. Cost and safety guardrails (AWS)

- Use the smallest viable sizes: EKS node group 2× `t3.medium` SPOT, MSK **Serverless** (single cluster, 3 topics with low partition counts), Keyspaces on-demand, Athena workgroup with a 1 GB bytes-scanned limit, one NLB.
- **No NAT gateways.** Also no multi-AZ databases, provisioned IOPS, reserved capacity or Savings Plans, Route 53 domains, or Marketplace subscriptions.
- Databases: `deletion_protection = false`, skip the final snapshot and keep backup retention minimal. S3 buckets are force-destroyable and ECR repos force-deletable. CloudWatch log groups get 1-day retention and are created by IaC, so they get destroyed with it.
- Public app endpoints are fine for the showcase. Keep databases private: their security groups admit only the app tier. Don't open SSH or database ports to `0.0.0.0/0` and don't use SSH at all. Use SSM if you need a shell.
- Don't send real emails or SMS, and don't call paid third-party APIs. Anything external is mocked unless `SPECS.md` says otherwise.
- `scripts/destroy.sh` must be idempotent and safe to re-run. It first deletes anything that would block the IaC destroy: delete the Envoy `LoadBalancer` Service and wait for the NLB to disappear, delete the Kubernetes namespaces, empty the S3 buckets (data + Athena results), delete the ECR images.
- `scripts/verify-destroyed.sh` lists anything still tagged `Project=event-ledger-bank` (`aws resourcegroupstaggingapi get-resources`) and runs the specific checks in `SPECS.md` §11. It exits non-zero if anything billable remains. The tagging API can show resources for a few minutes after deletion, so confirm each hit with a direct describe call before failing.

## 10. Git and CI

- Use Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `infra:`, `chore:`). Commit small and often, and push to `origin main` at least at the end of every phase.
- `.github/workflows/ci.yml` runs build, lint and the cloud-free tests on every push and pull request. Keep it green.
- Never force-push `main`. Never commit build output, dependency folders (`node_modules`, `target`, `bin`, `obj`, `.venv`) or state files.

## 11. Project-specific rules

- Services: `accounts`, `ledger`, `transfers`, `notifications`, `analytics-sink`, plus `envoy` and `web`. Every inter-service call goes **through Envoy** (sidecar-less front proxy with clusters per service), so retries/timeouts live in Envoy config, not in Go code.
- Ledger is **double-entry**: each transfer writes two entries (debit + credit) sharing a `posting_id`, and the balance is maintained with a conditional update (LWT on `version`). Idempotency: `transfers_by_idempotency_key` inserted with `IF NOT EXISTS`. Document Cassandra data modelling (query-first tables) in `docs/data-model.md`.
- Same code, two backends: Cassandra vs Keyspaces and Kafka vs MSK differ only by config (`CASSANDRA_AUTH=none|sigv4`, `KAFKA_SASL=none|aws-msk-iam`). Keyspaces table creation is asynchronous, so Terraform creates the tables and the app never runs DDL in AWS.
- Events are protobuf-encoded on Kafka topics `ledger.posted`, `transfer.completed` and `transfer.failed`, keyed by account ID for ordering. Consumers commit offsets only after processing.
- The web app talks only to Envoy's JSON-transcoded REST routes (no gRPC-web dependency needed). The live feed uses Server-Sent Events from `notifications`, routed by Envoy.
- Interview reality check: the post says Go, Cassandra, Kafka, Kubernetes, Envoy, AWS, React. Every one of these must be visible in a screenshot or log in both phases.

## 12. Definition of done

- [ ] Every stack item in §2 is used in running code. Cross-check with the coverage matrix in `SPECS.md` §2 and tick each row in `PROGRESS.md`.
- [ ] Phase 1 proof is complete, and all tests are green locally and in CI.
- [ ] Phase 2 proof is complete: cloud screenshots, a cloud test run and the resource list.
- [ ] Phase 3: everything is destroyed and `verify-destroyed.txt` shows nothing billable left.
- [ ] `docs/PORTFOLIO.md` and `README.md` are written, and everything is committed and pushed.
