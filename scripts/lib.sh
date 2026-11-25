# shellcheck shell=bash
# Shared helpers. Source from other scripts.
export MSYS_NO_PATHCONV=1   # Git Bash: do not rewrite /opt/... paths passed to docker
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export PATH="$PATH:$(go env GOPATH 2>/dev/null)/bin:/c/Users/ThinkPad/go/bin:/c/Program\ Files/Docker/Docker/resources/bin"
BASE_URL="${BASE_URL:-http://localhost:18080}"
# jfield <json> <name>: first string value of "name" in a JSON document
jfield() { printf '%s' "$1" | sed -n "s/.*\"$2\":\"\([^\"]*\)\".*/\1/p" | head -1; }
# redact: scrub account ids, keys and tokens from text before saving it as proof
redact() { sed -E 's/[0-9]{12}/<ACCOUNT_ID>/g; s/(AKIA|ASIA)[A-Z0-9]{16}/<ACCESS_KEY>/g'; }
# DuckDB CLI (winget installs it outside PATH of already-open shells)
duckdb_bin() { command -v duckdb || ls /c/Users/ThinkPad/AppData/Local/Microsoft/WinGet/Packages/DuckDB*/duckdb.exe | head -1; }
# wait_http <url> [tries]
wait_http() { for _ in $(seq 1 "${2:-60}"); do curl -fsS "$1" >/dev/null 2>&1 && return 0; sleep 2; done; return 1; }
