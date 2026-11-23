# Argo CD MCP Server — Public Tools

The server exposes **25 public tools** (design D1, revised 2026-11-23, see
[`design/refactor-0.3.md`](design/refactor-0.3.md)). The list is enforced by
`publicTools` in `internal/tools/register_test.go`.

**Mutating** tools change Argo CD state and are not registered when
`ARGOCD_MCP_READ_ONLY=true`.

## Applications

| Tool | Mutating | Notes |
|---|---|---|
| `list_application` | | `project`, `selector`, `app_namespace` filtered server-side; destination `cluster` / `namespace` filtered client-side |
| `get_application` | | `app_namespace` |
| `get_application_manifests` | | Secret data hidden by the Argo CD server |
| `get_application_events` | | |
| `get_application_logs` | | `follow=true` is rejected (stdio cannot stream) |
| `get_application_resource_tree` | | |
| `get_application_managed_resources` | | `ApplicationService.ManagedResources`. Default is a summary (group, kind, namespace, name, resourceVersion, hook, modified) with no manifest strings. `detailed=true` or any resource filter includes `targetState`, `liveState`, `predictedLiveState`, `normalizedLiveState`. Secret data is hidden by the Argo CD controller before it is cached |
| `get_application_resource` | | `ApplicationService.GetResource`. Live manifest; Argo CD replaces Secret data |
| `create_application` | ✔ | |
| `sync_application` | ✔ | `prune`, `dry_run`, `revision` (single-source), `revisions` (multi-source, one per `spec.sources` entry), `resources` (partial sync), `sync_options` (replace, not merge) |
| `refresh_application` | | treated as read (D11, revised); `hard` |
| `delete_application` | ✔ | `cascade` defaults to **false** (differs from the Argo CD CLI) |
| `terminate_operation` | ✔ | |

## ApplicationSets

| Tool | Mutating | Notes |
|---|---|---|
| `list_applicationset` | | |
| `get_applicationset` | | |
| `create_applicationset` | ✔ | `dry_run`, `upsert` |
| `delete_applicationset` | ✔ | `app_namespace` (legacy `appsetNamespace` accepted in 0.3); generated Applications are deleted too |

## Projects

| Tool | Mutating | Notes |
|---|---|---|
| `list_project` | | |
| `get_project` | | |
| `create_project` | ✔ | |

## Clusters

| Tool | Mutating | Notes |
|---|---|---|
| `list_cluster` | | credentials removed with upstream `Sanitized()` |
| `get_cluster` | | credentials removed with upstream `Sanitized()` |

## Repositories

| Tool | Mutating | Notes |
|---|---|---|
| `list_repository` | | credentials removed with upstream `Sanitized()` |
| `get_repository` | | credentials removed with upstream `Sanitized()` |

## Session

| Tool | Mutating | Notes |
|---|---|---|
| `get_user_info` | | |

## Not planned

The pre-0.3 roadmap in this file (update/patch/rollback of applications, project /
repository / cluster / certificate / GPG / settings / account management, and so on) is
**archived** for write tools. On 2026-11-23, D1 was revised to add two read tools
(`get_application_managed_resources`, `get_application_resource`) because the owner
approved expanding read coverage after a gap analysis. New write tools are still out of
scope. Access is controlled with Argo CD RBAC plus read-only mode, not a tool allowlist
(D10). Changing this scope needs a design update first.
