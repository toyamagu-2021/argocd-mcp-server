package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestListProjectsTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if ListProjectsTool.Name != "list_project" {
		t.Errorf("Expected tool name 'list_project', got %s", ListProjectsTool.Name)
	}

	// Verify tool has description
	assert.Contains(t, ListProjectsTool.Description, "detailed=false")
	assert.Contains(t, ListProjectsTool.Description, "50")

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
			wantMessage: `"totalItems":0`,
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

			result, err := listProjectsHandler(context.Background(), mockClient, false, false, 50, 0)

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

					var page struct {
						Items []v1alpha1.AppProject `json:"items"`
					}
					err := json.Unmarshal([]byte(textContent.Text), &page)
					projects := page.Items
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

	result, err := listProjectsHandler(context.Background(), mockClient, true, true, 50, 0)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var namePage struct {
		Items      []string `json:"items"`
		TotalItems int      `json:"totalItems"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &namePage)
	require.NoError(t, err)

	assert.Equal(t, 3, namePage.TotalItems)
	assert.Equal(t, []string{"default", "production", "staging"}, namePage.Items)
	assert.NotContains(t, textContent.Text, `"roles"`)
}

func TestListProjectsSummarySize(t *testing.T) {
	token := "jwt-marker:" + strings.Repeat("j", 2048)
	items := make([]v1alpha1.AppProject, 20)
	for i := range items {
		items[i] = v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:          "project-" + itoaSmall(i),
				ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
				Annotations:   map[string]string{lastAppliedAnnotation: "applied"},
			},
			Spec: v1alpha1.AppProjectSpec{
				Description: "desc",
				SourceRepos: []string{"https://example.com/repo"},
				Destinations: []v1alpha1.ApplicationDestination{{
					Server: "https://kubernetes.default.svc", Namespace: "default",
				}},
				Roles: []v1alpha1.ProjectRole{{
					Name:      "admin",
					JWTTokens: []v1alpha1.JWTToken{{ID: token}},
				}},
			},
		}
	}
	before, err := json.MarshalIndent(items, "", "  ")
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().ListProjects(gomock.Any()).Return(&v1alpha1.AppProjectList{Items: items}, nil).Times(2)

	summary := toolText(t, mustListProjects(t, m, false, false))
	assert.NotContains(t, summary, "jwt-marker")
	assert.NotContains(t, summary, `"roles"`)
	assert.Contains(t, summary, `"roleCount":1`)
	assert.Contains(t, summary, `"totalItems":20`)
	tokenBytes := strings.Count(string(before), token) * len(token)
	t.Logf("before_bytes=%d after_bytes=%d jwt_bytes=%d", len(before), len(summary), tokenBytes)
	assert.Less(t, len(summary), len(before)/5)

	detailed := toolText(t, mustListProjects(t, m, false, true))
	assert.Contains(t, detailed, "jwt-marker")
	assert.Contains(t, detailed, `"roles"`)
	assert.NotContains(t, detailed, "managedFields")
	assert.NotContains(t, detailed, lastAppliedAnnotation)
	assert.Less(t, len(detailed), len(before))
}

func mustListProjects(t *testing.T, m *mock.MockInterface, nameOnly, detailed bool) *mcp.CallToolResult {
	t.Helper()
	res, err := listProjectsHandler(context.Background(), m, nameOnly, detailed, 50, 0)
	require.NoError(t, err)
	return res
}
