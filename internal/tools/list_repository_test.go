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
)

func TestListRepositoryTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if ListRepositoryTool.Name != "list_repository" {
		t.Errorf("Expected tool name 'list_repository', got %s", ListRepositoryTool.Name)
	}

	// Verify tool has description
	assert.Contains(t, ListRepositoryTool.Description, "detailed=false")
	assert.Contains(t, ListRepositoryTool.Description, "50")

	// Check input schema exists
	if ListRepositoryTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", ListRepositoryTool.InputSchema.Type)
	}
}

func TestListRepositoryHandler(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name: "successful list with repositories",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().ListRepositories(gomock.Any()).Return(&v1alpha1.RepositoryList{
					Items: v1alpha1.Repositories{
						{
							Repo: "https://github.com/example/repo1.git",
							Type: "git",
						},
						{
							Repo: "https://github.com/example/repo2.git",
							Type: "git",
						},
					},
				}, nil)
			},
			wantError:   false,
			wantMessage: "https://github.com/example/repo1.git",
		},
		{
			name: "empty repository list",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().ListRepositories(gomock.Any()).Return(&v1alpha1.RepositoryList{
					Items: v1alpha1.Repositories{},
				}, nil)
			},
			wantError:   false,
			wantMessage: `"totalItems":0`,
		},
		{
			name: "list repositories fails",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().ListRepositories(gomock.Any()).Return(nil, assert.AnError)
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

			result, err := listRepositoryHandler(context.Background(), mockClient, false, 50, 0)

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

					var page struct {
						Items v1alpha1.Repositories `json:"items"`
					}
					err := json.Unmarshal([]byte(textContent.Text), &page)
					assert.NoError(t, err, "Response should be valid JSON")
				}
			}
		})
	}
}

func TestListRepositorySummarySize(t *testing.T) {
	secret := "password-marker:" + strings.Repeat("p", 2048)
	items := make(v1alpha1.Repositories, 40)
	for i := range items {
		items[i] = &v1alpha1.Repository{
			Repo:            "https://example.com/repo-" + itoaSmall(i) + ".git",
			Type:            "git",
			Name:            "repo",
			Project:         "default",
			Username:        "user-secret",
			Password:        secret,
			ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful},
		}
	}
	before, err := json.MarshalIndent(items, "", "  ")
	require.NoError(t, err)
	list := &v1alpha1.RepositoryList{Items: items}

	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().ListRepositories(gomock.Any()).Return(list, nil).Times(2)

	summaryRes, err := listRepositoryHandler(context.Background(), m, false, 50, 0)
	require.NoError(t, err)
	summary := toolText(t, summaryRes)
	assert.NotContains(t, summary, "user-secret")
	assert.NotContains(t, summary, "password-marker")
	assert.Contains(t, summary, `"connectionStatus"`)
	assert.Contains(t, summary, `"totalItems":40`)
	secretBytes := strings.Count(string(before), secret) * len(secret)
	t.Logf("before_bytes=%d after_bytes=%d password_bytes=%d", len(before), len(summary), secretBytes)
	assert.Less(t, len(summary), len(before)/5)
	assert.Greater(t, secretBytes, len(summary))

	detailedRes, err := listRepositoryHandler(context.Background(), m, true, 50, 0)
	require.NoError(t, err)
	detailed := toolText(t, detailedRes)
	assert.NotContains(t, detailed, "password-marker")
	assert.Contains(t, detailed, "https://example.com/repo-0.git")
}
