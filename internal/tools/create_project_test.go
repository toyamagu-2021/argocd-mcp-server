package tools

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test the tool schema
func TestCreateProjectTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if CreateProjectTool.Name != "create_project" {
		t.Errorf("Expected tool name 'create_project', got %s", CreateProjectTool.Name)
	}

	assert.Contains(t, CreateProjectTool.Description, "compact")
	assert.Contains(t, CreateProjectTool.Description, "managedFields")

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

func TestParseClusterResourceRestrictions(t *testing.T) {
	got := parseClusterResourceRestrictions(" rbac.authorization.k8s.io:ClusterRole , Namespace,,apps:Deployment ")
	assert.Equal(t, []v1alpha1.ClusterResourceRestrictionItem{
		{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
		{Group: "", Kind: "Namespace"},
		{Group: "apps", Kind: "Deployment"},
	}, got)
	assert.Empty(t, parseClusterResourceRestrictions(""))
}

func TestCreateProject_Defaults(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().CreateProject(gomock.Any(), gomock.Any(), false).
		DoAndReturn(func(_ interface{}, p *v1alpha1.AppProject, _ bool) (*v1alpha1.AppProject, error) {
			assert.Equal(t, "proj", p.Name)
			assert.Equal(t, []string{"*"}, p.Spec.SourceRepos)
			assert.Equal(t, []v1alpha1.ApplicationDestination{{Server: "https://kubernetes.default.svc", Namespace: "*"}}, p.Spec.Destinations)
			assert.Empty(t, p.Spec.ClusterResourceWhitelist)
			assert.Empty(t, p.Spec.NamespaceResourceWhitelist)
			assert.Empty(t, p.Spec.NamespaceResourceBlacklist)
			return p, nil
		})
	res := callTool(t, newTestServer(t, Deps{Client: m}), "create_project", map[string]interface{}{"name": "proj"})
	assert.False(t, res.IsError)
}

func TestCreateProject_AllArguments(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().CreateProject(gomock.Any(), gomock.Any(), true).
		DoAndReturn(func(_ interface{}, p *v1alpha1.AppProject, _ bool) (*v1alpha1.AppProject, error) {
			assert.Equal(t, "team project", p.Spec.Description)
			assert.Equal(t, []string{"https://github.com/a/x", "https://github.com/a/y"}, p.Spec.SourceRepos)
			assert.Equal(t, []v1alpha1.ApplicationDestination{{Server: "https://other.example", Namespace: "team-*"}}, p.Spec.Destinations)
			assert.Equal(t, []v1alpha1.ClusterResourceRestrictionItem{{Group: "", Kind: "Namespace"}}, p.Spec.ClusterResourceWhitelist)
			assert.Equal(t, []metav1.GroupKind{{Group: "apps", Kind: "Deployment"}}, p.Spec.NamespaceResourceWhitelist)
			assert.Equal(t, []metav1.GroupKind{{Group: "", Kind: "Secret"}}, p.Spec.NamespaceResourceBlacklist)
			return p, nil
		})
	res := callTool(t, newTestServer(t, Deps{Client: m}), "create_project", map[string]interface{}{
		"name":                         "proj",
		"description":                  "team project",
		"source_repos":                 "https://github.com/a/x, https://github.com/a/y",
		"destination_server":           "https://other.example",
		"destination_namespace":        "team-*",
		"cluster_resource_whitelist":   "Namespace",
		"namespace_resource_whitelist": "apps:Deployment",
		"namespace_resource_blacklist": "Secret",
		"upsert":                       true,
	})
	assert.False(t, res.IsError)
}
