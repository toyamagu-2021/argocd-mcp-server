# Tool Implementation Guide

This guide provides step-by-step instructions for implementing new tools in the ArgoCD MCP server. It covers the complete implementation process including the tool handler, tests, and integration.

## Table of Contents

1. [Overview](#overview)
2. [Implementation Steps](#implementation-steps)
3. [File Structure](#file-structure)
4. [Code Examples](#code-examples)
5. [Testing Strategy](#testing-strategy)
6. [Checklist](#checklist)

## Overview

Each tool in the ArgoCD MCP server follows a consistent pattern:
- Tool handler that processes MCP requests
- Unit tests for the handler logic
- E2E tests with real ArgoCD server
- Mock E2E tests for CI/CD environments
- Integration with the MCP server

## Implementation Steps

### Step 1: Implement the Tool Handler

Create a new file in `internal/tools/` for your tool handler.

**File naming convention:**
- List operations: `list_<resource>.go`
- Get operations: `get_<resource>.go`
- Create operations: `create_<resource>.go`
- Update operations: `update_<resource>.go`
- Delete operations: `delete_<resource>.go`
- Action operations: `<action>_<resource>.go`

**Handler structure:**

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "os"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// Define the tool schema
var <ToolName>Tool = mcp.NewTool("<tool_name>",
    mcp.WithDescription("Description of what the tool does"),
    // Add required parameters
    mcp.WithString("param_name",
        mcp.Required(),
        mcp.Description("Parameter description"),
    ),
    // Add optional parameters
    mcp.WithString("optional_param",
        mcp.Description("Optional parameter description"),
    ),
)

// Handle<ToolName> processes <tool_name> tool requests
func Handle<ToolName>(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // Extract parameters
    requiredParam := request.GetString("param_name", "")
    if requiredParam == "" {
        return mcp.NewToolResultError("param_name is required"), nil
    }
    
    optionalParam := request.GetString("optional_param", "default_value")
    
    // Create gRPC client
    config := &client.Config{
        ServerAddr:      os.Getenv("ARGOCD_SERVER"),
        AuthToken:       os.Getenv("ARGOCD_AUTH_TOKEN"),
        Insecure:        os.Getenv("ARGOCD_INSECURE") == "true",
        PlainText:       os.Getenv("ARGOCD_PLAINTEXT") == "true",
        GRPCWeb:         os.Getenv("ARGOCD_GRPC_WEB") == "true",
        GRPCWebRootPath: os.Getenv("ARGOCD_GRPC_WEB_ROOT_PATH"),
    }
    
    argoClient, err := client.New(config)
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Failed to create gRPC client: %v", err)), nil
    }
    defer func() { _ = argoClient.Close() }()
    
    // Use the handler function with the real client
    return <toolName>Handler(ctx, argoClient, requiredParam, optionalParam)
}

// <toolName>Handler handles the core logic for the tool.
// This is separated out to enable testing with mocked clients.
func <toolName>Handler(
    ctx context.Context,
    argoClient client.Interface,
    requiredParam string,
    optionalParam string,
) (*mcp.CallToolResult, error) {
    // Call the appropriate client method
    result, err := argoClient.<ClientMethod>(ctx, requiredParam)
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Failed to <action>: %v", err)), nil
    }
    
    // Convert to JSON for better readability in MCP responses
    jsonData, err := json.MarshalIndent(result, "", "  ")
    if err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
    }
    
    return mcp.NewToolResultText(string(jsonData)), nil
}
```

### Step 2: Add Unit Tests

Create a test file alongside your tool handler: `<tool_name>_test.go`

**Test structure:**

```go
package tools

import (
    "context"
    "encoding/json"
    "testing"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
    "go.uber.org/mock/gomock"
)

// Test the tool handler with environment variables
func TestHandle<ToolName>(t *testing.T) {
    tests := []struct {
        name          string
        request       mcp.CallToolRequest
        envVars       map[string]string
        wantError     bool
        errorContains string
    }{
        {
            name: "missing environment variables",
            request: mcp.CallToolRequest{
                Params: mcp.CallToolParams{
                    Name:      "<tool_name>",
                    Arguments: map[string]interface{}{},
                },
            },
            envVars: map[string]string{
                "ARGOCD_AUTH_TOKEN": "",
                "ARGOCD_SERVER":     "",
            },
            wantError:     true,
            errorContains: "server address is required",
        },
        // Add more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Set environment variables
            for k, v := range tt.envVars {
                t.Setenv(k, v)
            }
            
            // Execute handler
            result, err := Handle<ToolName>(context.Background(), tt.request)
            
            // Check expectations
            if tt.wantError {
                require.Nil(t, err)
                require.NotNil(t, result)
                assert.True(t, result.IsError)
                if tt.errorContains != "" && len(result.Content) > 0 {
                    assert.Contains(t, result.Content[0], tt.errorContains)
                }
            } else {
                require.Nil(t, err)
                require.NotNil(t, result)
                assert.False(t, result.IsError)
            }
        })
    }
}

// Test the tool schema
func Test<ToolName>Tool_Schema(t *testing.T) {
    // Verify tool is properly defined
    if <ToolName>Tool.Name != "<tool_name>" {
        t.Errorf("Expected tool name '<tool_name>', got %s", <ToolName>Tool.Name)
    }
    
    // Verify tool has description
    if <ToolName>Tool.Description == "" {
        t.Error("Tool description should not be empty")
    }
    
    // Check input schema exists
    if <ToolName>Tool.InputSchema.Type != "object" {
        t.Errorf("Expected schema type 'object', got %s", <ToolName>Tool.InputSchema.Type)
    }
    
    // Check required parameters
    // Add specific checks for your tool's parameters
}

// Test the handler logic with mocked client
func Test<ToolName>Handler(t *testing.T) {
    tests := []struct {
        name        string
        param1      string
        param2      string
        setupMock   func(*mock.MockInterface)
        wantError   bool
        wantMessage string
    }{
        {
            name:   "successful operation",
            param1: "value1",
            param2: "value2",
            setupMock: func(m *mock.MockInterface) {
                // Setup mock expectations
                m.EXPECT().<ClientMethod>(gomock.Any(), "value1").Return(expectedResult, nil)
            },
            wantError: false,
        },
        {
            name:   "operation fails",
            param1: "value1",
            param2: "value2",
            setupMock: func(m *mock.MockInterface) {
                m.EXPECT().<ClientMethod>(gomock.Any(), "value1").Return(nil, assert.AnError)
            },
            wantError: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctrl := gomock.NewController(t)
            defer ctrl.Finish()
            
            mockClient := mock.NewMockInterface(ctrl)
            tt.setupMock(mockClient)
            
            result, err := <toolName>Handler(context.Background(), mockClient, tt.param1, tt.param2)
            
            if tt.wantError {
                require.Nil(t, err)
                require.NotNil(t, result)
                assert.True(t, result.IsError)
            } else {
                require.Nil(t, err)
                require.NotNil(t, result)
                assert.False(t, result.IsError)
                
                if tt.wantMessage != "" {
                    require.Len(t, result.Content, 1)
                    textContent, ok := mcp.AsTextContent(result.Content[0])
                    require.True(t, ok)
                    assert.Contains(t, textContent.Text, tt.wantMessage)
                }
            }
        })
    }
}
```

### Step 3: Register the Tool

Add your tool to `internal/tools/tools.go`:

```go
func RegisterAll(s *server.MCPServer) {
    // ... existing tools ...
    
    // Register your new tool
    s.AddTool(<ToolName>Tool, Handle<ToolName>)
}
```

### Step 4: Update gRPC Client (if needed)

If your tool requires new client methods, add them to:

1. **Interface definition** in `internal/argocd/client/interface.go`:
```go
type Interface interface {
    // ... existing methods ...
    
    // Your new method
    <MethodName>(ctx context.Context, param string) (*ReturnType, error)
}
```

2. **Implementation** in `internal/argocd/client/client.go`:
```go
func (c *Client) <MethodName>(ctx context.Context, param string) (*ReturnType, error) {
    req := &servicepkg.RequestType{
        Field: param,
    }
    resp, err := c.serviceClient.Method(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to <action>: %w", err)
    }
    return resp, nil
}
```

### Step 5: Add E2E Tests

Create E2E tests in `test/argocd_e2e/`:

```go
func test<ToolName>(t *testing.T) {
    mcpCmd, stdin, stdout := startMCPServer(t)
    defer func() {
        _ = mcpCmd.Process.Kill()
        _ = mcpCmd.Wait()
    }()
    
    initializeMCPConnection(t, stdin, stdout)
    
    callToolRequest := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      2,
        "method":  "tools/call",
        "params": map[string]interface{}{
            "name": "<tool_name>",
            "arguments": map[string]interface{}{
                "param_name": "value",
            },
        },
    }
    
    response := sendRequest(t, stdin, stdout, callToolRequest)
    
    // Verify response
    result, ok := response["result"].(map[string]interface{})
    if !ok {
        t.Fatalf("expected result to be a map, got %T", response["result"])
    }
    
    // Add specific assertions for your tool
}
```

Add the test to the test suite in `argocd_test.go`:

```go
// In TestRealArgoCD_Suite function
t.Run("gRPC", func(t *testing.T) {
    // ... existing tests ...
    t.Run("<ToolName>", test<ToolName>)
})

