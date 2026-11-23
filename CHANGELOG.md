# Changelog

All notable changes to this project are documented here. The GitHub Release for each tag
links to the matching section.

## v0.5.0

Token-efficient tool results ([design](docs/design/token-efficiency.md), #117).
Not tagged yet.

### ⚠️ Breaking changes

1. **`get_application` returns a compact summary by default.**
   A call that omits `detailed` no longer returns the indented Application.
   `spec` and `status.resources` are absent. Pass `detailed=true` for the full
   Application. That body is compact JSON and does not include
   `metadata.managedFields` or `kubectl.kubernetes.io/last-applied-configuration`.
   Helm values are still included when `detailed=true`. (#117)
2. **`get_application_resource_tree` returns a page of node summaries by default.**
   The indented tree becomes one page. Orphaned nodes are items with `orphaned`
   true. `systemInfo`, `networkingInfo`, and `images` are absent unless
   `detailed=true`. Summary hosts are names. `limit` defaults to 50. (#117)
3. **`get_application_events` returns a page of event summaries by default.**
   The indented `EventList` becomes one page. `metadata` is absent unless
   `detailed=true`, and that path still drops `managedFields`. The message is
   not clipped. `limit` defaults to 50. (#117)
4. **`get_application_logs` returns plain text and tails 20 lines by default.**
   The JSON log object becomes tab-separated text. Pass `detailed=true` for
   compact JSON. `tail_lines` defaults to 20, not 100. `follow=true` is still
   an error. (#117)
5. **`list_application` returns at most 50 rows unless `limit` is set.**
   A short TSV page ends with a `#` notice line. JSON responses are a page
   object. `name_only` JSON no longer uses `{names, count}`. `detailed` is a
   page of Applications, not a raw array. The empty prose string is gone. (#117)
6. **`list_cluster` and `list_repository` return pages.**
   Cluster summaries and `{name, server}` rows sit in `items`. The `{clusters, count}`
   wrapper is gone. Repository responses are summaries unless `detailed=true`.
   `limit` defaults to 50. Empty lists are empty pages, not prose. (#117)
7. **`list_project` and `list_applicationset` return summary pages.**
   Project summaries omit roles, tokens, and allow-lists. ApplicationSet summaries
   omit generators and Helm values. `name_only` project JSON is a page of names.
   `detailed=true` returns the full objects without `managedFields`. `limit`
   defaults to 50. Empty lists are empty pages, not prose. (#117)
8. **`get_project` and `get_applicationset` return a compact summary by default.**
   A call that omits `detailed` no longer returns the indented object. Project
   summaries omit roles, tokens, and allow-lists. ApplicationSet summaries omit
   generators and Helm values. Pass `detailed=true` for the full object. That
   body is compact JSON and does not include `metadata.managedFields` or
   `kubectl.kubernetes.io/last-applied-configuration`. The result is one object,
   not a page. (#117)
9. **`get_application_manifests` returns a page of stripped manifests.**
   The indented response becomes one page. `limit` defaults to 10. Each
   manifest loses `managedFields` and the last-applied annotation. YAML
   manifests stay YAML and may be re-quoted. `commands` and integrity fields
   appear only when `detailed=true`. `revision` is still the git revision. (#117)
10. **`get_application_resource` returns compact JSON.**
    The live manifest loses `managedFields` and the last-applied annotation.
    A nil manifest is still omitted. (#117)
11. **Application write results are compact JSON.**
    `create_application`, `sync_application`, `refresh_application`, and
    `rollback_application` drop `managedFields` and the last-applied annotation.
    Call `get_application` when only the summary is needed.
    `update_application` still returns the full spec, including Helm values. (#117)
12. **`create_project` and `create_applicationset` return compact JSON.**
    `managedFields` and the last-applied annotation are removed. Helm values in
    an ApplicationSet template stay. (#117)
13. **`get_cluster`, `get_repository`, and `get_user_info` return compact JSON.**
    Cluster annotations lose the last-applied key. Repository fields and the
    user record are otherwise unchanged. Credentials stay removed by the client
    sanitizer. (#117)

## v0.3.0 (2026-11-23)

The 0.3 zero-based refactor ([design](docs/design/refactor-0.3.md)). Supported Argo CD
version: **3.6.x only**.

### ⚠️ Breaking changes

1. **`delete_application`: `cascade` now defaults to `false`.**
   Calling the tool without `cascade` deletes only the Application and leaves its Kubernetes
   resources in the cluster. Previously it deleted them too. This intentionally differs from
   the Argo CD CLI, whose default is `true`. (#100)
2. **`get_application_logs`: `follow=true` returns an error.**
   A streaming log request could never finish over the stdio transport and hung the call.
   It is now rejected immediately. (#101)
3. **`delete_applicationset`: the namespace argument is `app_namespace`.**
   The old `appsetNamespace` is still accepted during 0.3. Passing both with different values
   is an error. (#99)

### Migration notes for MCP clients and prompts

- **Anything that relied on `delete_application` removing cluster resources by default
  must now pass `cascade: true` explicitly.** Prompts or agents that expect
  "delete = remove everything" should be updated. Otherwise the resources stay in the
  cluster with no owning Application.
- Replace `follow: true` in `get_application_logs` calls with `tail_lines` and/or
  `since_seconds`, and poll if you need fresh lines.
- Rename `appsetNamespace` to `app_namespace` in `delete_applicationset` calls.
  `appsetNamespace` may be removed in a later release.
- `refresh_application` takes `hard` (boolean). Earlier README examples said
  `hard_refresh`, which was always ignored.

### Added

- **Read-only mode:** `ARGOCD_MCP_READ_ONLY=true` does not register state-changing tools
  (`create_application`, `sync_application`, `delete_application`, `terminate_operation`,
  `create_project`, `create_applicationset`, `delete_applicationset`). (#98)
- **MCP tool hints:** read tools are annotated `readOnlyHint=true`. `refresh_application`
  is a non-destructive, idempotent read and stays available in read-only mode. (#98, #102)
- **`app_namespace`** (the Application's own `metadata.namespace`, for apps-in-any-namespace)
  on `get_application`, `list_application`, `get_application_manifests`,
  `sync_application`, `refresh_application` and `delete_application`. In
  `list_application`, `namespace` still means the destination namespace. (#99)
- **`ARGOCD_GRPC_MAX_SIZE_MB`** is documented (default 200).

### Changed

- **One Argo CD client per process.** The client is created at startup instead of per tool
  call. With gRPC-Web this also means a single local proxy. Invalid client configuration
  (for example unreadable client certificates) now fails at startup instead of on every
  tool call. (#95)
- **`list_application`** sends the `project` filter to the server
  (`ApplicationQuery.Projects`). (#99)
- **Cluster and Repository responses** are additionally passed through the upstream
  `Sanitized()` (defence in depth; the Argo CD server already strips credentials).
  Application / ApplicationSet specs, including Helm values, are returned as-is, like the
  Argo CD UI. (#97)
- CI runs the end-to-end suite against Argo CD 3.6.2 over both gRPC and gRPC-Web. (#101)

### Removed

- Unused client methods (Update/Rollback application, Cluster/Project/Repository CUD that
  no tool used), the unused `internal/errors` package, and dead client errors and config
  fields. No public tool was removed. (#96, #101)
