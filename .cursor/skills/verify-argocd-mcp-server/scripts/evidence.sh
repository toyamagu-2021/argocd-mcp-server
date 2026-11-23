#!/usr/bin/env bash
# Capture Argo CD state for one feature. Does not start the MCP server.
# Leaves files next to drive transcripts. Never prints the auth token.
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib.sh"

FEATURE=
while [[ $# -gt 0 ]]; do
  case "$1" in
    --feature)
      FEATURE=${2:-}
      shift 2
      ;;
    *)
      die "unknown argument $1"
      ;;
  esac
done
[[ -n "$FEATURE" ]] || die "--feature is required"
case "$FEATURE" in
  *[!a-z0-9-]*) die "--feature must be lowercase letters, digits, and hyphens" ;;
esac

require_cmd kubectl
require_cmd curl
require_cmd python3
load_run_id
load_connection

dest="$EVIDENCE_ROOT/$RUN_ID/$FEATURE/cluster-state"
mkdir -p "$dest"

kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" get pods -o wide > "$dest/pods.txt" 2>&1
kubectl --context "$EXPECTED_CONTEXT" get applications.argoproj.io -A > "$dest/applications.txt" 2>&1
kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" get appprojects.argoproj.io > "$dest/appprojects.txt" 2>&1
# Spec fields the Application table does not show, plus history ids and the synced revision.
{
  printf '%s\n' $'name\trevisionHistoryLimit\ttargetRevision\trepoURL\tpath\tdestNamespace\thistoryIds\tsyncRevision'
  kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" get applications.argoproj.io \
    -o go-template='{{range .items}}{{.metadata.name}}{{"\t"}}{{with .spec.revisionHistoryLimit}}{{.}}{{end}}{{"\t"}}{{with .spec.source}}{{.targetRevision}}{{end}}{{"\t"}}{{with .spec.source}}{{.repoURL}}{{end}}{{"\t"}}{{with .spec.source}}{{.path}}{{end}}{{"\t"}}{{.spec.destination.namespace}}{{"\t"}}{{range $i, $h := .status.history}}{{if $i}},{{end}}{{$h.id}}{{end}}{{"\t"}}{{with .status.sync}}{{.revision}}{{end}}{{"\n"}}{{end}}'
} > "$dest/application-specs.txt" 2>&1
kubectl --context "$EXPECTED_CONTEXT" -n default get deploy,svc > "$dest/default-workloads.txt" 2>&1
# Cluster secrets, when Argo CD has persisted any. A fresh v3.6 install keeps the
# in-cluster entry in the API without a secret. clusters.txt also lists name and
# server from GET /api/v1/clusters. Config and certificates are not written.
{
  echo "# cluster-secrets"
  kubectl --context "$EXPECTED_CONTEXT" -n "$ARGOCD_NAMESPACE" get secrets \
    -l argocd.argoproj.io/secret-type=cluster \
    -o go-template='{{range .items}}{{.metadata.name}}{{"\t"}}{{if index .data "server"}}{{index .data "server" | base64decode}}{{end}}{{"\n"}}{{end}}'
  echo "# api-clusters"
} > "$dest/clusters.txt"
python3 - "$STATE_DIR/token" "$ARGOCD_URL" "$dest/clusters.txt" <<'PY'
import json, ssl, sys, urllib.request
token = open(sys.argv[1], encoding="utf-8").read().strip()
url = sys.argv[2] + "/api/v1/clusters"
dest = sys.argv[3]
req = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
ctx = ssl._create_unverified_context()
with urllib.request.urlopen(req, context=ctx, timeout=20) as resp:
    data = json.load(resp)
lines = []
for item in data.get("items") or []:
    name = str(item.get("name") or "")
    server = str(item.get("server") or "")
    lines.append(f"{name}\t{server}")
with open(dest, "a", encoding="utf-8") as handle:
    handle.write("\n".join(lines) + ("\n" if lines else ""))
PY

curl -fsSk --max-time 10 "$ARGOCD_URL/api/version" > "$dest/argocd-version.json"

python3 - "$STATE_DIR/token" "$ARGOCD_URL" "$dest/userinfo.json" <<'PY'
import json, ssl, sys, urllib.request
token = open(sys.argv[1], encoding="utf-8").read().strip()
url = sys.argv[2] + "/api/v1/session/userinfo"
dest = sys.argv[3]
req = urllib.request.Request(url, headers={"Authorization": "Bearer " + token})
ctx = ssl._create_unverified_context()
with urllib.request.urlopen(req, context=ctx, timeout=20) as resp:
    data = json.load(resp)
kept = {
    "loggedIn": data.get("loggedIn"),
    "username": data.get("username"),
    "groups": data.get("groups") or [],
}
open(dest, "w", encoding="utf-8").write(json.dumps(kept, indent=2) + "\n")
PY

python3 - "$dest" <<'PY'
import pathlib, re, sys
root = pathlib.Path(sys.argv[1])
jwt = re.compile(r"eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}")
for path in root.iterdir():
    if not path.is_file():
        continue
    text = path.read_text(encoding="utf-8", errors="replace")
    cleaned = jwt.sub("[redacted-jwt]", text)
    if cleaned != text:
        path.write_text(cleaned, encoding="utf-8")
PY

printf 'EVIDENCE_OK feature=%s dir=%s\n' "$FEATURE" "$dest"
