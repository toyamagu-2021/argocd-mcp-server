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
func TestHandleGetApplicationManifests(t *testing.T) {
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
					Name: "get_application_manifests",
					Arguments: map[string]interface{}{
						"name": "test-app",
					},
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
			name: "missing required application name",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_application_manifests",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "argocd.example.com:443",
			},
			wantError:     true,
			errorContains: "Application name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleGetApplicationManifests(context.Background(), tt.request)

			// Check expectations
			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" && len(result.Content) > 0 {
					textContent, ok := result.Content[0].(mcp.TextContent)
					if ok {
						assert.Contains(t, textContent.Text, tt.errorContains)
					}
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
func TestGetAppManifestsTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetAppManifestsTool.Name != "get_application_manifests" {
		t.Errorf("Expected tool name 'get_application_manifests', got %s", GetAppManifestsTool.Name)
	}

	// Verify tool has description
	if GetAppManifestsTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetAppManifestsTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetAppManifestsTool.InputSchema.Type)
	}

	// Check required parameters
	require.Contains(t, GetAppManifestsTool.InputSchema.Required, "name")
	require.NotContains(t, GetAppManifestsTool.InputSchema.Required, "revision") // revision is optional

	// Check parameter properties
	props := GetAppManifestsTool.InputSchema.Properties
	require.NotNil(t, props)

	// Check name parameter
	nameProp, ok := props["name"].(map[string]interface{})
	require.True(t, ok, "name property should be a map")
	assert.Equal(t, "string", nameProp["type"])
	assert.NotEmpty(t, nameProp["description"])

	// Check revision parameter (optional)
	revisionProp, ok := props["revision"].(map[string]interface{})
	require.True(t, ok, "revision property should be a map")
	assert.Equal(t, "string", revisionProp["type"])
	assert.NotEmpty(t, revisionProp["description"])
}

// Test the handler logic with mocked client
func TestGetApplicationManifestsHandler(t *testing.T) {
	// Sample manifest response - simulate what ArgoCD returns
	mockManifestResponse := map[string]interface{}{
		"manifests": []string{
			"apiVersion: v1\nkind: Service\nmetadata:\n  name: test-service\n",
			"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-deployment\n",
		},
		"namespace": "default",
		"server":    "https://kubernetes.default.svc",
		"revision":  "abc123",
	}

	tests := []struct {
		name        string
		appName     string
		revision    string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:     "successful operation without revision",
			appName:  "test-app",
			revision: "",
			setupMock: func(m *mock.MockInterface) {
				// The mock interface's GetApplicationManifests should just return the result
				// The real client implementation handles the GetApplication internally
				m.EXPECT().GetApplicationManifests(gomock.Any(), "test-app", "").Return(mockManifestResponse, nil)
			},
			wantError: false,
		},
		{
			name:     "successful operation with revision",
			appName:  "test-app",
			revision: "v1.0.0",
			setupMock: func(m *mock.MockInterface) {
				// The mock interface's GetApplicationManifests should just return the result
				// The real client implementation handles the GetApplication internally
				m.EXPECT().GetApplicationManifests(gomock.Any(), "test-app", "v1.0.0").Return(mockManifestResponse, nil)
			},
			wantError: false,
		},
		{
			name:     "application not found",
			appName:  "nonexistent-app",
			revision: "",
			setupMock: func(m *mock.MockInterface) {
				// The mock returns an error when manifests are requested
				m.EXPECT().GetApplicationManifests(gomock.Any(), "nonexistent-app", "").Return(nil, assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to get application manifests",
		},
		{
			name:     "empty application name",
			appName:  "",
			revision: "",
			setupMock: func(m *mock.MockInterface) {
				// No mock expectations as it should fail before calling the client
			},
			wantError:   true,
			wantMessage: "Application name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			result, err := getApplicationManifestsHandler(context.Background(), mockClient, tt.appName, tt.revision)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.wantMessage != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := result.Content[0].(mcp.TextContent)
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.wantMessage)
				}
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)
				require.Len(t, result.Content, 1)

				// Verify the response is valid JSON
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok)
				var responseData map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &responseData)
				assert.NoError(t, err, "Response should be valid JSON")

				// Verify response contains expected fields
				assert.Contains(t, responseData, "manifests")
				assert.Contains(t, responseData, "namespace")
			}
		})
	}
}
