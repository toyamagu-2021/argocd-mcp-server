# ArgoCD MCP Server

[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/toyamagu-2021/argocd-mcp-server/badge)](https://scorecard.dev/viewer/?uri=github.com/toyamagu-2021/argocd-mcp-server)

A Model Context Protocol (MCP) server that provides ArgoCD functionality as tools for LLMs via gRPC API.

## Features

### Application Management
- `list_application` - List applications. Default is a TSV summary, at most 50 rows (`limit` defaults to 50). `detailed=true` is a compact page of full Applications. Empty JSON is a page, not a sentence
- `get_application` - Compact summary of an application (`detailed=false` by default). `detailed=true` returns the full Application without managedFields or last-applied-configuration
- `get_application_manifests` - Get rendered Kubernetes manifests for an application
- `get_application_events` - Page of event summaries (`detailed=false` by default, `limit` 50). `metadata` is omitted. `detailed=true` returns full events without `managedFields`
- `get_application_logs` - Plain-text pod logs (`detailed=false` by default). `tail_lines` defaults to 20. `detailed=true` returns compact JSON. `follow=true` is rejected
- `get_application_resource_tree` - Page of node summaries (`detailed=false` by default, `limit` 50). Hosts are names. `detailed=true` includes `systemInfo` and can be very large
- `get_application_managed_resources` - List managed resources as compact JSON. Default is a summary (no manifest strings), `limit` 50. `detailed=true` or a resource filter includes state, without `managedFields` or last-applied-configuration
- `get_application_resource` - Get the live manifest of one managed resource (ApplicationService.GetResource). Argo CD replaces Secret data
- `create_application` - Create a new ArgoCD application with source and destination configuration
- `update_application` - Replace an application's spec (`ApplicationService.UpdateSpec`). The object replaces the whole spec; `validate` defaults to true
- `sync_application` - Trigger a sync operation for an application, with optional prune, dry-run, a target revision, a per-source revision list, a partial resource list, and per-sync sync options
- `refresh_application` - Refresh application state from the git repository
- `delete_application` - Delete an ArgoCD application. **`cascade` defaults to `false`** (resources are kept) — unlike the Argo CD CLI
- `terminate_operation` - Terminate the currently running operation (sync, refresh, etc.) on an application
- `rollback_application` - Roll an application back to a `status.history` id. `prune` and `dry_run` default to false

### ApplicationSet Management
- `list_applicationset` - Page of ApplicationSet summaries (`detailed=false` by default, `limit` 50). Generators and Helm values are detailed-only
- `get_applicationset` - Retrieve detailed information about a specific ApplicationSet
- `create_applicationset` - Create a new ApplicationSet with generators and templates
- `delete_applicationset` - Delete an ApplicationSet (Argo CD also deletes the Applications it generated)

### Project Management
- `list_project` - Page of project summaries (`detailed=false` by default, `limit` 50). `name_only` returns name strings. Roles and tokens are detailed-only
- `get_project` - Retrieve detailed project information by name
- `create_project` - Create new ArgoCD project with access controls and deployment restrictions

### Cluster Management
- `list_cluster` - Page of cluster summaries (`detailed=false` by default, `limit` 50). `name_only` returns `{name, server}` items
- `get_cluster` - Retrieve detailed cluster information including configuration and status

### Repository Management
- `list_repository` - Page of repository summaries (`detailed=false` by default, `limit` 50). Credentials stay off the summary. `detailed=true` returns sanitized repositories
- `get_repository` - Get details of a specific repository including connection status

## Prerequisites

- Go 1.27+ (to build from source)
- Argo CD **3.6.x** (the only supported version)
- Access to an ArgoCD server with API token

## Installation

```bash
go build -o argocd-mcp-server ./cmd/argocd-mcp-server
```

## Configuration

Set the following environment variables:

```bash
export ARGOCD_AUTH_TOKEN=$(argocd account generate-token)
export ARGOCD_SERVER=your-argocd-server.com:443  # Include port number

# Optional settings
export ARGOCD_INSECURE=false     # Skip TLS verification (default: false)
export ARGOCD_PLAINTEXT=false    # Use plaintext connection (default: false)
export ARGOCD_GRPC_WEB=false     # Enable gRPC-Web proxy mode (default: false)
export ARGOCD_GRPC_WEB_ROOT_PATH=""  # Custom root path for gRPC-Web requests (optional)
export ARGOCD_GRPC_MAX_SIZE_MB=200   # Max gRPC message size in MiB (default: 200)
export ARGOCD_MCP_READ_ONLY=false    # When true, state-changing tools are not registered (default: false)

# Logging configuration
export LOG_LEVEL=info     # Options: debug, info, warn, error
export LOG_FORMAT=text    # Options: text, json
```

### Application Namespace

`get_application`, `list_application`, `get_application_manifests`, `sync_application`,
`refresh_application` and `delete_application` (as well as the events, logs, resource tree,
managed resources, get resource and terminate tools) accept an optional `app_namespace`: the namespace of the Application
resource itself (`metadata.namespace`). You only need it when Argo CD is configured for
[Applications in any namespace](https://argo-cd.readthedocs.io/en/stable/operator-manual/app-any-namespace/).

In `list_application`, `namespace` filters by **destination** namespace
(`spec.destination.namespace`) while `app_namespace` filters by the Application's own
namespace. Both can be combined.

### Read-only Mode

With `ARGOCD_MCP_READ_ONLY=true` the server does not register any tool that changes Argo CD state:
`create_application`, `update_application`, `sync_application`, `delete_application`, `terminate_operation`,
`rollback_application`, `create_project`, `create_applicationset`, `delete_applicationset`.

Read-only mode is a convenience on top of Argo CD RBAC, not a replacement for it: issue the
server a token whose account can only do what you want the LLM to do.

All read tools carry the MCP `readOnlyHint` annotation. `refresh_application` counts as a
read tool and stays available in read-only mode: it is an application Get with a refresh
request, which makes Argo CD re-reconcile the Application but never changes cluster resources.

## Usage

Run the server:

```bash
./argocd-mcp-server
```

The server communicates via stdin/stdout using the MCP protocol.

### Testing

List available tools:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./argocd-mcp-server
```

### Tool Examples

#### List Applications
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "list_application",
    "arguments": {
      "project": "default"
    }
  }
}
```

#### Get Application Details
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "get_application",
    "arguments": {
      "name": "my-app"
    }
  }
}
```

#### Get Application Manifests
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "get_application_manifests",
    "arguments": {
      "name": "my-app",
      "revision": "main"
    }
  }
}
```

