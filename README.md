# ArgoCD MCP Server

A Model Context Protocol (MCP) server that provides ArgoCD functionality as tools for LLMs via gRPC API.

## Features

- `list_application` - List ArgoCD applications with optional filtering by project, cluster, namespace, and label selectors
- `get_application` - Retrieve detailed information about a specific ArgoCD application
- `sync_application` - Trigger a sync operation for an application with optional prune and dry-run modes
- `delete_application` - Delete an ArgoCD application with optional cascade control

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
export ARGOCD_INSECURE=true     # Skip TLS verification (for self-signed certs)
export ARGOCD_PLAINTEXT=true    # Use plaintext connection (for non-TLS servers)
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

#### Sync Application
```json
{
  "jsonrpc": "2.0",
  "id": 5,
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

#### Delete Application
```json
{
  "jsonrpc": "2.0",
  "id": 6,
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

## Architecture

- `internal/argocd/client/` - ArgoCD gRPC client implementation
- `internal/argocd/` - ArgoCD data models and types
- `internal/api/` - Legacy REST API client (deprecated)
- `internal/server/` - MCP server core logic  
- `internal/tools/` - MCP tool definitions and handlers
- `internal/logging/` - Structured logging configuration
- `internal/errors/` - Custom error types and handling
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

## Safety Considerations

- **sync_application**: Use `dry_run: true` to preview changes before actual sync
- **delete_application**: Use `cascade: false` to preserve cluster resources when deleting only the ArgoCD application
- Always verify application names before performing destructive operations

## Future Enhancements

- Add `create_application` tool for creating new applications
- Add `rollback_application` tool for rolling back to previous versions
- Add `get_application_resources` tool to list managed resources
- Add support for application sets
- Implement caching for frequently accessed data
- Add webhook support for real-time notifications