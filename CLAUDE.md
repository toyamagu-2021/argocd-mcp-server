# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Important Security Rules

**NEVER read .env files** - These files contain sensitive credentials and authentication tokens. If you need to understand environment variables, refer to the documentation or ask the user directly.

## Common Development Commands

### Build
```bash
go build -o argocd-mcp-server ./cmd/argocd-mcp-server
```

### Run Tests
```bash
# Run all tests with gotestsum (default)
make test

# Run basic go tests
go test ./...
make test-basic

# Run tests with verbose output
go test -v ./...
make test-verbose

# Run tests with race detector
go test -race ./...
make test-race

# Run tests with gotestsum (better output)
make test-pretty

# Run tests in watch mode
make test-watch

# Run tests with coverage
go test -cover ./...
make test-cover

# Generate coverage profile and HTML report
make test-coverprofile

# Run tests with gotestsum and coverage HTML report
make test-coverage-pretty

# Run specific package tests
go test ./internal/argocd/
go test ./internal/argocd/client/
go test ./internal/tools/
go test ./internal/config/
go test ./internal/grpcwebproxy/
go test ./internal/server/
```

### Linting
```bash
# Format code (includes goimports)
make fmt

# Check formatting (includes import formatting)
make fmt-check

# Run go vet
make vet

# Run all linters (fmt-check, vet, staticcheck, golint, ineffassign, misspell)
make lint

# Run basic linters only (fmt-check, vet)
make lint-basic

# Install all linter tools (includes goimports)
make lint-install

# Run advanced linting including security checks (gosec, revive)
make lint-advanced

# Run all quality checks (lint + test)
make check

# Run all checks including advanced linting and race detection
make check-all
```

### Development Setup
```bash
# Install dependencies
go mod download
make deps

# Verify dependencies (included in make deps)
go mod verify

# Tidy dependencies
go mod tidy
make tidy
```

### Running the Server
```bash
# Set required environment variables
export ARGOCD_AUTH_TOKEN=$(argocd account generate-token)
export ARGOCD_SERVER=your-argocd-server.com

# Optional: Configure connection options
export ARGOCD_INSECURE=false         # Skip TLS verification (default: false)
export ARGOCD_PLAINTEXT=false        # Use plaintext connection (default: false)

# Optional: Enable gRPC-Web support
export ARGOCD_GRPC_WEB=true           # Enable gRPC-Web proxy (default: false)
export ARGOCD_GRPC_WEB_ROOT_PATH=""   # gRPC-Web root path (optional)

# Optional: Configure logging
export LOG_LEVEL=debug  # Options: debug, info, warn, error
export LOG_FORMAT=json  # Options: text, json

# Run the server
./argocd-mcp-server
```

### Testing without ArgoCD
```bash
# Run unit tests (no ArgoCD server required)
make test

# Run tests with coverage
make test-cover
```

### E2E Testing with Kind Cluster
```bash
# Prerequisites - check if required tools are installed
make check-tools

# Create Kind cluster for testing (cluster name: argocd-mcp-server)
make kind-create

# Install Argo CD v3.6.2 in the cluster
make install-argocd

# Complete E2E setup (cluster + ArgoCD + .env generation)
make e2e-setup

# Port forward ArgoCD to localhost:8080 (run in separate terminal)
make argocd-port-forward

# Generate ArgoCD auth token via CLI (requires port-forward and argocd CLI)
make generate-token

# Alternative: Generate token via API (requires port-forward)
make generate-token-api

# Get ArgoCD admin password
make argocd-password

# Show cluster and ArgoCD pod information
make cluster-info

# Run E2E tests (add your test commands to Makefile)
make e2e-test

# Teardown E2E environment (delete Kind cluster)
make e2e-teardown

# Complete E2E flow (setup, test, teardown)
make e2e

# Verify kubectl context is correct
make check-context
```

## Architecture Overview

This is a Model Context Protocol (MCP) server that exposes ArgoCD operations as tools for LLMs. The architecture follows a clean separation of concerns:

### Core Components

**MCP Server Layer** (`internal/server/`)
- Handles MCP protocol communication via stdin/stdout
- Uses the `mark3labs/mcp-go` library for MCP implementation
- Includes recovery middleware to handle panics gracefully

**Tools Layer** (`internal/tools/`)
- Defines MCP tool schemas and handlers for ArgoCD operations
- Each tool has its own file: schema, `makeX(d Deps)` closure, and a testable `xHandler(ctx, client, ...)`
- `register.go` registers all tools with `Register(s, Deps{Client, ReadOnly})`; state-changing tools are skipped when `ARGOCD_MCP_READ_ONLY=true`
- Handlers use the process-wide client from `Deps` and never read env, dial or close it

**gRPC Client Layer** (`internal/argocd/client/`)
- Direct gRPC client for ArgoCD server communication
- Uses Argo CD v3.6 protocol buffer definitions (Argo CD 3.6.x is the only supported version)
- Handles JWT authentication via gRPC credentials
- Provides type-safe request/response handling
- Supports both secure and insecure connections
- Optional gRPC-Web proxy support for environments without direct gRPC access

**gRPC-Web Proxy Layer** (`internal/grpcwebproxy/`)
- Transparent gRPC-Web to gRPC translation layer
- Creates local Unix socket proxy server for gRPC communication
- Converts gRPC calls to HTTP/gRPC-Web requests with proper framing
- Handles streaming responses and proper resource cleanup
- Automatically enabled when ARGOCD_GRPC_WEB=true

**Configuration** (`internal/config/`)
- `config.Load()` is the only place that reads connection env vars
- `ToClientConfig()` converts to `client.Config`

**Logging** (`internal/logging/`)
- Structured logging using logrus
- Writes to stderr to avoid interfering with MCP protocol on stdout
- Configurable via environment variables

### Key Design Decisions

1. **gRPC Communication**: Uses ArgoCD gRPC API for efficient communication and full feature support

2. **Environment-based Configuration**: All configuration via environment variables:
   - `ARGOCD_AUTH_TOKEN`: Authentication token (required)
   - `ARGOCD_SERVER`: Server address with port (required)
   - `ARGOCD_INSECURE`: Skip TLS verification (optional, default: false)
   - `ARGOCD_PLAINTEXT`: Use plaintext connection (optional, default: false)
   - `ARGOCD_GRPC_WEB`: Enable gRPC-Web proxy mode (optional, default: false)
   - `ARGOCD_GRPC_WEB_ROOT_PATH`: Custom root path for gRPC-Web requests (optional)
   - `ARGOCD_GRPC_MAX_SIZE_MB`: Max gRPC message size in MiB (optional, default: 200)
   - `ARGOCD_MCP_READ_ONLY`: Do not register state-changing tools (optional, default: false)

3. **MCP Protocol**: Communicates via stdin/stdout using JSON-RPC format as per MCP specification

4. **Tool Safety**: Includes safety features like dry-run mode for sync operations and cascade control for deletions

## MCP Tools Available

The server exposes a fixed set of 23 public tools (design D1). The full inventory, with
which tools are mutating (removed by `ARGOCD_MCP_READ_ONLY=true`), is in
`docs/todo-tools.md`, and the locked list is `publicTools` in
`internal/tools/register_test.go`.

- Applications: `list_application`, `get_application`, `get_application_manifests`, `get_application_events`, `get_application_logs`, `get_application_resource_tree`, `create_application`, `sync_application`, `refresh_application`, `delete_application` (`cascade` defaults to false), `terminate_operation`
- ApplicationSets: `list_applicationset`, `get_applicationset`, `create_applicationset`, `delete_applicationset`
- Projects: `list_project`, `get_project`, `create_project`
- Clusters: `list_cluster`, `get_cluster`
- Repositories: `list_repository`, `get_repository`
- Session: `get_user_info`

To add or change a tool, follow `docs/TOOL_IMPLEMENTATION_GUIDE.md`.
