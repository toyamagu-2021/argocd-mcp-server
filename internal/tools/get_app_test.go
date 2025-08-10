package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleGetApplication(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		envVars       map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "missing application name",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_application",
					Arguments: map[string]interface{}{},
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
			name: "missing environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_application",
					Arguments: map[string]interface{}{
						"name": "test-app",
					},
				},
			},
			envVars:       map[string]string{},
			wantError:     true,
			errorContains: "ARGOCD_AUTH_TOKEN and ARGOCD_SERVER environment variables must be set",
		},
		{
			name: "valid request",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_application",
					Arguments: map[string]interface{}{
						"name": "test-app",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "Failed to get application",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleGetApplication(context.Background(), tt.request)

			// Check error expectation
			if tt.wantError {
				if err != nil {
					t.Errorf("HandleGetApplication() returned error = %v, but error was not expected in result", err)
				}
				if result == nil {
					t.Fatal("HandleGetApplication() returned nil result")
				}
				// Check if error is in the result
				if !result.IsError {
					t.Errorf("HandleGetApplication() expected error result, but got success")
				}
			} else {
				if err != nil {
					t.Errorf("HandleGetApplication() error = %v, wantError %v", err, tt.wantError)
				}
			}
		})
	}
}

func TestGetAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetAppTool.Name != "get_application" {
		t.Errorf("Expected tool name 'get_application', got %s", GetAppTool.Name)
	}

	// Verify tool has description
	if GetAppTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetAppTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if GetAppTool.InputSchema.Properties == nil || len(GetAppTool.InputSchema.Properties) == 0 {
		t.Error("Tool should have properties defined")
	}

	// Check that we have required fields defined
	if GetAppTool.InputSchema.Required == nil || len(GetAppTool.InputSchema.Required) == 0 {
		t.Error("Tool should have required fields defined")
	}
}