#### Get Application Events
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "get_application_events",
    "arguments": {
      "name": "my-app",
      "resource_namespace": "default",
      "resource_name": "my-deployment"
    }
  }
}
```

#### Get Application Logs
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "get_application_logs",
    "arguments": {
      "name": "my-app",
      "pod_name": "my-app-deployment-abc123",
      "container": "main",
      "tail_lines": 20,
      "since_seconds": 3600
    }
  }
}
```

#### Get Application Resource Tree
```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "get_application_resource_tree",
    "arguments": {
      "name": "my-app"
    }
  }
}
```

#### Get Managed Resources
```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "get_application_managed_resources",
    "arguments": {
      "name": "my-app",
      "detailed": false
    }
  }
}
```

#### Get One Resource
```json
{
  "jsonrpc": "2.0",
  "id": 8,
  "method": "tools/call",
  "params": {
    "name": "get_application_resource",
    "arguments": {
      "name": "my-app",
      "resource_name": "guestbook-ui",
      "resource_namespace": "default",
      "group": "apps",
      "kind": "Deployment",
      "version": "v1"
    }
  }
}
```

#### Create Application
```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "method": "tools/call",
  "params": {
    "name": "create_application",
    "arguments": {
      "name": "my-app",
      "repo_url": "https://github.com/myorg/myrepo.git",
      "path": "manifests",
      "dest_namespace": "default",
      "dest_server": "https://kubernetes.default.svc",
      "project": "default",
      "target_revision": "main",
      "auto_sync": true,
      "self_heal": true
    }
  }
}
```

#### Sync Application
```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "tools/call",
  "params": {
    "name": "sync_application",
    "arguments": {
      "name": "my-app",
      "prune": true,
      "dry_run": false,
      "revision": "abc123",
      "resources": [
        {"group": "apps", "kind": "Deployment", "name": "web", "namespace": "default"}
      ],
      "sync_options": ["CreateNamespace=true"]
    }
  }
}
```

#### Refresh Application
```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "tools/call",
  "params": {
    "name": "refresh_application",
    "arguments": {
      "name": "my-app",
      "hard": false
    }
  }
}
```

