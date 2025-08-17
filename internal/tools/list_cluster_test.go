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

func TestHandleListCluster(t *testing.T) {
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
					Name:      "list_cluster",
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
			name: "valid environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "list_cluster",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "argocd.example.com:443",
			},
			wantError: false,
		},
		{
			name: "with detailed=true",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "list_cluster",
					Arguments: map[string]interface{}{"detailed": true},
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

			result, err := HandleListCluster(context.Background(), tt.request)

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

func TestListClusterTool_Schema(t *testing.T) {
	if ListClusterTool.Name != "list_cluster" {
		t.Errorf("Expected tool name 'list_cluster', got %s", ListClusterTool.Name)
	}

	if ListClusterTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	if ListClusterTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", ListClusterTool.InputSchema.Type)
	}
}

func TestListClusterHandler(t *testing.T) {
	tests := []struct {
		name        string
		detailed    bool
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:     "successful list - summary",
			detailed: false,
			setupMock: func(m *mock.MockInterface) {
				expectedClusters := &v1alpha1.ClusterList{
					Items: []v1alpha1.Cluster{
						{
							Server: "https://kubernetes.default.svc",
							Name:   "in-cluster",
							ConnectionState: v1alpha1.ConnectionState{
								Status: v1alpha1.ConnectionStatusSuccessful,
							},
						},
						{
							Server: "https://external-cluster.example.com",
							Name:   "external-cluster",
							ConnectionState: v1alpha1.ConnectionState{
								Status: v1alpha1.ConnectionStatusFailed,
							},
						},
					},
				}
				m.EXPECT().ListClusters(gomock.Any()).Return(expectedClusters, nil)
			},
			wantError:   false,
			wantMessage: "in-cluster",
		},
		{
			name:     "successful list - detailed",
			detailed: true,
			setupMock: func(m *mock.MockInterface) {
				expectedClusters := &v1alpha1.ClusterList{
					Items: []v1alpha1.Cluster{
						{
							Server: "https://kubernetes.default.svc",
							Name:   "in-cluster",
							Config: v1alpha1.ClusterConfig{
								TLSClientConfig: v1alpha1.TLSClientConfig{
									Insecure: false,
								},
							},
						},
					},
				}
				m.EXPECT().ListClusters(gomock.Any()).Return(expectedClusters, nil)
			},
			wantError:   false,
			wantMessage: "in-cluster",
		},
		{
			name:     "empty cluster list",
			detailed: false,
			setupMock: func(m *mock.MockInterface) {
				expectedClusters := &v1alpha1.ClusterList{
					Items: []v1alpha1.Cluster{},
				}
				m.EXPECT().ListClusters(gomock.Any()).Return(expectedClusters, nil)
			},
			wantError:   false,
			wantMessage: "No clusters found",
		},
		{
			name:     "list fails",
			detailed: false,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().ListClusters(gomock.Any()).Return(nil, assert.AnError)
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

			result, err := listClusterHandler(context.Background(), mockClient, tt.detailed, false)

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

func TestListClusterHandler_NameOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	clusterList := &v1alpha1.ClusterList{
		Items: []v1alpha1.Cluster{
			{
				Name:   "in-cluster",
				Server: "https://kubernetes.default.svc",
			},
			{
				Name:   "prod-cluster",
				Server: "https://prod.example.com",
			},
			{
				Name:   "dev-cluster",
				Server: "https://dev.example.com",
			},
		},
	}

	mockClient.EXPECT().ListClusters(gomock.Any()).Return(clusterList, nil)

	result, err := listClusterHandler(context.Background(), mockClient, false, true)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	// Since we don't import ClusterNameList in tests, we'll check the JSON structure
	var response map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(3), response["count"])
	clusters, ok := response["clusters"].([]interface{})
	require.True(t, ok)
	assert.Len(t, clusters, 3)

	// Check first cluster
	firstCluster, ok := clusters[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "in-cluster", firstCluster["name"])
	assert.Equal(t, "https://kubernetes.default.svc", firstCluster["server"])
}