t.Run("gRPC-Web", func(t *testing.T) {
    // ... existing tests ...
    t.Run("<ToolName>", test<ToolName>GRPCWeb)
})
```

### Step 6: Add Mock Server Support

If your tool uses a new service, update `test/mock/server.go`:

```go
// Add mock service implementation
type mock<Service>Service struct {
    service.Unimplemented<Service>ServiceServer
}

func (s *mock<Service>Service) <Method>(ctx context.Context, req *service.RequestType) (*service.ResponseType, error) {
    // Verify authentication
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "missing metadata")
    }
    
    auth := md.Get("authorization")
    if len(auth) == 0 || auth[0] != "Bearer test-token" {
        return nil, status.Error(codes.Unauthenticated, "invalid authorization")
    }
    
    // Return mock data
    return &service.ResponseType{
        // ... mock response ...
    }, nil
}

// Register in main()
func main() {
    // ... existing code ...
    
    s := grpc.NewServer()
    // ... existing registrations ...
    service.Register<Service>ServiceServer(s, &mock<Service>Service{})
}
```

### Step 7: Add Mock E2E Tests

Create mock E2E tests in `test/mock_argocd_e2e/`:

```go
func TestParallel_<ToolName>(t *testing.T) {
    t.Parallel()
    
    callToolRequest := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "tools/call",
        "params": map[string]interface{}{
            "name": "<tool_name>",
            "arguments": map[string]interface{}{
                "param_name": "value",
            },
        },
    }
    
    response := sendSharedRequest(t, callToolRequest)
    
    // Verify response
    result, ok := response["result"].(map[string]interface{})
    if !ok {
        t.Fatalf("expected result to be a map, got %T", response["result"])
    }
    
    // Add specific assertions for your tool
}
```

### Step 8: Update Documentation

After completing the implementation, update the documentation to reflect the new tool:

1. **Update todo-tools.md**: Move the implemented tool from the TODO section to the Completed section in `docs/todo-tools.md`
2. **Update README if needed**: Add the new tool to the README if it's a major feature
3. **Add usage examples**: Document how to use the new tool with example MCP requests

## File Structure

After implementing a new tool, you should have the following files:

```
argocd-mcp-server/
├── internal/
│   ├── tools/
│   │   ├── <tool_name>.go           # Tool handler implementation
│   │   ├── <tool_name>_test.go      # Unit tests
│   │   └── tools.go                 # Tool registration (updated)
│   └── argocd/
│       └── client/
│           ├── client.go            # Client implementation (if updated)
│           └── interface.go         # Interface definition (if updated)
├── test/
│   ├── argocd_e2e/
│   │   └── <resource>_test.go       # E2E tests with real ArgoCD
│   ├── mock/
│   │   └── server.go                # Mock server (if updated)
│   └── mock_argocd_e2e/
│       └── <resource>_e2e_test.go   # Mock E2E tests
└── docs/
    ├── TOOL_IMPLEMENTATION_GUIDE.md  # This guide
    └── todo-tools.md                 # Tool implementation status (update this!)
