---
name: verify-argocd-mcp-server
description: "Drive argocd-mcp-server over stdio against a real Argo CD on Kind, the way an MCP client does. Use to prove tools/list and tools/call, including ARGOCD_MCP_READ_ONLY=true hiding write tools, and to capture transcripts plus kubectl state."
---

# Verify argocd-mcp-server

The user of this server is an MCP client. The client starts `argocd-mcp-server`, speaks newline-delimited JSON-RPC on stdin/stdout, and expects Argo CD state to change only when a write tool is registered and called. This skill builds that binary, starts Argo CD in the repo's Kind cluster, drives one stdio session per check, and keeps the transcripts after the cluster is gone.

Run every command from the repository root. The helpers find the root themselves, but the evidence paths below are relative to that root.

Two verification runs cannot share a machine. The Kind cluster name is `argocd-mcp-server`, the kube context is `kind-argocd-mcp-server`, and Argo CD is published on `127.0.0.1:8080` by `kind-config.yaml`. If that cluster or port belongs to something else, launch refuses. Do not point this skill at a live Argo CD.

Do not read or write `.env`. `make e2e-setup` writes `.env` and installs the Argo CD CLI with Homebrew. This skill calls `make kind-create` and `make install-argocd` only, then mints an admin API token over HTTPS into `.verify-argocd-mcp-server/token` (mode `600`).

## Launch

Prerequisites on `PATH`: `docker` (the current user can run `docker info`), `kind`, `kubectl`, `go`, `python3`, `curl`, `make`. `launch.sh` exports `GOTOOLCHAIN=auto` so an older system Go downloads the version in `go.mod` before `make build`.

`127.0.0.1:8080` must be free, and `kind get clusters` must not already list `argocd-mcp-server`, unless `.verify-argocd-mcp-server/owned-cluster` says this state directory created it. A second launch in the same state directory reuses that cluster.

```bash
.cursor/skills/verify-argocd-mcp-server/scripts/launch.sh
```

What it does:

1. Creates the Kind cluster with `make kind-create` and records ownership before install, so a failed install is still cleaned up by this skill.
2. Installs Argo CD v3.6.2 with `make install-argocd`. That target waits up to 300s for each deployment, including `argocd-redis`. If install fails, the script deletes pods labeled `app.kubernetes.io/name=argocd-redis` and retries the whole target, up to 3 attempts. The redis rollout timeout is a known flake.
3. Restarts `argocd-server` so the `accounts.admin: apiKey, login` patch from `install-argocd` is loaded, then waits until `https://127.0.0.1:8080/api/version` returns JSON.
4. Mints an admin API token. The password and token stay in `.verify-argocd-mcp-server/` and are not printed. If the account API rejects the first mint, it restarts `argocd-server` once and retries.
5. Builds `./argocd-mcp-server` with `make build`.

Ready is the line:

```text
VERIFY_READY run=<id> server=127.0.0.1:8080 context=kind-argocd-mcp-server binary=<repo>/argocd-mcp-server evidence=<repo>/.cursor/skills/verify-argocd-mcp-server/evidence/<id>
```

The version endpoint JSON is `<evidence>/<id>/argocd-version.json`. The API responding is not enough on its own: the token mint and the version command of the built binary must both have succeeded.

There is no long-lived MCP process. Each drive starts `argocd-mcp-server` with pipes. A tmux or PTY wrapper would mix terminal bytes into stdout, so the client is `scripts/mcp_session.py`, invoked by `drive.sh`.

## Doctor

Run this before a drive whenever launch output is stale or a call fails before a tool result.

```bash
.cursor/skills/verify-argocd-mcp-server/scripts/doctor.sh
```

Doctor is read-only. It checks the owned Kind cluster, kube context `kind-argocd-mcp-server`, token file mode `600`, `argocd-server` Available, HTTPS nodePort `30080`, `api/version`, session userinfo for a logged-in username, and `<binary> version` containing `argocd-mcp-server version`. It does not start the MCP server.

A usable instance prints one line and writes the same line to `evidence/<run-id>/doctor.txt`:

```text
DOCTOR_OK context=kind-argocd-mcp-server argocd=<version> user=<username> binary=<path> run=<id> evidence=<path>
```

Anything else is `DOCTOR_FAIL`. Do not drive that instance.

## Drive

`drive.sh` loads `.verify-argocd-mcp-server/connection.env`, reads the token file into the environment (not argv), and starts one server process. Stdout is JSON-RPC only. Server logs go to stderr and are saved, with JWTs redacted.

