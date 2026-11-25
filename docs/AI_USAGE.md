# AI usage

This project was built with Claude Code (autonomous agent) following `CLAUDE.md` and `SPECS.md`.

- **What the AI did:** wrote the Go services, protobuf contracts, Envoy/Kubernetes/Terraform configuration, the React app, scripts and docs.
- **How output was reviewed:** every change was run, not just read. `scripts/test-all.sh` runs buf lint, Go unit tests, Go integration tests against real Cassandra and Kafka through Envoy, a grpcurl smoke test and a Playwright browser test. CI runs the same script.
- **Proof over claims:** `proof/` holds logs and screenshots captured by scripts from the running system (local, AWS and the destroy check), so results can be reproduced.
- **Human decisions:** the stack, phases and guardrails come from the owner's CLAUDE.md. Deviations are logged in `docs/DECISIONS.md`.