#### Delete Application
```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "method": "tools/call",
  "params": {
    "name": "delete_application",
    "arguments": {
      "name": "my-app",
      "cascade": true
    }
  }
}
```

#### Terminate Operation
```json
{
  "jsonrpc": "2.0",
  "id": 13,
  "method": "tools/call",
  "params": {
    "name": "terminate_operation",
    "arguments": {
      "name": "my-app",
      "app_namespace": "argocd",
      "project": "default"
    }
  }
}
```

### ApplicationSet Examples

#### List ApplicationSets
```json
{
  "jsonrpc": "2.0",
  "id": 13,
  "method": "tools/call",
  "params": {
    "name": "list_applicationset",
    "arguments": {
      "project": "default"
    }
  }
}
```

#### Get ApplicationSet
```json
{
  "jsonrpc": "2.0",
  "id": 14,
  "method": "tools/call",
  "params": {
    "name": "get_applicationset",
    "arguments": {
      "name": "my-appset"
    }
  }
}
```

#### Create ApplicationSet
```json
{
  "jsonrpc": "2.0",
  "id": 15,
  "method": "tools/call",
  "params": {
    "name": "create_applicationset",
    "arguments": {
      "name": "my-appset",
      "namespace": "argocd",
      "generators": [
        {
          "list": {
            "elements": [
              {"cluster": "dev", "namespace": "app-dev"},
              {"cluster": "prod", "namespace": "app-prod"}
            ]
          }
        }
      ],
      "template": {
        "metadata": {
          "name": "{{cluster}}-app"
        },
        "spec": {
          "project": "default",
          "source": {
            "repoURL": "https://github.com/myorg/myrepo.git",
            "targetRevision": "main",
            "path": "manifests/{{cluster}}"
          },
          "destination": {
            "server": "https://kubernetes.default.svc",
            "namespace": "{{namespace}}"
          }
        }
      }
    }
  }
}
```

#### Delete ApplicationSet
```json
{
  "jsonrpc": "2.0",
  "id": 16,
  "method": "tools/call",
  "params": {
    "name": "delete_applicationset",
    "arguments": {
      "name": "my-appset",
      "app_namespace": "argocd"
    }
  }
}
```

### Project Examples

#### List Projects
```json
{
  "jsonrpc": "2.0",
  "id": 17,
  "method": "tools/call",
  "params": {
    "name": "list_project",
    "arguments": {}
  }
}
```

#### Get Project Details
```json
{
  "jsonrpc": "2.0",
  "id": 18,
  "method": "tools/call",
  "params": {
    "name": "get_project",
    "arguments": {
      "name": "my-project"
    }
  }
}
```

#### Create Project
```json
{
  "jsonrpc": "2.0",
  "id": 19,
  "method": "tools/call",
  "params": {
    "name": "create_project",
    "arguments": {
      "name": "my-project",
      "description": "My ArgoCD project",
      "source_repos": "https://github.com/myorg/*",
      "destination_namespace": "*",
      "destination_server": "https://kubernetes.default.svc",
      "namespace_resource_whitelist": "apps:Deployment,:Service,networking.k8s.io:Ingress",
      "cluster_resource_whitelist": "",
      "upsert": false
    }
  }
}
```

### Cluster Examples

#### List Clusters
```json
{
  "jsonrpc": "2.0",
  "id": 20,
  "method": "tools/call",
  "params": {
    "name": "list_cluster",
    "arguments": {}
  }
}
```

#### Get Cluster Details
```json
{
  "jsonrpc": "2.0",
  "id": 21,
  "method": "tools/call",
  "params": {
    "name": "get_cluster",
    "arguments": {
      "id_or_name": "https://kubernetes.default.svc"
    }
  }
}
```

### Repository Examples

#### List Repositories
```json
{
  "jsonrpc": "2.0",
  "id": 22,
  "method": "tools/call",
  "params": {
    "name": "list_repository",
    "arguments": {}
  }
}
```

#### Get Repository Details
```json
{
  "jsonrpc": "2.0",
  "id": 23,
  "method": "tools/call",
  "params": {
    "name": "get_repository",
    "arguments": {
      "repo_url": "https://github.com/myorg/myrepo.git"
    }
  }
}
```

## Architecture

