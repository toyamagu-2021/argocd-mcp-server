# ArgoCD MCP Server

A Model Context Protocol (MCP) server that provides ArgoCD functionality as tools for LLMs via gRPC API.

## Features

### Application Management
- `list_application` - List ArgoCD applications with optional filtering by project, cluster, namespace, and label selectors
- `get_application` - Retrieve detailed information about a specific ArgoCD application
- `get_application_manifests` - Get rendered Kubernetes manifests for an application
- `get_application_events` - Get Kubernetes events for resources belonging to an application
- `get_application_logs` - Retrieve logs from pods in an ArgoCD application
- `get_application_resource_tree` - Get the resource tree structure of an application showing all managed resources
- `create_application` - Create a new ArgoCD application with source and destination configuration
- `sync_application` - Trigger a sync operation for an application with optional prune and dry-run modes
- `refresh_application` - Refresh application state from the git repository
- `delete_application` - Delete an ArgoCD application with optional cascade control
- `terminate_operation` - Terminate the currently running operation (sync, refresh, etc.) on an application

### ApplicationSet Management
- `list_applicationset` - List ArgoCD ApplicationSets with optional filtering
- `get_applicationset` - Retrieve detailed information about a specific ApplicationSet
- `create_applicationset` - Create a new ApplicationSet with generators and templates
- `delete_applicationset` - Delete an ApplicationSet with cascade control

### Project Management
- `list_project` - List all ArgoCD projects
- `get_project` - Retrieve detailed project information by name
- `create_project` - Create new ArgoCD project with access controls and deployment restrictions

### Cluster Management
- `list_cluster` - List all registered clusters in ArgoCD
- `get_cluster` - Retrieve detailed cluster information including configuration and status

### Repository Management
- `list_repository` - List all configured Git repositories
- `get_repository` - Get details of a specific repository including connection status

## Prerequisites

- Go 1.21+
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

# Logging configuration
export LOG_LEVEL=info     # Options: debug, info, warn, error
export LOG_FORMAT=text    # Options: text, json
```

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
      "tail_lines": 100,
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
      "dry_run": false
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
      "hard_refresh": false
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
      "cascade": true
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

- `internal/argocd/client/` - ArgoCD gRPC client implementation
- `internal/argocd/` - ArgoCD data models and types
- `internal/api/` - Legacy REST API client (deprecated)
- `internal/server/` - MCP server core logic  
- `internal/tools/` - MCP tool definitions and handlers
- `internal/logging/` - Structured logging configuration
- `internal/errors/` - Custom error types and handling
- `internal/grpcwebproxy/` - gRPC-Web proxy for environments without direct gRPC access
- `cmd/argocd-mcp-server/` - Main entry point

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

- **sync_application**: Use `dry_run: true` to preview changes before actual sync
- **refresh_application**: Use `hard_refresh: false` for incremental refresh, `true` for full refresh
- **delete_application**: Use `cascade: false` to preserve cluster resources when deleting only the ArgoCD application
- **delete_applicationset**: Use `cascade: false` to preserve generated applications
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