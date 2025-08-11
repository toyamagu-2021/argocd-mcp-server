package tools

import (
	"context"
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
func TestHandleCreateProject(t *testing.T) {
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
					Name: "create_project",
					Arguments: map[string]interface{}{
						"name": "test-project",
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
			name: "missing project name",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "create_project",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server:443",
			},
			wantError:     true,
			errorContains: "Project name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleCreateProject(context.Background(), tt.request)

			// Check expectations
			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" && len(result.Content) > 0 {
					textContent, ok := mcp.AsTextContent(result.Content[0])
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
func TestCreateProjectTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if CreateProjectTool.Name != "create_project" {
		t.Errorf("Expected tool name 'create_project', got %s", CreateProjectTool.Name)
	}

	// Verify tool has description
	if CreateProjectTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if CreateProjectTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", CreateProjectTool.InputSchema.Type)
	}

	// Check required parameters
	required := CreateProjectTool.InputSchema.Required
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("Expected required parameter 'name', got %v", required)
	}
}

// Test the handler logic with mocked client
func TestCreateProjectHandler(t *testing.T) {
	tests := []struct {
		name                string
		project             *v1alpha1.AppProject
		upsert              bool
		setupMock           func(*mock.MockInterface)
		wantError           bool
		wantMessage         string
		wantProjectInResult bool
	}{
		{
			name: "successful project creation",
			project: &v1alpha1.AppProject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "argoproj.io/v1alpha1",
					Kind:       "AppProject",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-project",
				},
				Spec: v1alpha1.AppProjectSpec{
					Description: "Test project",
					SourceRepos: []string{"*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "https://kubernetes.default.svc",
							Namespace: "*",
						},
					},
				},
			},
			upsert: false,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().CreateProject(gomock.Any(), gomock.Any(), false).Return(&v1alpha1.AppProject{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project",
					},
					Spec: v1alpha1.AppProjectSpec{
						Description: "Test project",
						SourceRepos: []string{"*"},
						Destinations: []v1alpha1.ApplicationDestination{
							{
								Server:    "https://kubernetes.default.svc",
								Namespace: "*",
							},
						},
					},
				}, nil)
			},
			wantError:           false,
			wantProjectInResult: true,
		},
		{
			name: "successful project creation with upsert",
			project: &v1alpha1.AppProject{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "argoproj.io/v1alpha1",
					Kind:       "AppProject",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-project",
				},
				Spec: v1alpha1.AppProjectSpec{
					Description: "Updated project",
					SourceRepos: []string{"https://github.com/example/*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "https://kubernetes.default.svc",
							Namespace: "prod",
						},
					},
				},
			},
			upsert: true,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().CreateProject(gomock.Any(), gomock.Any(), true).Return(&v1alpha1.AppProject{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-project",
					},
					Spec: v1alpha1.AppProjectSpec{
						Description: "Updated project",
						SourceRepos: []string{"https://github.com/example/*"},
						Destinations: []v1alpha1.ApplicationDestination{
							{
								Server:    "https://kubernetes.default.svc",
								Namespace: "prod",
							},
						},
					},
				}, nil)
			},
			wantError:           false,
			wantProjectInResult: true,
		},
		{
			name: "project creation fails",
			project: &v1alpha1.AppProject{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-project",
				},
				Spec: v1alpha1.AppProjectSpec{
					SourceRepos: []string{"*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "https://kubernetes.default.svc",
							Namespace: "*",
						},
					},
				},
			},
			upsert: false,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().CreateProject(gomock.Any(), gomock.Any(), false).Return(nil, assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to create project",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := createProjectHandler(context.Background(), mockClient, tt.project, tt.upsert)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.wantMessage != "" && len(result.Content) > 0 {
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.wantMessage)
				}
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				if tt.wantProjectInResult {
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, "test-project")
				}
			}
		})
	}
}

// Test helper functions
func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single value",
			input:    "value1",
			expected: []string{"value1"},
		},
		{
			name:     "multiple values",
			input:    "value1,value2,value3",
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "values with spaces",
			input:    " value1 , value2 , value3 ",
			expected: []string{"value1", "value2", "value3"},
		},
		{
			name:     "values with empty parts",
			input:    "value1,,value3",
			expected: []string{"value1", "value3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCommaSeparated(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseGroupKinds(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []metav1.GroupKind
	}{
		{
			name:     "empty string",
			input:    "",
			expected: []metav1.GroupKind{},
		},
		{
			name:  "single group:kind",
			input: "apps:Deployment",
			expected: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
			},
		},
		{
			name:  "multiple group:kind",
			input: "apps:Deployment,batch:Job,:Service",
			expected: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
				{Group: "batch", Kind: "Job"},
				{Group: "", Kind: "Service"},
			},
		},
		{
			name:  "core resources (no group)",
			input: "Service,ConfigMap",
			expected: []metav1.GroupKind{
				{Group: "", Kind: "Service"},
				{Group: "", Kind: "ConfigMap"},
			},
		},
		{
			name:  "values with spaces",
			input: " apps:Deployment , batch:Job , :Service ",
			expected: []metav1.GroupKind{
				{Group: "apps", Kind: "Deployment"},
				{Group: "batch", Kind: "Job"},
				{Group: "", Kind: "Service"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseGroupKinds(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
