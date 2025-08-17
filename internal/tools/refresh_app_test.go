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
func TestHandleRefreshApplication(t *testing.T) {
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
					Name: "refresh_application",
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
			name: "missing application name",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "refresh_application",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
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
			result, err := HandleRefreshApplication(context.Background(), tt.request)

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
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)
			}
		})
	}
}

// Test the tool schema
func TestRefreshAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if RefreshAppTool.Name != "refresh_application" {
		t.Errorf("Expected tool name 'refresh_application', got %s", RefreshAppTool.Name)
	}

	// Verify tool has description
	if RefreshAppTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if RefreshAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", RefreshAppTool.InputSchema.Type)
	}

	// Check required parameters
	requiredParams := RefreshAppTool.InputSchema.Required
	if len(requiredParams) != 1 || requiredParams[0] != "name" {
		t.Errorf("Expected required parameter 'name', got %v", requiredParams)
	}

	// Check optional parameters
	properties := RefreshAppTool.InputSchema.Properties
	if _, ok := properties["hard"]; !ok {
		t.Error("Expected 'hard' parameter in schema properties")
	}
}

// Test the handler logic with mocked client
func TestRefreshApplicationHandler(t *testing.T) {
	tests := []struct {
		name        string
		appName     string
		hardRefresh bool
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:        "successful normal refresh",
			appName:     "test-app",
			hardRefresh: false,
			setupMock: func(m *mock.MockInterface) {
				expectedApp := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-app",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: "Synced",
						},
					},
				}
				m.EXPECT().RefreshApplication(gomock.Any(), "test-app", "normal").Return(expectedApp, nil)
			},
			wantError:   false,
			wantMessage: "test-app",
		},
		{
			name:        "successful hard refresh",
			appName:     "test-app",
			hardRefresh: true,
			setupMock: func(m *mock.MockInterface) {
				expectedApp := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-app",
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: "Synced",
						},
					},
				}
				m.EXPECT().RefreshApplication(gomock.Any(), "test-app", "hard").Return(expectedApp, nil)
			},
			wantError:   false,
			wantMessage: "test-app",
		},
		{
			name:        "refresh fails",
			appName:     "test-app",
			hardRefresh: false,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().RefreshApplication(gomock.Any(), "test-app", "normal").Return(nil, assert.AnError)
			},
			wantError: true,
		},
		{
			name:        "empty application name",
			appName:     "",
			hardRefresh: false,
			setupMock:   func(m *mock.MockInterface) {},
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := refreshApplicationHandler(context.Background(), mockClient, tt.appName, tt.hardRefresh)

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

					// Verify the response contains expected application name
					var app v1alpha1.Application
					err := json.Unmarshal([]byte(textContent.Text), &app)
					require.NoError(t, err)
					assert.Equal(t, tt.wantMessage, app.Name)
				}
			}
		})
	}
}
