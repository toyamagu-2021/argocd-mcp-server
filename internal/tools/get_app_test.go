package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestGetApplicationHandler(t *testing.T) {
	tests := []struct {
		name        string
		appName     string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
		checkApp    func(*testing.T, *v1alpha1.Application)
	}{
		{
			name:    "successful get application",
			appName: "test-app",
			setupMock: func(m *mock.MockInterface) {
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app",
						Namespace: "argocd",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/test/repo",
							TargetRevision: "HEAD",
							Path:           "manifests",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: "Synced",
						},
						Health: v1alpha1.HealthStatus{
							Status: "Healthy",
						},
					},
				}
				m.EXPECT().GetApplication(gomock.Any(), "test-app").Return(app, nil)
			},
			wantError: false,
			checkApp: func(t *testing.T, app *v1alpha1.Application) {
				assert.Equal(t, "test-app", app.Name)
				assert.Equal(t, "default", app.Spec.Project)
				assert.Equal(t, "https://github.com/test/repo", app.Spec.Source.RepoURL)
				assert.Equal(t, v1alpha1.SyncStatusCodeSynced, app.Status.Sync.Status)
				assert.Equal(t, health.HealthStatusCode("Healthy"), app.Status.Health.Status)
			},
		},
		{
			name:        "empty application name",
			appName:     "",
			setupMock:   func(m *mock.MockInterface) {},
			wantError:   false,
			wantMessage: "Application name is required",
		},
		{
			name:    "application not found",
			appName: "nonexistent-app",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplication(gomock.Any(), "nonexistent-app").Return(nil, assert.AnError)
			},
			wantError:   false,
			wantMessage: "Failed to get application",
		},
		{
			name:    "get application with multi-source",
			appName: "multi-source-app",
			setupMock: func(m *mock.MockInterface) {
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-source-app",
						Namespace: "argocd",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "production",
						Sources: []v1alpha1.ApplicationSource{
							{
								RepoURL:        "https://github.com/test/repo1",
								TargetRevision: "v1.0.0",
								Path:           "app",
							},
							{
								RepoURL:        "https://github.com/test/repo2",
								TargetRevision: "main",
								Path:           "config",
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://prod-cluster.example.com",
							Namespace: "production",
						},
					},
				}
				m.EXPECT().GetApplication(gomock.Any(), "multi-source-app").Return(app, nil)
			},
			wantError: false,
			checkApp: func(t *testing.T, app *v1alpha1.Application) {
				assert.Equal(t, "multi-source-app", app.Name)
				assert.Equal(t, "production", app.Spec.Project)
				assert.Len(t, app.Spec.Sources, 2)
				assert.Equal(t, "https://github.com/test/repo1", app.Spec.Sources[0].RepoURL)
				assert.Equal(t, "v1.0.0", app.Spec.Sources[0].TargetRevision)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			ctx := context.Background()
			result, err := getApplicationHandler(ctx, mockClient, tt.appName)

			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check the result content
			require.Len(t, result.Content, 1)
			textContent, ok := mcp.AsTextContent(result.Content[0])
			require.True(t, ok)

			if tt.wantMessage != "" {
				// Expecting an error message
				assert.Contains(t, textContent.Text, tt.wantMessage)
			} else if tt.checkApp != nil {
				// Expecting JSON with application details
				var app v1alpha1.Application
				err := json.Unmarshal([]byte(textContent.Text), &app)
				require.NoError(t, err)
				tt.checkApp(t, &app)
			}
		})
	}
}
