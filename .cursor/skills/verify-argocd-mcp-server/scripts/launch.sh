#!/usr/bin/env bash
# Bring up a disposable Kind cluster with Argo CD and build argocd-mcp-server.
# Reuses `make kind-create` and `make install-argocd`. Does not read or write .env.
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

require_cmd kind
require_cmd kubectl
require_cmd docker
require_cmd go
require_cmd python3
require_cmd curl
require_cmd make

docker info >/dev/null 2>&1 || die "docker info failed; the current user cannot talk to the Docker daemon"

mkdir -p "$STATE_DIR"
chmod 700 "$STATE_DIR"

if [[ ! -f "$STATE_DIR/run-id" ]]; then
  printf '%s\n' "$(date -u +%Y%m%dT%H%M%SZ)-$$" > "$STATE_DIR/run-id"
fi
load_run_id
mkdir -p "$EVIDENCE_ROOT/$RUN_ID"
LAUNCH_LOG="$EVIDENCE_ROOT/$RUN_ID/launch.log"
touch "$LAUNCH_LOG"

log() {
  printf '%s\n' "$*" | tee -a "$LAUNCH_LOG"
}

port_in_use() {
  python3 - "$ARGOCD_PORT" <<'PY'
import socket, sys
port = int(sys.argv[1])
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.settimeout(1)
try:
    result = sock.connect_ex(("127.0.0.1", port))
finally:
    sock.close()
sys.exit(0 if result == 0 else 1)
PY
}

cluster_exists() {
  kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"
}

log "launch run=$RUN_ID repo=$REPO_ROOT"

owned=0
if [[ -f "$STATE_DIR/owned-cluster" ]] && [[ "$(tr -d '[:space:]' < "$STATE_DIR/owned-cluster")" == "$CLUSTER_NAME" ]]; then
  owned=1
fi

if cluster_exists; then
  if [[ "$owned" -ne 1 ]]; then
    die "Kind cluster ${CLUSTER_NAME} already exists and this verification state does not own it. Refusing to drive a shared cluster. Run scripts/cleanup.sh only for a cluster this skill created."
  fi
  log "reusing owned Kind cluster ${CLUSTER_NAME}"
else
  if port_in_use; then
    die "127.0.0.1:${ARGOCD_PORT} is already accepting connections. Refusing to create ${CLUSTER_NAME}, which publishes Argo CD on that port."
  fi
  log "creating Kind cluster ${CLUSTER_NAME}"
  make -C "$REPO_ROOT" kind-create
  printf '%s\n' "$CLUSTER_NAME" > "$STATE_DIR/owned-cluster"
  owned=1
fi

kubectl config use-context "$EXPECTED_CONTEXT" >/dev/null
current=$(kubectl config current-context)
[[ "$current" == "$EXPECTED_CONTEXT" ]] || die "kubectl context is ${current}, expected ${EXPECTED_CONTEXT}"

install_argocd_with_retry() {
  local attempt
  for attempt in 1 2 3; do
    log "install-argocd attempt ${attempt}/3"
    if make -C "$REPO_ROOT" install-argocd; then
      return 0
    fi
    log "install-argocd failed. The argocd-redis rollout timeout is a known flake; deleting redis pods and retrying."
    kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" delete pod -l app.kubernetes.io/name=argocd-redis --force --grace-period=0 --ignore-not-found || true
  done
  return 1
}

if curl -fsSk --max-time 5 "$ARGOCD_URL/api/version" >/dev/null 2>&1; then
  log "Argo CD API already responds at ${ARGOCD_URL}; skipping install-argocd"
else
  install_argocd_with_retry || die "make install-argocd failed after 3 attempts"
fi

log "restarting argocd-server so the admin apiKey capability from install-argocd is loaded"
kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" rollout restart deployment/argocd-server
kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" rollout status deployment/argocd-server --timeout=180s

ready=0
for _ in $(seq 1 30); do
  if curl -fsSk --max-time 5 "$ARGOCD_URL/api/version" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 2
done
[[ "$ready" -eq 1 ]] || die "Argo CD API did not respond at ${ARGOCD_URL}/api/version"
curl -fsSk --max-time 10 "$ARGOCD_URL/api/version" > "$EVIDENCE_ROOT/$RUN_ID/argocd-version.json"
log "Argo CD version endpoint saved (see argocd-version.json)"

secret_ready=0
for _ in $(seq 1 30); do
  if kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" get secret argocd-initial-admin-secret >/dev/null 2>&1; then
    secret_ready=1
    break
  fi
  sleep 2
