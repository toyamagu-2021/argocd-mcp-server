#!/usr/bin/env bash
# Read-only check: is the Kind Argo CD from launch.sh worth driving?
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd kubectl
require_cmd kind
require_cmd curl
require_cmd python3

load_run_id
load_connection
mkdir -p "$EVIDENCE_ROOT/$RUN_ID"
DOCTOR_OUT="$EVIDENCE_ROOT/$RUN_ID/doctor.txt"
: > "$DOCTOR_OUT"

say() {
  printf '%s\n' "$*" | tee -a "$DOCTOR_OUT"
}

fail() {
  say "DOCTOR_FAIL $*"
  exit 1
}

[[ -f "$STATE_DIR/owned-cluster" ]] || fail "missing owned-cluster marker"
[[ "$(tr -d '[:space:]' < "$STATE_DIR/owned-cluster")" == "$CLUSTER_NAME" ]] || fail "owned-cluster marker is not ${CLUSTER_NAME}"

kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME" || fail "kind cluster ${CLUSTER_NAME} is not present"

context=$(kubectl config current-context 2>/dev/null || true)
[[ "$context" == "$EXPECTED_CONTEXT" ]] || fail "kubectl context is '${context}', expected ${EXPECTED_CONTEXT}"

[[ -f "$STATE_DIR/token" ]] || fail "token file missing"
mode=$(stat -c %a "$STATE_DIR/token")
[[ "$mode" == "600" ]] || fail "token file mode is ${mode}, expected 600"

available=$(kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" get deployment argocd-server -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')
[[ "$available" == "True" ]] || fail "argocd-server deployment is not Available"

node_port=$(kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" get service argocd-server -o jsonpath='{.spec.ports[?(@.name=="https")].nodePort}')
[[ "$node_port" == "30080" ]] || fail "argocd-server https nodePort is '${node_port}', expected 30080"

version_json=$(curl -fsSk --max-time 10 "$ARGOCD_URL/api/version") || fail "GET ${ARGOCD_URL}/api/version failed"
printf '%s\n' "$version_json" > "$EVIDENCE_ROOT/$RUN_ID/argocd-version.json"
version=$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("Version",""))' <<<"$version_json")
[[ -n "$version" ]] || fail "api/version returned no Version field"

user_line=$(python3 - "$STATE_DIR/token" "$ARGOCD_URL" <<'PY'
import json, ssl, sys, urllib.request
token = open(sys.argv[1], encoding="utf-8").read().strip()
url = sys.argv[2] + "/api/v1/session/userinfo"
req = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
ctx = ssl._create_unverified_context()
with urllib.request.urlopen(req, context=ctx, timeout=20) as resp:
    data = json.load(resp)
logged = "true" if data.get("loggedIn") is True else "false"
username = str(data.get("username") or "")
print(f"{logged} {username}")
PY
) || fail "session userinfo request failed"
logged=${user_line%% *}
username=${user_line#* }
[[ "$logged" == "true" && -n "$username" ]] || fail "userinfo loggedIn=${logged} username=${username}"

binary=$(binary_path)
[[ -x "$binary" ]] || fail "binary is not executable: ${binary}"
version_out=$("$binary" version)
printf '%s\n' "$version_out" > "$STATE_DIR/binary-version.txt"
printf '%s\n' "$version_out" | grep -q "argocd-mcp-server version" || fail "binary version output was: ${version_out}"

say "DOCTOR_OK context=${EXPECTED_CONTEXT} argocd=${version} user=${username} binary=${binary} run=${RUN_ID} evidence=${EVIDENCE_ROOT}/${RUN_ID}"
