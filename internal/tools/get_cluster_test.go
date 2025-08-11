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
)

func TestHandleGetCluster(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		envVars       map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "missing server parameter",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_cluster",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "argocd.example.com:443",
			},
			wantError:     true,
			errorContains: "server is required",
		},
		{
			name: "missing environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_cluster",
					Arguments: map[string]interface{}{
						"server": "https://kubernetes.default.svc",
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
			name: "valid request",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "get_cluster",
					Arguments: map[string]interface{}{
						"server": "https://kubernetes.default.svc",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "argocd.example.com:443",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			result, err := HandleGetCluster(context.Background(), tt.request)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" && len(result.Content) > 0 {
					content, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, content.Text, tt.errorContains)
				}
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

func TestGetClusterTool_Schema(t *testing.T) {
	if GetClusterTool.Name != "get_cluster" {
		t.Errorf("Expected tool name 'get_cluster', got %s", GetClusterTool.Name)
	}

	if GetClusterTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	if GetClusterTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetClusterTool.InputSchema.Type)
	}

	// Check required parameter
	_, serverExists := GetClusterTool.InputSchema.Properties["server"]
	if !serverExists {
		t.Error("server parameter should exist in schema")
	}

	// Check required array contains server
	requiredFound := false
	for _, req := range GetClusterTool.InputSchema.Required {
		if req == "server" {
			requiredFound = true
			break
		}
	}
	if !requiredFound {
		t.Error("server should be in required parameters")
	}
}

func TestGetClusterHandler(t *testing.T) {
	tests := []struct {
		name        string
		server      string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:   "successful get",
			server: "https://kubernetes.default.svc",
			setupMock: func(m *mock.MockInterface) {
				expectedCluster := &v1alpha1.Cluster{
					Server: "https://kubernetes.default.svc",
					Name:   "in-cluster",
					Config: v1alpha1.ClusterConfig{
						TLSClientConfig: v1alpha1.TLSClientConfig{
							Insecure: false,
						},
					},
					ServerVersion: "1.28",
				}
				m.EXPECT().GetCluster(gomock.Any(), "https://kubernetes.default.svc").Return(expectedCluster, nil)
			},
			wantError:   false,
			wantMessage: "in-cluster",
		},
		{
			name:   "cluster not found",
			server: "https://non-existent.example.com",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetCluster(gomock.Any(), "https://non-existent.example.com").Return(nil, assert.AnError)
			},
			wantError: true,
		},
		{
			name:   "get fails",
			server: "https://kubernetes.default.svc",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetCluster(gomock.Any(), "https://kubernetes.default.svc").Return(nil, assert.AnError)
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

			result, err := getClusterHandler(context.Background(), mockClient, tt.server)

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
			}
		})
	}
}
