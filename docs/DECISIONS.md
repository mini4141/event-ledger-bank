# Decisions

- 2026-11-26 Single Go module at repo root (`internal/` shared packages, `services/*` mains): simplest layout that keeps all five services buildable by one Dockerfile.
- 2026-11-26 Installed Go 1.27 on host; `go.mod` pins `go 1.25` and images use golang:1.25. Tools installed: buf, protoc-gen-go, protoc-gen-go-grpc, grpcurl (go install), DuckDB CLI (winget).
- 2026-11-26 Host ports 8080/9090/16686/3000 are used by an unrelated project's containers, so this repo maps Envoy to 18080, Jaeger UI to 26686, Prometheus to 29090.
- 2026-11-26 One `envoy/envoy.yaml` for compose, kind and EKS: service DNS names are identical in all three, so a separate envoy-eks.yaml would be a duplicate.
- 2026-11-26 Envoy also serves the web bundle (route `/` -> nginx `web` cluster) so the app and REST API share one origin; no CORS.
- 2026-11-26 `topic-init` Go binary (franz-go kadm) creates topics in compose, kind and MSK.
- 2026-11-26 `ledger.Post` records an idempotent `postings_by_id` marker (IF NOT EXISTS) so Envoy retries cannot double-post.
- 2026-11-26 Dependencies (otel, grpc, aws sdk) require Go 1.26, so go.mod says 1.26 and images use golang:1.26-alpine instead of 1.25; same language family, closest viable equivalent.
- 2026-11-26 Amazon Keyspaces: the deployer user (PowerUserAccess + IAMFullAccess) is denied cassandra:* (CreateKeyspace/ListKeyspaces return AccessDenied). I did not grant the user extra permissions to itself. Terraform keeps the Keyspaces resources behind enable_keyspaces; deploy.sh probes access and, when denied, runs the same services against Cassandra 5 as an in-cluster StatefulSet (overlay eks-cassandra, config only). To use Keyspaces, attach AmazonKeyspacesFullAccess to the deployer and redeploy.
- 2026-11-26 Ledger/accounts/transfers Services are headless so Envoy resolves pod IPs and retries/outlier detection work per pod (needed for the drill).
- 2026-11-26 Route timeouts: 1 s on ledger routes as specified, 3 s on accounts/transfers routes because they wrap ledger calls (outer must exceed inner).