- `cmd/argocd-mcp-server/` - Main entry point; wiring only (config → one client → tools)
- `internal/config/` - Loads and validates configuration from environment variables
- `internal/argocd/client/` - ArgoCD gRPC client (one per process), its `Interface` and response sanitizing
- `internal/grpcwebproxy/` - gRPC-Web proxy for environments without direct gRPC access
- `internal/server/` - MCP server construction
- `internal/tools/` - MCP tool definitions and handlers (`Register(s, Deps)`)
- `internal/logging/` - Structured logging configuration

## Implementation Details

### gRPC Communication

This server communicates directly with the ArgoCD gRPC API. This provides:
- High performance binary protocol
- Full feature parity with ArgoCD CLI
- Structured error handling with gRPC status codes
- Type-safe request/response handling via protocol buffers
- Support for streaming operations
- No dependency on ArgoCD CLI installation

### gRPC-Web Support

For environments without direct gRPC access, the server supports gRPC-Web proxy mode:
- Automatically enabled when `ARGOCD_GRPC_WEB=true`
- Creates a local Unix socket proxy server
- Translates gRPC calls to HTTP/gRPC-Web requests
- Handles proper framing and streaming responses
- Supports custom root paths via `ARGOCD_GRPC_WEB_ROOT_PATH`

### Authentication

The server uses JWT token authentication with the ArgoCD gRPC API:
- Token is passed via `ARGOCD_AUTH_TOKEN` environment variable
- Server address with port is configured via `ARGOCD_SERVER` environment variable
- JWT token is sent as gRPC metadata with each request
- Supports both TLS and plaintext connections

### Secrets in Responses

- **Clusters and repositories**: the Argo CD API server already removes credentials from these responses. As defence in depth, this server additionally passes every Cluster and Repository through the upstream `Sanitized()` methods before returning it, so bearer tokens, passwords, TLS keys, SSH keys and exec-provider args/env are never included.
- **Applications and ApplicationSets**: returned as-is, at the same level of detail as the Argo CD UI and CLI. Helm `values` / `parameters` are **not** redacted. Do not put secrets in Application specs; use a secret manager or Kubernetes Secrets instead.
- **Manifests**: `Secret` data in `get_application_manifests` is already hidden by the Argo CD server.

### Logging

Structured logging is implemented using logrus:
- Log level configurable via `LOG_LEVEL` environment variable (debug, info, warn, error)
- Log format configurable via `LOG_FORMAT` environment variable (text or json)
- Logs are written to stderr to avoid interfering with MCP protocol on stdout

## Development

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-cover

# Run tests with race detector
make test-race

# Run E2E tests with Kind cluster
make e2e-setup  # Setup Kind cluster with ArgoCD
make e2e-test   # Run E2E tests
make e2e-teardown  # Cleanup
```

### Linting and Formatting

```bash
# Format code
make fmt

# Run all linters
make lint

# Run advanced linting (includes security checks)
make lint-advanced
```

## Safety Considerations

- **update_application**: Sends `ApplicationService.UpdateSpec`. Argo CD keeps the current metadata and status and replaces `spec` with the object you send, so omitted spec fields are cleared. Call `get_application` with `detailed=true` and send the full spec. `validate` defaults to `true`
- **rollback_application**: `id` is `status.history[].id` (zero is valid), not a git SHA. `prune` defaults to `false`. Argo CD rejects the call when auto-sync is enabled. `dry_run: true` previews it
- **sync_application**: Use `dry_run: true` to preview changes before actual sync. `revision` syncs a single-source app to that git revision, tag, or chart version (omit it to use the spec target). `revisions` is the multi-source list, one entry per source in `spec.sources` order; an empty entry keeps that source's target. `resources` limits the sync to those objects. `sync_options` replaces the Application's sync options for this operation (it does not merge). Syncing to a revision other than the spec target fails when auto-sync is enabled, unless `dry_run` is true
- **refresh_application**: Use `hard: false` (default) for a normal refresh, `hard: true` to also invalidate the manifest cache
- **delete_application**: `cascade` defaults to `false`, so only the Application is deleted and its resources stay in the cluster. Pass `cascade: true` to delete the resources as well. This intentionally differs from the Argo CD CLI default (`true`)
- **delete_applicationset**: Deleting an ApplicationSet also deletes the Applications it generated; there is no cascade option. The namespace argument is `app_namespace` (the old `appsetNamespace` is still accepted during 0.3)
- **create_project**: Use `upsert: true` to update existing projects instead of failing on conflict
- Always verify application and project names before performing destructive operations

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes with tests
4. Run `make check-all` to ensure quality
5. Submit a pull request

## License

MIT License - see LICENSE file for details