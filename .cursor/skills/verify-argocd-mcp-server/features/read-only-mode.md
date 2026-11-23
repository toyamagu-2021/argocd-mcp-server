# Read-only mode

With `ARGOCD_MCP_READ_ONLY=true`, the server registers the read tools and omits the seven tools that change Argo CD. `get_user_info` and `list_cluster` still return live data. `create_application` is rejected with JSON-RPC `-32602` and does not create an Application. A second session without the flag lists all 23 tools, which shows the flag is what removed the write tools.

## Sub-features

- `full-registration` lists all 23 tools when the flag is unset, including `create_application`.
- `read-only-hides-writes` lists 16 tools and omits the seven mutating tools.
- `read-only-keeps-reads` still calls `get_user_info` and `list_cluster` successfully.
- `read-only-rejects-create` calls `create_application` with a complete payload and observes no Application named `verify-should-not-exist`.

## How to get to it (user POV)

- The MCP client starts the server with `ARGOCD_MCP_READ_ONLY=true` and calls `tools/list`.
- The same client calls `get_user_info`, `list_cluster`, and `create_application` on that process.
- A separate start without the variable calls `tools/list` and sees the write tools again.

## Driving it with verify-argocd-mcp-server

Preconditions:

- `scripts/doctor.sh` prints `DOCTOR_OK`.
- `kubectl get applications.argoproj.io -A` does not list `verify-should-not-exist` before the drive. `evidence.sh` after the drive is the check that matters.

- **Full registration.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh list-tools --feature read-only-mode --label full`. The summary has `read_only` false, `stderr_mentions_read_only` false, `stderr_mentions_registered` true, and `stderr_mentions_waiting` true. `tool_names` has length 23 and includes every name in the two sets below.
- **Read-only session.** Run this session as one process:

```bash
.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh session --feature read-only-mode --label readonly --read-only <<'EOF'
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_user_info","arguments":{}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_cluster","arguments":{"name_only":true}}}
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"create_application","arguments":{"name":"verify-should-not-exist","repo_url":"https://github.com/argoproj/argocd-example-apps.git","path":"guestbook","dest_namespace":"default"}}}
EOF
```

- **Cluster view.** Run `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature read-only-mode`.
- **Assert.** Set `FULL`, `READONLY`, `APPS`, `CLUSTERS`, and `USERINFO` to the `summary.json`, `applications.txt`, `clusters.txt`, and `userinfo.json` paths from `DRIVE_OK` and `EVIDENCE_OK`, then run:

```bash
python3 - "$FULL" "$READONLY" "$APPS" "$CLUSTERS" "$USERINFO" <<'PY'
import json, sys
full, readonly, apps, clusters, userinfo = [json.load(open(p)) if p.endswith(".json") else open(p).read() for p in sys.argv[1:]]
absent = {
    "create_application", "sync_application", "delete_application", "terminate_operation",
    "create_project", "create_applicationset", "delete_applicationset",
}
present = {
    "list_application", "get_application", "get_application_manifests", "get_application_events",
    "get_application_logs", "get_application_resource_tree", "refresh_application",
    "list_project", "get_project", "list_cluster", "get_cluster",
    "list_applicationset", "get_applicationset", "list_repository", "get_repository", "get_user_info",
}
full_names = set(full["tool_names"])
ro_names = set(readonly["tool_names"])
assert full["read_only"] is False and full["stderr_mentions_read_only"] is False
assert full["stderr_mentions_registered"] is True and full["stderr_mentions_waiting"] is True
assert full_names == absent | present and len(full_names) == 23
assert readonly["read_only"] is True and readonly["stderr_mentions_read_only"] is True
assert readonly["stderr_mentions_waiting"] is True
assert ro_names == present and len(ro_names) == 16
calls = {c["tool"]: c for c in readonly["calls"]}
user = calls["get_user_info"]
assert user["is_error"] is False and user["rpc_error"] is None
assert '"loggedIn": true' in user["text"] and '"username": "admin"' in user["text"]
listed = calls["list_cluster"]
assert listed["is_error"] is False and "https://kubernetes.default.svc" in listed["text"]
rejected = calls["create_application"]
assert rejected["rpc_error"]["code"] == -32602
assert "tool 'create_application' not found" in rejected["rpc_error"]["message"]
assert "verify-should-not-exist" not in apps
assert "https://kubernetes.default.svc" in clusters
assert userinfo["loggedIn"] is True and userinfo["username"] == "admin"
print("READONLY_ASSERT_OK")
PY
```

`READONLY_ASSERT_OK` is the end state for this feature. `refresh_application` is in the read-only set. The seven names in `absent` are not.

## Gotchas

- `ARGOCD_MCP_READ_ONLY` enables this mode only when the value is the literal `true`. `1` and `yes` leave all 23 tools registered. Use `--read-only` on `drive.sh`.
- `refresh_application` stays available. It is not one of the seven hidden tools.
- The `create_application` arguments are a complete create payload. A registered tool would add Application `verify-should-not-exist`. If `applications.txt` contains that name, the feature failed. Do not delete the Application before `evidence.sh` records it.
- A tool result with `isError` true and a message about a missing argument means the handler ran. That is not the `-32602` `tool 'create_application' not found: tool not found` rejection this recipe requires.
- The full-registration session must not call `create_application`. Listing the tool is the contrast. Calling it would mutate Argo CD.
- `bash -x` prints the token from the environment. Run the helpers without shell trace.
