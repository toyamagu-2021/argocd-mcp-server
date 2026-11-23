#!/usr/bin/env bash
# Start one argocd-mcp-server stdio session and speak JSON-RPC to it.
#
#   scripts/drive.sh list-tools --feature NAME [--label NAME] [--read-only]
#   scripts/drive.sh call --feature NAME [--label NAME] [--read-only] TOOL JSON_ARGS
#   scripts/drive.sh session --feature NAME [--label NAME] [--read-only] [--requests FILE]
#
# session reads JSONL from --requests, or from stdin when --requests is omitted.
# The harness sends initialize and notifications/initialized before that JSONL.
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

usage() {
  printf '%s\n' \
    "usage: drive.sh list-tools --feature NAME [--label NAME] [--read-only] [--timeout SECONDS]" \
    "       drive.sh call --feature NAME [--label NAME] [--read-only] [--timeout SECONDS] TOOL JSON_ARGS" \
    "       drive.sh session --feature NAME [--label NAME] [--read-only] [--timeout SECONDS] [--requests FILE]"
}

[[ $# -ge 1 ]] || { usage; exit 2; }
COMMAND=$1
shift

FEATURE=
LABEL=
READ_ONLY=0
REQUESTS_FILE=
TIMEOUT=120

while [[ $# -gt 0 ]]; do
  case "$1" in
    --feature)
      FEATURE=${2:-}
      shift 2
      ;;
    --label)
      LABEL=${2:-}
      shift 2
      ;;
    --read-only)
      READ_ONLY=1
      shift
      ;;
    --requests)
      REQUESTS_FILE=${2:-}
      shift 2
      ;;
    --timeout)
      TIMEOUT=${2:-}
      shift 2
      ;;
    --)
      shift
      break
      ;;
    -*)
      usage
      die "unknown flag $1"
      ;;
    *)
      break
      ;;
  esac
done

[[ -n "$FEATURE" ]] || die "--feature is required"
case "$FEATURE" in
  *[!a-z0-9-]*) die "--feature must be lowercase letters, digits, and hyphens" ;;
esac

load_run_id
load_connection
require_cmd python3

binary=$(binary_path)
[[ -x "$binary" ]] || die "binary is not executable: ${binary}"
[[ -f "$STATE_DIR/token" ]] || die "token file missing; run scripts/launch.sh"

TOKEN=$(python3 -c 'import sys; print(open(sys.argv[1], encoding="utf-8").read().strip())' "$STATE_DIR/token")
[[ -n "$TOKEN" ]] || die "token file is empty"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

case "$COMMAND" in
  list-tools)
    [[ $# -eq 0 ]] || die "list-tools takes no positional arguments"
    [[ -n "$LABEL" ]] || LABEL=list-tools
    printf '%s\n' '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' > "$tmpdir/requests.jsonl"
    ;;
  call)
    [[ $# -eq 2 ]] || die "call requires TOOL and JSON_ARGS"
    tool=$1
    args_json=$2
    case "$tool" in
      *[!a-z_]*) die "tool name must be lowercase letters and underscores" ;;
    esac
    python3 -c 'import json,sys; json.loads(sys.argv[1])' "$args_json" || die "JSON_ARGS is not JSON"
    [[ -n "$LABEL" ]] || LABEL="call-${tool}"
    python3 - "$tool" "$args_json" "$tmpdir/requests.jsonl" <<'PY'
import json, sys
tool, args, dest = sys.argv[1:]
message = {
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {"name": tool, "arguments": json.loads(args)},
}
open(dest, "w", encoding="utf-8").write(json.dumps(message) + "\n")
PY
    ;;
  session)
    [[ $# -eq 0 ]] || die "session takes no positional arguments"
    [[ -n "$LABEL" ]] || LABEL=session
    if [[ -n "$REQUESTS_FILE" ]]; then
      [[ -f "$REQUESTS_FILE" ]] || die "requests file not found: ${REQUESTS_FILE}"
      cp "$REQUESTS_FILE" "$tmpdir/requests.jsonl"
    else
      if [[ -t 0 ]]; then
        die "session needs --requests FILE or a JSONL stdin"
      fi
      cat > "$tmpdir/requests.jsonl"
    fi
    ;;
  *)
    usage
    die "unknown command ${COMMAND}"
    ;;
esac

case "$LABEL" in
  *[!a-z0-9-]*) die "--label must be lowercase letters, digits, and hyphens" ;;
esac

evidence="$EVIDENCE_ROOT/$RUN_ID/$FEATURE/$LABEL"
mkdir -p "$evidence" "$STATE_DIR/pids"
pid_file="$STATE_DIR/pids/mcp-$$"

export ARGOCD_SERVER ARGOCD_INSECURE ARGOCD_PLAINTEXT ARGOCD_GRPC_WEB LOG_LEVEL
export ARGOCD_AUTH_TOKEN="$TOKEN"
unset TOKEN

read_only_value=false
if [[ "$READ_ONLY" -eq 1 ]]; then
  export ARGOCD_MCP_READ_ONLY=true
  read_only_value=true
else
  unset ARGOCD_MCP_READ_ONLY
fi

python3 "$SCRIPT_DIR/mcp_session.py" \
  --binary "$binary" \
  --evidence-dir "$evidence" \
  --requests "$tmpdir/requests.jsonl" \
  --pid-file "$pid_file" \
  --feature "$FEATURE" \
  --label "$LABEL" \
  --timeout "$TIMEOUT" \
  --read-only "$read_only_value"
