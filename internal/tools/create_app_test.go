package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleCreateApplication(t *testing.T) {
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
					Name: "create_application",
					Arguments: map[string]interface{}{
						"name":           "test-app",
						"repo_url":       "https://github.com/example/repo",
						"dest_namespace": "default",
					},
				},
			},
			envVars:       map[string]string{},
			wantError:     true,
			errorContains: "ARGOCD_AUTH_TOKEN and ARGOCD_SERVER environment variables must be set",
		},
		{
			name: "missing required name parameter",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_application",
					Arguments: map[string]interface{}{
						"repo_url":       "https://github.com/example/repo",
						"dest_namespace": "default",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true,
			errorContains: "Application name is required",
		},
		{
			name: "missing required repo_url parameter",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_application",
					Arguments: map[string]interface{}{
						"name":           "test-app",
						"dest_namespace": "default",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true,
			errorContains: "Repository URL is required",
		},
		{
			name: "missing required dest_namespace parameter",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_application",
					Arguments: map[string]interface{}{
						"name":     "test-app",
						"repo_url": "https://github.com/example/repo",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true,
			errorContains: "Destination namespace is required",
		},
		{
			name: "valid minimal request",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_application",
					Arguments: map[string]interface{}{
						"name":           "test-app",
						"repo_url":       "https://github.com/example/repo",
						"dest_namespace": "default",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because gRPC server is not actually running
			errorContains: "Failed to create gRPC client",
		},
		{
			name: "valid request with all parameters",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_application",
					Arguments: map[string]interface{}{
						"name":            "test-app",
						"namespace":       "argocd",
						"project":         "my-project",
						"repo_url":        "https://github.com/example/repo",
						"path":            "manifests/",
						"target_revision": "main",
						"dest_server":     "https://kubernetes.default.svc",
						"dest_namespace":  "production",
						"upsert":          true,
						"auto_sync":       true,
						"self_heal":       true,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because gRPC server is not actually running
			errorContains: "Failed to create gRPC client",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleCreateApplication(context.Background(), tt.request)

			// Check error expectation
			if tt.wantError {
				if err != nil {
					t.Errorf("HandleCreateApplication() returned error = %v, but error was not expected in result", err)
				}
				if result == nil {
					t.Fatal("HandleCreateApplication() returned nil result")
				}
				// Check if error is in the result
				if !result.IsError {
					t.Errorf("HandleCreateApplication() expected error result, but got success")
				}
				// Check error content is present (we know it's an error result)
				if tt.errorContains != "" && len(result.Content) == 0 {
					t.Errorf("HandleCreateApplication() expected error content, but got empty")
				}
			} else {
				if err != nil {
					t.Errorf("HandleCreateApplication() error = %v, wantError %v", err, tt.wantError)
				}
			}
		})
	}
}

func TestCreateAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if CreateAppTool.Name != "create_application" {
		t.Errorf("Expected tool name 'create_application', got %s", CreateAppTool.Name)
	}

	// Verify tool has description
	if CreateAppTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if CreateAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", CreateAppTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(CreateAppTool.InputSchema.Properties) == 0 {
		t.Error("Tool schema should have properties defined")
	}

	// Check required fields are marked as required
	expectedRequired := []string{"name", "repo_url", "dest_namespace"}
	if CreateAppTool.InputSchema.Required == nil {
		t.Error("Tool schema should have required fields defined")
	} else {
		for _, req := range expectedRequired {
			found := false
			for _, actual := range CreateAppTool.InputSchema.Required {
				if actual == req {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected required field '%s' not found in schema", req)
			}
		}
	}
}
