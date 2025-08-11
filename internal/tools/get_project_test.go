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

func TestHandleGetProject(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		envVars       map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "missing project name",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_project",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true,
			errorContains: "Project name is required",
		},
		{
			name: "missing environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_project",
					Arguments: map[string]interface{}{
						"name": "default",
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
			name: "valid configuration",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_project",
					Arguments: map[string]interface{}{
						"name": "default",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "failed to get project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleGetProject(context.Background(), tt.request)

			// Check error expectation
			if tt.wantError {
				if err != nil {
					t.Errorf("HandleGetProject() returned error = %v, but error was not expected in result", err)
				}
				if result == nil {
					t.Fatal("HandleGetProject() returned nil result")
				}
				// Check if error is in the result
				if !result.IsError {
					t.Errorf("HandleGetProject() expected error result, but got success")
				}
				// Check error content is present (we know it's an error result)
				if tt.errorContains != "" && len(result.Content) == 0 {
					t.Errorf("HandleGetProject() expected error content, but got empty")
				}
			} else {
				if err != nil {
					t.Errorf("HandleGetProject() error = %v, wantError %v", err, tt.wantError)
				}
			}
		})
	}
}

func TestGetProjectTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetProjectTool.Name != "get_project" {
		t.Errorf("Expected tool name 'get_project', got %s", GetProjectTool.Name)
	}

	// Verify tool has description
	if GetProjectTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetProjectTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetProjectTool.InputSchema.Type)
	}

	// Check that name parameter is required
	found := false
	for _, req := range GetProjectTool.InputSchema.Required {
		if req == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Name parameter should be required")
	}
}

func TestGetProjectHandler(t *testing.T) {
	tests := []struct {
		name        string
		projectName string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantProject *v1alpha1.AppProject
	}{
		{
			name:        "successful get project",
			projectName: "default",
			setupMock: func(m *mock.MockInterface) {
				project := &v1alpha1.AppProject{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "default",
						Namespace: "argocd",
					},
					Spec: v1alpha1.AppProjectSpec{
						Description: "Default project",
						SourceRepos: []string{"*"},
						Destinations: []v1alpha1.ApplicationDestination{
							{
								Server:    "*",
								Namespace: "*",
							},
						},
						ClusterResourceWhitelist: []metav1.GroupKind{
							{
								Group: "*",
								Kind:  "*",
							},
						},
					},
				}
				m.EXPECT().GetProject(gomock.Any(), "default").Return(project, nil)
			},
			wantError: false,
			wantProject: &v1alpha1.AppProject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "argocd",
				},
				Spec: v1alpha1.AppProjectSpec{
					Description: "Default project",
					SourceRepos: []string{"*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "*",
							Namespace: "*",
						},
					},
					ClusterResourceWhitelist: []metav1.GroupKind{
						{
							Group: "*",
							Kind:  "*",
						},
					},
				},
			},
		},
		{
			name:        "get project fails",
			projectName: "nonexistent",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetProject(gomock.Any(), "nonexistent").Return(nil, assert.AnError)
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

			result, err := getProjectHandler(context.Background(), mockClient, tt.projectName)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				// Parse the JSON response
				require.Len(t, result.Content, 1)
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)

				var project v1alpha1.AppProject
				err := json.Unmarshal([]byte(textContent.Text), &project)
				require.NoError(t, err)
				assert.Equal(t, tt.wantProject.Name, project.Name)
				assert.Equal(t, tt.wantProject.Spec.Description, project.Spec.Description)
			}
		})
	}
}
