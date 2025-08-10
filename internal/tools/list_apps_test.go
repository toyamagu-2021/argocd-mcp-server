package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleListApplications(t *testing.T) {
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
					Name:      "list_application",
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
		{
			name: "with filter parameters",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "list_application",
					Arguments: map[string]interface{}{
						"project":   "default",
						"cluster":   "in-cluster",
						"namespace": "argocd",
						"selector":  "app=test",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "failed to list applications",
		},
		{
			name: "empty filter parameters",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "list_application",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "failed to list applications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleListApplications(context.Background(), tt.request)

			// Check error expectation
			if tt.wantError {
				if err != nil {
					t.Errorf("HandleListApplications() returned error = %v, but error was not expected in result", err)
				}
				if result == nil {
					t.Fatal("HandleListApplications() returned nil result")
				}
				// Check if error is in the result
				if !result.IsError {
					t.Errorf("HandleListApplications() expected error result, but got success")
				}
				// Check error content is present (we know it's an error result)
				if tt.errorContains != "" && len(result.Content) == 0 {
					t.Errorf("HandleListApplications() expected error content, but got empty")
				}
			} else {
				if err != nil {
					t.Errorf("HandleListApplications() error = %v, wantError %v", err, tt.wantError)
				}
			}
		})
	}
}

func TestListAppsTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if ListAppsTool.Name != "list_application" {
		t.Errorf("Expected tool name 'list_application', got %s", ListAppsTool.Name)
	}

	// Verify tool has description
	if ListAppsTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if ListAppsTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", ListAppsTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if ListAppsTool.InputSchema.Properties == nil || len(ListAppsTool.InputSchema.Properties) == 0 {
		t.Error("Tool schema should have properties defined")
	}
}
