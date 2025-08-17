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
)

func TestHandleListRepository(t *testing.T) {
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
					Name:      "list_repository",
					Arguments: map[string]interface{}{},
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
			name: "missing auth token",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "list_repository",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "",
				"ARGOCD_SERVER":     "argocd.example.com:443",
			},
			wantError:     true,
			errorContains: "auth token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleListRepository(context.Background(), tt.request)

			// Check expectations
			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" && len(result.Content) > 0 {
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok, "expected text content")
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

func TestListRepositoryTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if ListRepositoryTool.Name != "list_repository" {
		t.Errorf("Expected tool name 'list_repository', got %s", ListRepositoryTool.Name)
	}

	// Verify tool has description
	if ListRepositoryTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

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
			wantMessage: "No repositories found.",
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

			result, err := listRepositoryHandler(context.Background(), mockClient)

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

					// If it's not the "No repositories found" message, verify it's valid JSON
					if tt.wantMessage != "No repositories found." {
						var repos v1alpha1.Repositories
						err := json.Unmarshal([]byte(textContent.Text), &repos)
						assert.NoError(t, err, "Response should be valid JSON")
					}
				}
			}
		})
	}
}