The harness sends this initialize, then `notifications/initialized`, then the requests for the command. Protocol version is `2024-11-05`, which is what `test/argocd_e2e` and the README send. Client name is `verify-argocd-mcp-server`.

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{"roots":{"listChanged":true}},"clientInfo":{"name":"verify-argocd-mcp-server","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
```

Commands, all from the repo root:

```bash
# Every registered tool, write tools included.
.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh list-tools --feature read-only-mode --label full

# Same session shape with ARGOCD_MCP_READ_ONLY=true. The flag is the literal true.
.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh list-tools --feature read-only-mode --label readonly-list --read-only

# One tool call. JSON_ARGS is the tool arguments object.
.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature list-clusters --label name-only list_cluster '{"name_only":true}'

# Several requests in one process, which is how an MCP client holds a session.
.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh session --feature read-only-mode --label readonly --read-only <<'EOF'
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_user_info","arguments":{}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_cluster","arguments":{"name_only":true}}}
EOF
```

`--read-only` sets `ARGOCD_MCP_READ_ONLY=true` for that process. Without the flag, `drive.sh` unsets the variable. A pre-exported `ARGOCD_MCP_READ_ONLY=1` does not enable read-only mode: the server accepts only the literal `true`, and `drive.sh` will not leave a stray value set.

A completed session prints:

```text
DRIVE_OK feature=<feature> label=<label> summary=<evidence>/<run-id>/<feature>/<label>/summary.json
```

`summary.json` lists `tool_names` from `tools/list` and `calls[]` with `tool`, `is_error`, `rpc_error`, and redacted `text`. The directory also has `requests.jsonl`, `responses.jsonl`, `transcript.jsonl`, and `server.stderr.log`. Stderr for a live server contains `Waiting for requests on stdin`. Read-only stderr also contains `Read-only mode: state-changing tools are not registered`.

Follow `features/<feature>.md` for the session that proves that feature. One feature per session label. Do not mark a feature proven because a different feature happened to call the same tool.

## Evidence

Proof is the MCP transcript plus a second view of Argo CD taken after the drive. The second view is kubectl (and, for the logged-in user, `GET /api/v1/session/userinfo` with the fields `loggedIn`, `username`, and `groups` only).

```bash
.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature read-only-mode
```

That writes `evidence/<run-id>/<feature>/cluster-state/`:

- `pods.txt`
- `applications.txt` (stdout and stderr; an empty cluster includes `No resources found`)
- `appprojects.txt`
- `clusters.txt` (cluster secret name and server, then `/api/v1/clusters` name and server; config omitted)
- `argocd-version.json`
- `userinfo.json`

It prints `EVIDENCE_OK feature=<feature> dir=<path>`. JWTs are stripped from those files. Do not copy `.verify-argocd-mcp-server/token` or `admin-password` into the evidence directory.

Capture the action and the resulting state. A `tools/call` text body alone is not proof. Compare it with the matching file from `cluster-state/`. For a write that must not happen, `applications.txt` must not contain the application name from the rejected call.

Evidence stays under `.cursor/skills/verify-argocd-mcp-server/evidence/<run-id>/`. That directory is gitignored. Cleanup never deletes it.

## Cleanup

Cleanup stops MCP processes recorded under `.verify-argocd-mcp-server/pids/` whose command line contains `argocd-mcp-server`, and deletes the Kind cluster only when `owned-cluster` names `argocd-mcp-server`. It removes the state directory and the built `./argocd-mcp-server` binary. It does not delete evidence.

```bash
.cursor/skills/verify-argocd-mcp-server/scripts/cleanup.sh
```

Success includes:

```text
EVIDENCE_KEPT <repo>/.cursor/skills/verify-argocd-mcp-server/evidence
```

Run cleanup after a failed launch or drive too, so a half-created cluster is not left behind. After a proof run, list the evidence directory again and confirm `summary.json` is still there. A cleanup that removes the transcript is a failed proof.

## Helpers

| Script | Invocation |
|---|---|
| Library (source only) | `source .cursor/skills/verify-argocd-mcp-server/scripts/lib.sh` |
| Launch | `.cursor/skills/verify-argocd-mcp-server/scripts/launch.sh` |
| Doctor | `.cursor/skills/verify-argocd-mcp-server/scripts/doctor.sh` |
| Drive | `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh list-tools --feature read-only-mode --label full` |
| Drive one call | `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh call --feature list-clusters --label name-only list_cluster '{"name_only":true}'` |
| Drive a session | `.cursor/skills/verify-argocd-mcp-server/scripts/drive.sh session --feature read-only-mode --label readonly --read-only --requests requests.jsonl` |
| Stdio client used by drive | `python3 .cursor/skills/verify-argocd-mcp-server/scripts/mcp_session.py --help` |
| Evidence | `.cursor/skills/verify-argocd-mcp-server/scripts/evidence.sh --feature read-only-mode` |
| Cleanup | `.cursor/skills/verify-argocd-mcp-server/scripts/cleanup.sh` |

`drive.sh` is the supported client. It calls `mcp_session.py` with `--binary`, `--evidence-dir`, `--requests`, `--pid-file`, `--feature`, `--label`, `--timeout` (default 120), and `--read-only true|false`. Call `mcp_session.py` directly only when debugging the harness, and export `ARGOCD_SERVER` plus `ARGOCD_AUTH_TOKEN` in the environment first. Do not put the token on the command line. Do not run the helpers under `bash -x`.

## Maintenance

When tool registration, Kind install, or the stdio handshake changes, update this skill and `features/` with `/maintain-verification-skill`. The feature files are the source of what a later run must drive. A proof that exercises one file does not cover the others.
