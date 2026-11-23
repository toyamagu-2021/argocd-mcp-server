package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetProjectTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetProjectTool.Name != "get_project" {
		t.Errorf("Expected tool name 'get_project', got %s", GetProjectTool.Name)
	}

	assert.Contains(t, GetProjectTool.Description, "detailed=false")

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
						ClusterResourceWhitelist: []v1alpha1.ClusterResourceRestrictionItem{
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
					ClusterResourceWhitelist: []v1alpha1.ClusterResourceRestrictionItem{
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

			result, err := getProjectHandler(context.Background(), mockClient, tt.projectName, false)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				text := toolText(t, result)
				assert.NotContains(t, text, "totalItems")
				assert.NotContains(t, text, `"roles"`)
				assert.NotContains(t, text, "metadata")

				var summary projectSummary
				err := json.Unmarshal([]byte(text), &summary)
				require.NoError(t, err)
				assert.Equal(t, tt.wantProject.Name, summary.Name)
				assert.Equal(t, tt.wantProject.Spec.Description, summary.Description)
				assert.Equal(t, len(tt.wantProject.Spec.SourceRepos), summary.SourceRepoCount)
				assert.Equal(t, len(tt.wantProject.Spec.Destinations), summary.DestinationCount)
				assert.Equal(t, len(tt.wantProject.Spec.Roles), summary.RoleCount)
			}
		})
	}
}

func TestGetProjectSummarySize(t *testing.T) {
	token := "jwt-marker:" + strings.Repeat("j", 2048)
	project := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "fat",
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
	before, err := json.MarshalIndent(project, "", "  ")
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetProject(gomock.Any(), "fat").Return(project, nil).Times(2)

	summaryRes, err := getProjectHandler(context.Background(), m, "fat", false)
	require.NoError(t, err)
	summary := toolText(t, summaryRes)
	assert.NotContains(t, summary, "jwt-marker")
	assert.NotContains(t, summary, `"roles"`)
	assert.NotContains(t, summary, "totalItems")
	assert.Contains(t, summary, `"roleCount":1`)
	assert.Contains(t, summary, `"name":"fat"`)
	tokenBytes := strings.Count(string(before), token) * len(token)
	t.Logf("before_bytes=%d after_bytes=%d jwt_bytes=%d", len(before), len(summary), tokenBytes)
	assert.Less(t, len(summary), len(before)/5)

	detailedRes, err := getProjectHandler(context.Background(), m, "fat", true)
	require.NoError(t, err)
	detailed := toolText(t, detailedRes)
	assert.Contains(t, detailed, "jwt-marker")
	assert.Contains(t, detailed, `"roles"`)
	assert.NotContains(t, detailed, "managedFields")
	assert.NotContains(t, detailed, lastAppliedAnnotation)
	assert.NotContains(t, detailed, "totalItems")
	assert.Less(t, len(detailed), len(before))
	var full v1alpha1.AppProject
	require.NoError(t, json.Unmarshal([]byte(detailed), &full))
	assert.Equal(t, "fat", full.Name)
	assert.Equal(t, token, full.Spec.Roles[0].JWTTokens[0].ID)
}
