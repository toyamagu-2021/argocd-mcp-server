#!/usr/bin/env bash
# Shared paths for the argocd-mcp-server verification helpers.
# Source it; do not execute it for a drive.
#   source .cursor/skills/verify-argocd-mcp-server/scripts/lib.sh

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  echo "source .cursor/skills/verify-argocd-mcp-server/scripts/lib.sh"
  exit 0
fi

_lib_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
SKILL_DIR=$(cd "$_lib_dir/.." && pwd)
REPO_ROOT=$(cd "$_lib_dir/../../../.." && pwd)
STATE_DIR="$REPO_ROOT/.verify-argocd-mcp-server"
EVIDENCE_ROOT="$SKILL_DIR/evidence"
SCRIPT_DIR="$_lib_dir"

CLUSTER_NAME=argocd-mcp-server
EXPECTED_CONTEXT=kind-argocd-mcp-server
ARGOCD_NAMESPACE=argocd
ARGOCD_HOST=127.0.0.1
ARGOCD_PORT=8080
ARGOCD_URL="https://${ARGOCD_HOST}:${ARGOCD_PORT}"

die() {
  printf 'VERIFY_FAIL: %s\n' "$*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "required command not on PATH: $1"
}

load_run_id() {
  [[ -f "$STATE_DIR/run-id" ]] || die "no run id at $STATE_DIR/run-id; run scripts/launch.sh first"
  RUN_ID=$(tr -d '[:space:]' < "$STATE_DIR/run-id")
  [[ -n "$RUN_ID" ]] || die "empty run id"
}

load_connection() {
  [[ -f "$STATE_DIR/connection.env" ]] || die "missing $STATE_DIR/connection.env; run scripts/launch.sh first"
  # connection.env has no token. It sets ARGOCD_SERVER and the TLS flags.
  # shellcheck disable=SC1091
  set -a
  source "$STATE_DIR/connection.env"
  set +a
  [[ -n "${ARGOCD_SERVER:-}" ]] || die "ARGOCD_SERVER missing from connection.env"
}

binary_path() {
  [[ -f "$STATE_DIR/built-binary" ]] || die "missing $STATE_DIR/built-binary; run scripts/launch.sh first"
  tr -d '[:space:]' < "$STATE_DIR/built-binary"
}
