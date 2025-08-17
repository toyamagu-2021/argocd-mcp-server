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

func TestHandleListProjects(t *testing.T) {
	tests := []struct {
		name          string
		envVars       map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "missing environment variables",
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "",
				"ARGOCD_SERVER":     "",
			},
			wantError:     true,
			errorContains: "server address is required",
		},
		{
			name: "valid configuration",
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "failed to list projects",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleListProjects(context.Background(), mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "list_project",
					Arguments: map[string]interface{}{},
				},
			})

			// Check error expectation
			if tt.wantError {
				if err != nil {
					t.Errorf("HandleListProjects() returned error = %v, but error was not expected in result", err)
				}
				if result == nil {
					t.Fatal("HandleListProjects() returned nil result")
				}
				// Check if error is in the result
				if !result.IsError {
					t.Errorf("HandleListProjects() expected error result, but got success")
				}
				// Check error content is present (we know it's an error result)
				if tt.errorContains != "" && len(result.Content) == 0 {
					t.Errorf("HandleListProjects() expected error content, but got empty")
				}
			} else {
				if err != nil {
					t.Errorf("HandleListProjects() error = %v, wantError %v", err, tt.wantError)
				}
			}
		})
	}
}

func TestListProjectsTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if ListProjectsTool.Name != "list_project" {
		t.Errorf("Expected tool name 'list_project', got %s", ListProjectsTool.Name)
	}

	// Verify tool has description
	if ListProjectsTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if ListProjectsTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", ListProjectsTool.InputSchema.Type)
	}
}

func TestListProjectsHandler(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*mock.MockInterface)
		wantError    bool
		wantMessage  string
		wantProjects int
	}{
		{
			name: "successful list projects",
			setupMock: func(m *mock.MockInterface) {
				projectList := &v1alpha1.AppProjectList{
					Items: []v1alpha1.AppProject{
						{
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
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "production",
								Namespace: "argocd",
							},
							Spec: v1alpha1.AppProjectSpec{
								Description: "Production project",
								SourceRepos: []string{"https://github.com/myorg/*"},
								Destinations: []v1alpha1.ApplicationDestination{
									{
										Server:    "https://prod-cluster.example.com",
										Namespace: "prod-*",
									},
								},
							},
						},
					},
				}
				m.EXPECT().ListProjects(gomock.Any()).Return(projectList, nil)
			},
			wantError:    false,
			wantProjects: 2,
		},
		{
			name: "empty project list",
			setupMock: func(m *mock.MockInterface) {
				projectList := &v1alpha1.AppProjectList{
					Items: []v1alpha1.AppProject{},
				}
				m.EXPECT().ListProjects(gomock.Any()).Return(projectList, nil)
			},
			wantError:   false,
			wantMessage: "No projects found.",
		},
		{
			name: "list projects fails",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().ListProjects(gomock.Any()).Return(nil, assert.AnError)
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

			result, err := listProjectsHandler(context.Background(), mockClient, false)

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
					assert.Contains(t, textContent.Text, tt.wantMessage)
				}

				if tt.wantProjects > 0 {
					// Parse the JSON response
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)

					var projects []v1alpha1.AppProject
					err := json.Unmarshal([]byte(textContent.Text), &projects)
					require.NoError(t, err)
					assert.Len(t, projects, tt.wantProjects)
				}
			}
		})
	}
}

func TestListProjectsHandler_NameOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	projectList := &v1alpha1.AppProjectList{
		Items: []v1alpha1.AppProject{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "argocd",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "production",
					Namespace: "argocd",
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "staging",
					Namespace: "argocd",
				},
			},
		},
	}

	mockClient.EXPECT().ListProjects(gomock.Any()).Return(projectList, nil)

	result, err := listProjectsHandler(context.Background(), mockClient, true)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var nameList ProjectNameList
	err = json.Unmarshal([]byte(textContent.Text), &nameList)
	require.NoError(t, err)

	assert.Equal(t, 3, nameList.Count)
	assert.Equal(t, []string{"default", "production", "staging"}, nameList.Names)
}
