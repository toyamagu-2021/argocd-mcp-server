# Changelog

All notable changes to this project are documented here. The GitHub Release for each tag
links to the matching section.

## v0.3.0 (unreleased)

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
