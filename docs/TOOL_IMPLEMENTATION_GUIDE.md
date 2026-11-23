# Tool Implementation Guide

How tools are built in the Argo CD MCP server (0.3 layout), and what to touch when you add
or change one.

> **Scope (design D1, revised 2026-11-23):** the server exposes 27 public tools. The
> revision added read tools `get_application_managed_resources` and
> `get_application_resource` (owner approved expanding read coverage after a gap
> analysis) and write tools `update_application` and `rollback_application` (owner
> approved these two write tools). Other write tools stay out of scope — see
> [`docs/design/refactor-0.3.md`](design/refactor-0.3.md) §0 and §7. The same steps apply
> when you change an existing tool or add another one.

## Table of Contents

1. [Architecture in one minute](#architecture-in-one-minute)
2. [Implementation Steps](#implementation-steps)
3. [File Structure](#file-structure)
4. [Testing Strategy](#testing-strategy)
5. [Checklist](#checklist)
6. [Common Patterns](#common-patterns)
7. [Troubleshooting](#troubleshooting)

## Architecture in one minute

```
main  ── config.Load() ──► client.New(cfg.ToClientConfig())   // one client per process
  │                                   │
  └── tools.Register(s, tools.Deps{Client: argo, ReadOnly: cfg.ReadOnly})
                │
                └── for each tool: s.AddTool(tool, makeX(deps))
```

- **`internal/config`** is the only place that reads connection environment variables.
- **One `client.Client` per process.** `main` creates it and closes it on shutdown. With
  gRPC-Web enabled, the client also owns the single local proxy.
- **Handlers never** call `os.Getenv`, `client.New`, or `Close()`. They use `d.Client`.
- Each tool file has three parts:
  1. the schema (`var XTool = mcp.NewTool(...)`),
  2. `makeX(d Deps) server.ToolHandlerFunc` — reads MCP arguments and calls the handler,
  3. `xHandler(ctx, client.Interface, ...)` — the logic, unit-tested with the gomock client.
- **`ARGOCD_MCP_READ_ONLY=true`** skips every entry marked `mutating` in `register.go`.

## Implementation Steps

### Step 1: Define the schema and handler

Create `internal/tools/<verb>_<resource>.go`
(`list_`, `get_`, `create_`, `delete_`, or `<action>_<resource>.go`).

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
    "github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetWidgetTool defines the get_widget tool schema.
var GetWidgetTool = mcp.NewTool("get_widget",
    mcp.WithDescription("Retrieves a widget."),
    // Hints: read tools are readOnly and non-destructive. Write tools must set
    // ReadOnlyHint(false) and an honest DestructiveHint.
    mcp.WithReadOnlyHintAnnotation(true),
    mcp.WithDestructiveHintAnnotation(false),
    mcp.WithString("name",
        mcp.Required(),
        mcp.Description("The name of the widget."),
    ),
    mcp.WithString("app_namespace",
        mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed for apps-in-any-namespace."),
    ),
)

// makeGetWidget returns the handler for this tool bound to the shared client in d.
func makeGetWidget(d Deps) server.ToolHandlerFunc {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        name := request.GetString("name", "")
        appNamespace := request.GetString("app_namespace", "")
        return getWidgetHandler(ctx, d.Client, name, appNamespace)
    }
}

// getWidgetHandler holds the tool logic; it is unit-tested with a mocked client.
func getWidgetHandler(ctx context.Context, argoClient client.Interface, name, appNamespace string) (*mcp.CallToolResult, error) {
    if name == "" {
        return mcp.NewToolResultError("name is required"), nil
    }
    w, err := argoClient.GetWidget(ctx, name, appNamespace)
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Failed to get widget: %v", err)), nil
    }
    jsonData, err := json.MarshalIndent(w, "", "  ")
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
    }
    return mcp.NewToolResultText(string(jsonData)), nil
}
```

Conventions:

- Argument names are `snake_case`. Use `app_namespace` for the Application's own namespace;
  `namespace` in `list_application` means the **destination** namespace.
- Defaults that are surprising, or that differ from the Argo CD CLI, go in the description
  (for example `delete_application`: `cascade` defaults to `false`).
- Anything that would block the stdio call forever (for example streaming with
  `follow=true`) must return a tool error instead.

### Step 2: Register the tool

Add one entry to `entries()` in `internal/tools/register.go`:

```go
{GetWidgetTool, makeGetWidget, false}, // third field: mutating
```

Set `mutating` to `true` for any tool that changes Argo CD state. Those tools are not
registered when `ARGOCD_MCP_READ_ONLY=true`. `refresh_application` is deliberately **not**
mutating (design D11, revised).

Then update the locked lists in `internal/tools/register_test.go`:

- `publicTools` — every public tool name;
- `mutatingTools` — the tools removed in read-only mode.

`TestToolHints` fails if the `mutating` flag and the `readOnlyHint` annotation disagree.

### Step 3: Client method (if needed)

1. Add the method to `Interface` in `internal/argocd/client/interface.go`. The interface only
   contains what public tools call.
2. Implement it in `internal/argocd/client/client.go`:
   - Return typed results (no `interface{}`).
   - With many optional arguments, take an options struct (see `LogsOptions`,
     `EventsOptions`, `ListApplicationsOptions`) instead of a long list of strings.
   - Send optional request fields only when set.
   - Endpoints that need the app's namespace/project for authorization can find them with
     `lookupApplication`.
   - **Cluster / Repository responses** must go through `sanitizeCluster` /
     `sanitizeRepository` (upstream `Sanitized()`), never a hand-written deny-list.
3. Regenerate the mock:

   ```bash
   cd internal/argocd/client && go run go.uber.org/mock/mockgen \
     -source=interface.go -destination=mock/mock_client.go -package=mock
   ```

4. Test the request construction in `internal/argocd/client/query_test.go` with the fake
   gRPC service client there.

### Step 4: Unit tests

In `internal/tools/<tool>_test.go`:

- **Schema test** — name, description, required parameters.
- **Handler test** — table-driven, with `mock.NewMockInterface`, covering success, client
  errors and validation errors.

In `internal/tools/wiring_test.go` (or `register_test.go`), add a row that calls the tool
**through the registered server** and asserts the exact client call. This catches typos in
argument names, which handler tests cannot:

```go
{
    name: "get_widget",
    tool: "get_widget",
    args: map[string]interface{}{"name": "w", "app_namespace": "team-a"},
    expect: func(m *mock.MockInterface) {
        m.EXPECT().GetWidget(gomock.Any(), "w", "team-a").Return(&Widget{}, nil)
    },
},
```

Configuration and environment handling are tested in `internal/config`, not in tool tests.

### Step 5: Mock server and mock E2E tests

If the tool uses a new gRPC service or method, implement it in `test/mock/server.go`. Check
the `authorization: Bearer test-token` metadata like the existing services do. Then add a
test in `test/mock_argocd_e2e/`:

```go
func TestParallel_GetWidget(t *testing.T) {
    t.Parallel()
    response := sendSharedRequest(t, map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "tools/call",
        "params": map[string]interface{}{
            "name":      "get_widget",
            "arguments": map[string]interface{}{"name": "w"},
        },
    })
    if _, ok := response["result"].(map[string]interface{}); !ok {
        t.Fatalf("expected result map, got %T", response["result"])
    }
}
```

### Step 6: Real Argo CD E2E tests

Add a `test<ToolName>(t)` function in `test/argocd_e2e/` and call it from
`TestRealArgoCD_Suite` in `argocd_parallel_test.go`. The suite runs once over gRPC and, when
`ARGOCD_MCP_E2E_GRPC_WEB` is set, once over gRPC-Web. CI runs both against Argo CD 3.6.2 in
Kind (the gRPC-Web pass runs the independent tests).

### Step 7: Documentation

- `README.md` — feature list, an example request, and safety notes for write tools.
- `CLAUDE.md` — the tool list.
- `docs/todo-tools.md` — the public tool inventory.

## File Structure

```
argocd-mcp-server/
├── cmd/argocd-mcp-server/main.go     # wiring only
├── internal/
│   ├── config/                       # env → Config (the only env reader)
│   ├── argocd/client/
│   │   ├── interface.go              # Interface + options structs
│   │   ├── client.go                 # gRPC implementation
│   │   ├── sanitize.go               # Cluster/Repository Sanitized()
│   │   └── mock/mock_client.go       # generated (mockgen)
│   ├── grpcwebproxy/                 # gRPC-Web proxy (owned by the client)
│   ├── server/                       # MCP server construction
│   └── tools/
│       ├── deps.go                   # Deps{Client, ReadOnly}
│       ├── register.go               # entries() + Register
│       ├── register_test.go          # locked tool lists, hints, read-only
│       ├── wiring_test.go            # argument → client call wiring
│       └── <verb>_<resource>.go(+_test.go)
├── test/
│   ├── mock/server.go                # mock Argo CD gRPC server
│   ├── mock_argocd_e2e/              # E2E against the mock server
│   └── argocd_e2e/                   # E2E against a real Argo CD
└── docs/
    ├── design/refactor-0.3.md
    ├── TOOL_IMPLEMENTATION_GUIDE.md
    └── todo-tools.md
```

## Testing Strategy

| Layer | Where | What |
|---|---|---|
| Config | `internal/config` | env parsing, defaults, validation |
| Client | `internal/argocd/client` | request construction (fake service), sanitizing, Close |
| Tools | `internal/tools` | schema, handler logic (gomock), argument wiring, registration and hints |
| Mock E2E | `test/mock_argocd_e2e` | the real binary over stdio against the mock gRPC server |
| Real E2E | `test/argocd_e2e` | the real binary against Argo CD 3.6.x (gRPC and gRPC-Web) |

```bash
go test ./internal/...                         # unit tests
go test ./test/mock_argocd_e2e/                # mock E2E
ARGOCD_SERVER=<server> ARGOCD_AUTH_TOKEN=<token> ARGOCD_INSECURE=true \
  go test ./test/argocd_e2e -run TestRealArgoCD_Suite   # real E2E
make test                                      # everything
```

## Checklist

- [ ] **Tool**
  - [ ] Schema with clear descriptions, and every surprising default documented
  - [ ] Correct hints (`readOnlyHint`, `destructiveHint`, `idempotentHint` where relevant)
  - [ ] `makeX(d Deps)` reads the arguments; `xHandler` holds the logic
  - [ ] No `os.Getenv`, `client.New` or `Close()` in the tool
- [ ] **Registration**
  - [ ] Entry in `register.go` with the right `mutating` flag
  - [ ] `publicTools` / `mutatingTools` updated in `register_test.go`
- [ ] **Client** (if needed)
  - [ ] Typed return value; options struct for many optional arguments
  - [ ] Cluster/Repository results sanitized
  - [ ] Mock regenerated; request construction tested
- [ ] **Tests**
  - [ ] Schema, handler and wiring tests
  - [ ] Mock server and mock E2E test
  - [ ] Real E2E test added to the suite
- [ ] **Docs**: README, CLAUDE.md, `docs/todo-tools.md`
- [ ] **Quality**: `make fmt`, `make lint`, `make test`

## Common Patterns

### Error handling

Return errors as tool results, not Go errors:

```go
if err != nil {
    return mcp.NewToolResultError(fmt.Sprintf("Failed to %s: %v", action, err)), nil
}
```

### Parameter extraction

```go
name := request.GetString("name", "")        // required: validate in xHandler
dryRun := request.GetBool("dry_run", false)  // optional with default
tail := int64(request.GetInt("tail_lines", 20))
```

### Response formatting

```go
jsonData, err := json.MarshalIndent(result, "", "  ")
if err != nil {
    return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
}
return mcp.NewToolResultText(string(jsonData)), nil
```

Large list responses may offer a compact TSV format (see `list_apps_tsv.go`).

### Client lifetime

Tools never create or close the client. `main` owns it:

```go
argo, err := client.New(cfg.ToClientConfig())
if err != nil { /* fatal at startup */ }
defer func() { _ = argo.Close() }() // Close is idempotent
tools.Register(s, tools.Deps{Client: argo, ReadOnly: cfg.ReadOnly})
```

## Troubleshooting

1. **gomock "missing call" or "unexpected call" in wiring tests** — an argument name in the
   schema or in `makeX` doesn't match what the test sends, or a default changed.
2. **`TestRegister_AllPublicTools` or `TestToolHints` fails** — update `publicTools` /
   `mutatingTools`, or fix the `mutating` flag or the hints.
3. **"unknown service" in mock E2E** — register the service in `test/mock/server.go`.
4. **"HTTP/1.x transport connection broken"** — check that `ARGOCD_GRPC_WEB` matches what
   the server supports.
5. **Tests hang** — make sure test MCP processes are killed. Never add a tool that streams
   forever over stdio.