done
[[ "$secret_ready" -eq 1 ]] || die "argocd-initial-admin-secret was not created"

kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d > "$STATE_DIR/admin-password"
chmod 600 "$STATE_DIR/admin-password"

mint_token() {
  python3 - "$STATE_DIR" "$ARGOCD_URL" "$RUN_ID" <<'PY'
import json, os, ssl, sys, time, urllib.error, urllib.request
state, base, run_id = sys.argv[1:]
token_path = os.path.join(state, "token")
ctx = ssl._create_unverified_context()

def request(url, payload=None, bearer=None):
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    headers = {}
    if payload is not None:
        headers["Content-Type"] = "application/json"
    if bearer:
        headers["Authorization"] = "Bearer " + bearer
    method = "POST" if payload is not None else "GET"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, context=ctx, timeout=20) as resp:
            return resp.status, resp.read().decode("utf-8", errors="replace")
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read().decode("utf-8", errors="replace")

def public_error(body):
    try:
        parsed = json.loads(body)
    except json.JSONDecodeError:
        return "non-json response"
    message = parsed.get("message") or parsed.get("error") or "request failed"
    return str(message)[:300]

if os.path.isfile(token_path):
    existing = open(token_path, encoding="utf-8").read().strip()
    if existing:
        status, body = request(base + "/api/v1/session/userinfo", bearer=existing)
        if status == 200:
            try:
                info = json.loads(body)
            except json.JSONDecodeError:
                info = {}
            if info.get("loggedIn") is True and info.get("username"):
                print("TOKEN_REUSED")
                raise SystemExit(0)

password = open(os.path.join(state, "admin-password"), encoding="utf-8").read().strip()
session_token = ""
last = "no attempt"
for _ in range(15):
    status, body = request(base + "/api/v1/session", {"username": "admin", "password": password})
    if status == 200:
        try:
            session_token = json.loads(body).get("token") or ""
        except json.JSONDecodeError:
            session_token = ""
        if session_token:
            break
    last = f"session HTTP {status}: {public_error(body)}"
    time.sleep(3)
if not session_token:
    raise SystemExit(f"TOKEN_ERROR {last}")

name = f"verify-{run_id}-{int(time.time())}"
status, body = request(base + "/api/v1/account/admin/token", {"name": name}, bearer=session_token)
token = ""
if status == 200:
    try:
        token = json.loads(body).get("token") or ""
    except json.JSONDecodeError:
        token = ""
if not token:
    raise SystemExit(f"TOKEN_ERROR account token HTTP {status}: {public_error(body)}")
fd = os.open(token_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
with os.fdopen(fd, "w", encoding="utf-8") as handle:
    handle.write(token)
print("TOKEN_OK")
PY
}

if ! mint_token; then
  log "token mint failed; restarting argocd-server so accounts.admin apiKey is loaded, then retrying once"
  kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" rollout restart deployment/argocd-server
  kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" rollout status deployment/argocd-server --timeout=180s
  for _ in $(seq 1 30); do
    if curl -fsSk --max-time 5 "$ARGOCD_URL/api/version" >/dev/null 2>&1; then
      break
    fi
    sleep 2
  done
  mint_token || die "could not mint an Argo CD API token"
fi

chmod 600 "$STATE_DIR/token"
log "admin API token is in the state directory"

cat > "$STATE_DIR/connection.env" <<EOF
ARGOCD_SERVER=${ARGOCD_HOST}:${ARGOCD_PORT}
ARGOCD_INSECURE=true
ARGOCD_PLAINTEXT=false
ARGOCD_GRPC_WEB=false
LOG_LEVEL=info
EOF
chmod 600 "$STATE_DIR/connection.env"

log "building argocd-mcp-server"
export GOTOOLCHAIN=auto
make -C "$REPO_ROOT" build
printf '%s\n' "$REPO_ROOT/argocd-mcp-server" > "$STATE_DIR/built-binary"
"$REPO_ROOT/argocd-mcp-server" version | tee "$STATE_DIR/binary-version.txt" | tee -a "$LAUNCH_LOG"

log "VERIFY_READY run=${RUN_ID} server=${ARGOCD_HOST}:${ARGOCD_PORT} context=${EXPECTED_CONTEXT} binary=${REPO_ROOT}/argocd-mcp-server evidence=${EVIDENCE_ROOT}/${RUN_ID}"
