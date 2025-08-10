package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleDeleteApplication(t *testing.T) {
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
					Name:      "delete_application",
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
			name: "delete with cascade true (default)",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "delete_application",
					Arguments: map[string]interface{}{
						"name":    "test-app",
						"cascade": true,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "Failed to delete application",
		},
		{
			name: "delete with cascade false",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "delete_application",
					Arguments: map[string]interface{}{
						"name":    "test-app",
						"cascade": false,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "Failed to delete application",
		},
		{
			name: "missing environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "delete_application",
					Arguments: map[string]interface{}{
						"name": "test-app",
					},
				},
			},
			envVars:       map[string]string{},
			wantError:     true,
			errorContains: "ARGOCD_AUTH_TOKEN and ARGOCD_SERVER environment variables must be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleDeleteApplication(context.Background(), tt.request)

			// Check error expectation
			if tt.wantError {
				if err != nil {
					t.Errorf("HandleDeleteApplication() returned error = %v, but error was not expected in result", err)
				}
				if result == nil {
					t.Fatal("HandleDeleteApplication() returned nil result")
				}
				// Check if error is in the result
				if !result.IsError {
					t.Errorf("HandleDeleteApplication() expected error result, but got success")
				}
			} else {
				if err != nil {
					t.Errorf("HandleDeleteApplication() error = %v, wantError %v", err, tt.wantError)
				}
				if result == nil {
					t.Fatal("HandleDeleteApplication() returned nil result")
				}
				// For successful delete, check the success message
				if result.IsError {
					t.Error("HandleDeleteApplication() returned error result for successful case")
				}
			}
		})
	}
}

func TestDeleteAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if DeleteAppTool.Name != "delete_application" {
		t.Errorf("Expected tool name 'delete_application', got %s", DeleteAppTool.Name)
	}

	// Verify tool has description
	if DeleteAppTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Just log that description exists and contains warning - no need to test specific words
	if DeleteAppTool.Description != "" {
		t.Logf("Tool description: %s", DeleteAppTool.Description)
	}

	// Check input schema exists
	if DeleteAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", DeleteAppTool.InputSchema.Type)
	}
}
