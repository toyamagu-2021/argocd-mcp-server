# Argo CD MCP Server — Public Tools

The server exposes **27 public tools** (design D1, revised 2026-11-23, see
[`design/refactor-0.3.md`](design/refactor-0.3.md)). The list is enforced by
`publicTools` in `internal/tools/register_test.go`.

**Mutating** tools change Argo CD state and are not registered when
`ARGOCD_MCP_READ_ONLY=true`.

## Applications

| Tool | Mutating | Notes |
|---|---|---|
| `list_application` | | `project`, `selector`, `app_namespace` filtered server-side; destination `cluster` / `namespace` filtered client-side |
| `get_application` | | `detailed=false` (default) is a compact summary without spec, status.resources, or history. `detailed=true` is the full Application, compact, without managedFields or last-applied-configuration. Per-resource data is `get_application_resource_tree` and `get_application_managed_resources`. `app_namespace` |
| `get_application_manifests` | | Secret data hidden by the Argo CD server |
| `get_application_events` | | |
| `get_application_logs` | | `follow=true` is rejected (stdio cannot stream) |
| `get_application_resource_tree` | | |
| `get_application_managed_resources` | | Compact JSON. Default summary omits manifest strings. `detailed=true` or a resource filter includes the four state strings with `managedFields` and `last-applied-configuration` removed. `limit` defaults to 50, `offset` defaults to 0, response has `totalItems` and `hasMore` |
| `get_application_resource` | | `ApplicationService.GetResource`. Live manifest; Argo CD replaces Secret data |
| `create_application` | ✔ | |
| `update_application` | ✔ | `ApplicationService.UpdateSpec`. Replaces the whole spec. `validate` defaults to true |
| `sync_application` | ✔ | `prune`, `dry_run`, `revision` (single-source), `revisions` (multi-source, one per `spec.sources` entry), `resources` (partial sync), `sync_options` (replace, not merge) |
| `refresh_application` | | treated as read (D11, revised); `hard` |
| `delete_application` | ✔ | `cascade` defaults to **false** (differs from the Argo CD CLI) |
| `terminate_operation` | ✔ | |
| `rollback_application` | ✔ | `ApplicationService.Rollback`. `id` is `status.history[].id`. `prune` and `dry_run` default to false |

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
**archived** except for the 2026-11-23 D1 revision. That revision added two read tools
(`get_application_managed_resources`, `get_application_resource`; owner approved expanding
read coverage after a gap analysis) and two write tools (`update_application`,
`rollback_application`; owner approved these two write tools). Other write tools stay out
of scope. Access is controlled with Argo CD RBAC plus read-only mode, not a tool allowlist
(D10). Changing this scope needs a design update first.
