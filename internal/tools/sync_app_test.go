package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestHandleSyncApplication(t *testing.T) {
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
					Name:      "sync_application",
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
			name: "sync with prune option",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "sync_application",
					Arguments: map[string]interface{}{
						"name":  "test-app",
						"prune": true,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "Failed to sync application",
		},
		{
			name: "sync with dry_run option",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "sync_application",
					Arguments: map[string]interface{}{
						"name":    "test-app",
						"dry_run": true,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "Failed to sync application",
		},
		{
			name: "sync with both prune and dry_run",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "sync_application",
					Arguments: map[string]interface{}{
						"name":    "test-app",
						"prune":   true,
						"dry_run": true,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "Failed to sync application",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleSyncApplication(context.Background(), tt.request)

			// Check error expectation
			if tt.wantError {
				if err != nil {
					t.Errorf("HandleSyncApplication() returned error = %v, but error was not expected in result", err)
				}
				if result == nil {
					t.Fatal("HandleSyncApplication() returned nil result")
				}
				// Check if error is in the result
				if !result.IsError {
					t.Errorf("HandleSyncApplication() expected error result, but got success")
				}
			} else {
				if err != nil {
					t.Errorf("HandleSyncApplication() error = %v, wantError %v", err, tt.wantError)
				}
				if result == nil {
					t.Fatal("HandleSyncApplication() returned nil result")
				}
				// For successful sync, check the success message
				if result.IsError {
					t.Error("HandleSyncApplication() returned error result for successful case")
				}
			}
		})
	}
}

func TestSyncAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if SyncAppTool.Name != "sync_application" {
		t.Errorf("Expected tool name 'sync_application', got %s", SyncAppTool.Name)
	}

	// Verify tool has description
	if SyncAppTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if SyncAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", SyncAppTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if SyncAppTool.InputSchema.Properties == nil || len(SyncAppTool.InputSchema.Properties) == 0 {
		t.Error("Tool should have properties defined")
	}

	// Check that we have required fields defined
	if SyncAppTool.InputSchema.Required == nil || len(SyncAppTool.InputSchema.Required) == 0 {
		t.Error("Tool should have required fields defined (name should be required)")
	}
}