```

## Code Examples

### Example: List Tool

See `internal/tools/list_projects.go` for a complete example of a list operation.

### Example: Get Tool

See `internal/tools/get_project.go` for a complete example of a get operation.

### Example: Action Tool

See `internal/tools/sync_app.go` for a complete example of an action operation.

## Testing Strategy

### 1. Unit Tests
- Test tool schema validation
- Test parameter extraction
- Test handler logic with mocked clients
- Test error handling

### 2. Integration Tests
- Test with real gRPC client (mocked responses)
- Test environment variable configuration
- Test connection errors

### 3. E2E Tests
- Test with real ArgoCD server (optional, requires setup)
- Test with mock ArgoCD server (required for CI/CD)
- Test both gRPC and gRPC-Web modes

### Running Tests

```bash
# Run unit tests
go test ./internal/tools -v -run Test<ToolName>

# Run mock E2E tests
go test ./test/mock_argocd_e2e -v -run TestParallel_<ToolName>

# Run real E2E tests (requires ArgoCD server)
ARGOCD_SERVER=<server> ARGOCD_AUTH_TOKEN=<token> \
  go test ./test/argocd_e2e -v -run Test<ToolName>

# Run all tests
make test
```

## Checklist

When implementing a new tool, ensure you complete all these steps:

- [ ] **Tool Implementation**
  - [ ] Create tool handler file (`internal/tools/<tool_name>.go`)
  - [ ] Define tool schema with proper descriptions
  - [ ] Implement handler function with error handling
  - [ ] Separate handler logic for testability
  - [ ] Format response as JSON

- [ ] **Unit Tests**
  - [ ] Create test file (`internal/tools/<tool_name>_test.go`)
  - [ ] Test tool schema
  - [ ] Test handler with various inputs
  - [ ] Test error cases
  - [ ] Use mocked client for handler tests

- [ ] **Integration**
  - [ ] Register tool in `tools.go`
  - [ ] Update client interface if needed
  - [ ] Implement client method if needed

- [ ] **E2E Tests**
  - [ ] Add E2E test function
  - [ ] Register in test suite
  - [ ] Test both gRPC and gRPC-Web modes
  - [ ] Handle authentication errors gracefully
  - [ ] Handle not-found errors appropriately

- [ ] **Mock Tests**
  - [ ] Update mock server if new service
  - [ ] Add mock E2E test
  - [ ] Test parallel execution
  - [ ] Verify response format

- [ ] **Documentation**
  - [ ] Update README if major feature
  - [ ] Add usage examples
  - [ ] Document environment variables
  - [ ] Update `docs/todo-tools.md` to move completed tools from TODO to Completed section

- [ ] **Code Quality**
  - [ ] Run `make fmt` to format code
  - [ ] Run `make lint` to check for issues
  - [ ] Run `make test` to verify all tests pass
  - [ ] Ensure no sensitive data in logs

## Common Patterns

### Error Handling

Always return errors as tool results, not Go errors:

```go
// Good
if err != nil {
    return mcp.NewToolResultError(fmt.Sprintf("Failed to %s: %v", action, err)), nil
}

