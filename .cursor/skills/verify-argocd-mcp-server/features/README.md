# argocd-mcp-server verification map

This directory is the maintained source for verifying the MCP client behavior of argocd-mcp-server against a real Argo CD. Read this index, then follow one feature file as the recipe.

## Baseline preconditions

- Run from the repository root after `scripts/launch.sh` prints `VERIFY_READY`.
- `scripts/doctor.sh` prints `DOCTOR_OK` for this run id, context `kind-argocd-mcp-server`, and `https://127.0.0.1:8080`.
- The Kind cluster name is `argocd-mcp-server`. Do not drive a cluster this run did not create.
- A fresh install has the `default` AppProject, the in-cluster server `https://kubernetes.default.svc`, the admin user, and no Applications.
- `drive.sh` is the MCP client. It sends `initialize`, `notifications/initialized`, then the feature's requests on one stdio process.
- Do not read or write `.env`. Do not print `.verify-argocd-mcp-server/token`.

## Driving conventions

- Start every recipe from the baseline above unless that feature file adds a precondition.
- One `drive.sh` invocation is one MCP process. A feature that needs the read-only flag and the full registration uses two invocations and two labels.
- Treat the JSON in each feature file as literal, including tool names and argument keys.
- `--read-only` is the switch that sets `ARGOCD_MCP_READ_ONLY=true`.
- After the drive, run `scripts/evidence.sh --feature <id>` and compare the transcript with `cluster-state/`.
- Leave `evidence/<run-id>/` in place when cleaning up.

## Proof and skip reporting

- Capture the JSON-RPC request, the JSON-RPC response, and the kubectl (or userinfo) view of the same fact.
- `summary.json` is the index of a session. The raw exchange is `transcript.jsonl`.
- Record the feature id and the `--label` with every artifact. The evidence path already contains both.
- An unreachable entry point stays unverified. Say which command failed and which precondition was missing.
- Do not report a skipped feature as verified through a different feature's tool call.

## Feature entry contract

Each feature file starts with an H1 and one paragraph of user-visible behavior, then these H2 sections in order:

1. `Sub-features`
2. `How to get to it (user POV)`
3. `Driving it with verify-argocd-mcp-server`
4. `Gotchas`

## Features

- [List applications](list-applications.md) covers `list_application` against `kubectl get applications.argoproj.io`.
- [List clusters](list-clusters.md) covers `list_cluster` and `get_cluster` for the in-cluster server.
- [List projects](list-projects.md) covers `list_project` against AppProjects, including `default`.
- [Session user info](session-user-info.md) covers `get_user_info` for the admin token.
- [Read-only mode](read-only-mode.md) covers `ARGOCD_MCP_READ_ONLY=true`: write tools disappear, read tools still run, and a create call does not add an Application.
