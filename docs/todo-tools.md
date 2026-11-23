# Argo CD MCP Server — Public Tools

The server exposes **27 public tools** (design D1, revised 2026-11-23, see
[`design/refactor-0.3.md`](design/refactor-0.3.md)). The list is enforced by
`publicTools` in `internal/tools/register_test.go`.

**Mutating** tools change Argo CD state and are not registered when
`ARGOCD_MCP_READ_ONLY=true`.

## Applications

| Tool | Mutating | Notes |
|---|---|---|
| `list_application` | | `project`, `selector`, `app_namespace` filtered server-side; destination `cluster` / `namespace` filtered client-side. `detailed=false` and `output_format=tsv` are the defaults. `limit` defaults to 50. `name_only` wins over `detailed`. Empty JSON is a page, not prose |
| `get_application` | | `detailed=false` (default) is a compact summary without spec, status.resources, or history. `detailed=true` is the full Application, compact, without managedFields or last-applied-configuration. Per-resource data is `get_application_resource_tree` and `get_application_managed_resources`. `app_namespace` |
| `get_application_manifests` | | Secret data hidden by the Argo CD server |
| `get_application_events` | | `detailed=false` (default) is a page of event summaries without `metadata`. The message is not clipped. `detailed=true` pages full events without `managedFields`. `limit` defaults to 50. Filters `resource_name`, `resource_namespace`, and `resource_uid` still apply before paging |
| `get_application_logs` | | `detailed=false` (default) is plain text. `tail_lines` defaults to 20. `detailed=true` is compact JSON at the same tail. `follow=true` is rejected (stdio cannot stream) |
| `get_application_resource_tree` | | `detailed=false` (default) is a page of node summaries. Hosts are names. `systemInfo`, `networkingInfo`, and `images` are omitted. `detailed=true` pages full nodes and repeats full hosts, including `systemInfo`, on every page. `limit` defaults to 50 |
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
| `list_applicationset` | | `detailed=false` (default) is a page of summaries (`name`, `namespace`, `project`, `generatorCount`). Generators and Helm values are detailed-only. `detailed=true` pages full ApplicationSets without `managedFields`. `limit` defaults to 50. Empty list is an empty page |
| `get_applicationset` | | `detailed=false` (default) is one summary (`name`, `namespace`, `project`, `generatorCount`). Generators and Helm values are detailed-only. `detailed=true` is the full ApplicationSet without `managedFields`. Not a page |
| `create_applicationset` | ✔ | `dry_run`, `upsert` |
| `delete_applicationset` | ✔ | `app_namespace` (legacy `appsetNamespace` accepted in 0.3); generated Applications are deleted too |

## Projects

| Tool | Mutating | Notes |
|---|---|---|
| `list_project` | | `detailed=false` (default) is a page of summaries (`name`, `description`, counts). `name_only` wins and items are name strings. `detailed=true` pages full projects without `managedFields`. `limit` defaults to 50. Empty list is an empty page |
| `get_project` | | `detailed=false` (default) is one summary (`name`, `description`, counts). Roles, tokens, and allow-lists are detailed-only. `detailed=true` is the full project without `managedFields`. Not a page |
| `create_project` | ✔ | |

## Clusters

| Tool | Mutating | Notes |
|---|---|---|
| `list_cluster` | | `detailed=false` (default) is a page of summaries. `name_only` wins and items are `{name, server}`. `detailed=true` pages sanitized clusters without the last-applied annotation. `limit` defaults to 50. Empty list is an empty page |
| `get_cluster` | | credentials removed with upstream `Sanitized()` |

## Repositories

| Tool | Mutating | Notes |
|---|---|---|
| `list_repository` | | `detailed=false` (default) is a page of summaries (`repo`, `type`, `name`, `project`, `connectionStatus`) without credentials. `detailed=true` pages sanitized repositories. `limit` defaults to 50. Empty list is an empty page |
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
