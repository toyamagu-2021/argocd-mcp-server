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

func TestHandleGetRepository(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		envVars       map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "missing repository URL",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_repository",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "argocd.example.com:443",
			},
			wantError:     true,
			errorContains: "Repository URL is required",
		},
		{
			name: "missing environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_repository",
					Arguments: map[string]interface{}{
						"repo": "https://github.com/example/repo.git",
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
			name: "missing auth token",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_repository",
					Arguments: map[string]interface{}{
						"repo": "https://github.com/example/repo.git",
					},
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
			result, err := HandleGetRepository(context.Background(), tt.request)

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

func TestGetRepositoryTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetRepositoryTool.Name != "get_repository" {
		t.Errorf("Expected tool name 'get_repository', got %s", GetRepositoryTool.Name)
	}

	// Verify tool has description
	if GetRepositoryTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetRepositoryTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetRepositoryTool.InputSchema.Type)
	}

	// Check required parameters
	requiredParams := GetRepositoryTool.InputSchema.Required
	if len(requiredParams) != 1 || requiredParams[0] != "repo" {
		t.Errorf("Expected required parameter 'repo', got %v", requiredParams)
	}

	// Check repo parameter exists and is a string
	repoParam, exists := GetRepositoryTool.InputSchema.Properties["repo"]
	if !exists {
		t.Error("Expected 'repo' parameter to exist in schema")
	} else {
		paramMap, ok := repoParam.(map[string]interface{})
		if !ok {
			t.Error("Expected repo parameter to be a map")
		} else if paramMap["type"] != "string" {
			t.Errorf("Expected repo parameter type to be 'string', got %v", paramMap["type"])
		}
	}
}

func TestGetRepositoryHandler(t *testing.T) {
	tests := []struct {
		name        string
		repo        string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name: "successful get repository",
			repo: "https://github.com/example/repo.git",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetRepository(gomock.Any(), "https://github.com/example/repo.git").Return(&v1alpha1.Repository{
					Repo:     "https://github.com/example/repo.git",
					Type:     "git",
					Username: "user",
					Name:     "example-repo",
				}, nil)
			},
			wantError:   false,
			wantMessage: "https://github.com/example/repo.git",
		},
		{
			name: "get repository fails",
			repo: "https://github.com/example/nonexistent.git",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetRepository(gomock.Any(), "https://github.com/example/nonexistent.git").Return(nil, assert.AnError)
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

			result, err := getRepositoryHandler(context.Background(), mockClient, tt.repo)

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

					// Verify it's valid JSON
					var repo v1alpha1.Repository
					err := json.Unmarshal([]byte(textContent.Text), &repo)
					assert.NoError(t, err, "Response should be valid JSON")
				}
			}
		})
	}
}
