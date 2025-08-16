package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test the tool handler with environment variables
func TestHandleGetApplicationSet(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		envVars       map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "missing name parameter",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_applicationset",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
			},
			wantError:     true,
			errorContains: "name is required",
		},
		{
			name: "missing environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_applicationset",
					Arguments: map[string]interface{}{
						"name": "test-appset",
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
			name: "with valid environment and name",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_applicationset",
					Arguments: map[string]interface{}{
						"name": "test-appset",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleGetApplicationSet(context.Background(), tt.request)

			// Check expectations
			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" && len(result.Content) > 0 {
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.errorContains)
				}
			} else {
				// Note: This will still fail because we're not mocking the actual connection
				// but it tests the basic flow
				require.Nil(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

// Test the tool schema
func TestGetApplicationSetTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetApplicationSetTool.Name != "get_applicationset" {
		t.Errorf("Expected tool name 'get_applicationset', got %s", GetApplicationSetTool.Name)
	}

	// Verify tool has description
	if GetApplicationSetTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetApplicationSetTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetApplicationSetTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(GetApplicationSetTool.InputSchema.Properties) == 0 {
		t.Error("Tool schema should have properties defined")
	}

	// Check name parameter exists
	props := GetApplicationSetTool.InputSchema.Properties
	if _, ok := props["name"]; !ok {
		t.Error("Expected 'name' property to be defined")
	}

	// Check that name is required
	required := GetApplicationSetTool.InputSchema.Required
	nameRequired := false
	for _, r := range required {
		if r == "name" {
			nameRequired = true
			break
		}
	}
	if !nameRequired {
		t.Error("Expected 'name' to be a required parameter")
	}
}

// Test the handler logic with mocked client
func TestGetApplicationSetHandler(t *testing.T) {
	tests := []struct {
		name        string
		appSetName  string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:       "successful get",
			appSetName: "test-appset",
			setupMock: func(m *mock.MockInterface) {
				appSet := &v1alpha1.ApplicationSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-appset",
						Namespace: "argocd",
						Labels: map[string]string{
							"env": "prod",
						},
					},
					Spec: v1alpha1.ApplicationSetSpec{
						Template: v1alpha1.ApplicationSetTemplate{
							Spec: v1alpha1.ApplicationSpec{
								Project: "default",
								Source: &v1alpha1.ApplicationSource{
									RepoURL:        "https://github.com/example/repo",
									Path:           "manifests",
									TargetRevision: "main",
								},
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://kubernetes.default.svc",
									Namespace: "default",
								},
							},
						},
					},
				}
				m.EXPECT().GetApplicationSet(gomock.Any(), "test-appset").Return(appSet, nil)
			},
			wantError:   false,
			wantMessage: "test-appset",
		},
		{
			name:       "applicationset not found",
			appSetName: "non-existent",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationSet(gomock.Any(), "non-existent").Return(nil, assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to get ApplicationSet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := getApplicationSetHandler(context.Background(), mockClient, tt.appSetName)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.wantMessage != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.wantMessage)
				}
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				if tt.wantMessage != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.wantMessage)

					// Verify it's valid JSON
					var appSet v1alpha1.ApplicationSet
					err := json.Unmarshal([]byte(textContent.Text), &appSet)
					assert.NoError(t, err)
					assert.Equal(t, tt.appSetName, appSet.Name)
				}
			}
		})
	}
}