// Bad - this will crash the MCP server
if err != nil {
    return nil, err
}
```

### Parameter Extraction

Use the helper methods for safe parameter extraction:

```go
// String parameters
name := request.GetString("name", "default_value")

// Boolean parameters
dryRun := request.GetBool("dry_run", false)

// Required parameters - check and return error
name := request.GetString("name", "")
if name == "" {
    return mcp.NewToolResultError("name is required"), nil
}
```

### Response Formatting

Always format responses as JSON for consistency:

```go
jsonData, err := json.MarshalIndent(result, "", "  ")
if err != nil {
    return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
}
return mcp.NewToolResultText(string(jsonData)), nil
```

### Resource Cleanup

Always defer cleanup operations:

```go
argoClient, err := client.New(config)
if err != nil {
    return mcp.NewToolResultError(fmt.Sprintf("Failed to create client: %v", err)), nil
}
defer func() { _ = argoClient.Close() }()
```

## Troubleshooting

### Common Issues

1. **"unknown service" error in tests**
   - Ensure the service is registered in the mock server
   - Check that you're using the updated mock server code

2. **"HTTP/1.x transport connection broken" error**
   - Ensure `ARGOCD_GRPC_WEB=false` for plain gRPC connections
   - Check that the server supports the connection type

3. **Tests hanging or timing out**
   - Check for proper cleanup in defer statements
   - Ensure processes are killed after tests
   - Look for port conflicts (use `lsof -i :<port>`)

4. **"Failed to parse JSON" in tests**
   - Log the actual response to debug
   - Check for error messages in the response
   - Verify the mock data format matches expectations

## Conclusion

Following this guide ensures consistent, well-tested tool implementations that integrate smoothly with the ArgoCD MCP server. Always prioritize:

1. **Testability** - Separate business logic from I/O operations
2. **Error handling** - Return user-friendly error messages
3. **Consistency** - Follow existing patterns and conventions
4. **Documentation** - Keep code self-documenting with clear names

For questions or improvements to this guide, please submit a pull request or open an issue.