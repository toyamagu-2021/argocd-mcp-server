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

func TestListClusterTool_Schema(t *testing.T) {
	if ListClusterTool.Name != "list_cluster" {
		t.Errorf("Expected tool name 'list_cluster', got %s", ListClusterTool.Name)
	}

	assert.Contains(t, ListClusterTool.Description, "detailed=false")
	assert.Contains(t, ListClusterTool.Description, "50")

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
			wantMessage: `"totalItems":0`,
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

			result, err := listClusterHandler(context.Background(), mockClient, tt.detailed, false, 50, 0)

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

	result, err := listClusterHandler(context.Background(), mockClient, false, true, 50, 0)
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	// Since we don't import ClusterNameList in tests, we'll check the JSON structure
	var response map[string]interface{}
	err = json.Unmarshal([]byte(textContent.Text), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(3), response["totalItems"])
	clusters, ok := response["items"].([]interface{})
	require.True(t, ok)
	assert.Len(t, clusters, 3)

	// Check first cluster
	firstCluster, ok := clusters[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "in-cluster", firstCluster["name"])
	assert.Equal(t, "https://kubernetes.default.svc", firstCluster["server"])
}

func TestListClusterSummarySize(t *testing.T) {
	token := "bearer-marker:" + strings.Repeat("t", 2048)
	items := make([]v1alpha1.Cluster, 40)
	for i := range items {
		items[i] = v1alpha1.Cluster{
			Name:   "cluster-" + itoaSmall(i),
			Server: "https://example.com/" + itoaSmall(i),
			Config: v1alpha1.ClusterConfig{BearerToken: token},
			Annotations: map[string]string{
				"keep-me":             "yes",
				lastAppliedAnnotation: "applied",
			},
			Info: v1alpha1.ClusterInfo{ConnectionState: v1alpha1.ConnectionState{Status: v1alpha1.ConnectionStatusSuccessful}},
		}
	}
	list := &v1alpha1.ClusterList{Items: items}
	before, err := json.MarshalIndent(list, "", "  ")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().ListClusters(gomock.Any()).Return(list, nil).Times(2)

	summaryRes, err := listClusterHandler(context.Background(), m, false, false, 50, 0)
	require.NoError(t, err)
	summary := toolText(t, summaryRes)
	assert.Contains(t, summary, `"name"`)
	assert.Contains(t, summary, `"server"`)
	assert.Contains(t, summary, `"connectionStatus"`)
	assert.NotContains(t, summary, "bearer-marker")
	assert.Contains(t, summary, `"totalItems":40`)
	tokenBytes := strings.Count(string(before), token) * len(token)
	t.Logf("before_bytes=%d after_bytes=%d bearer_bytes=%d", len(before), len(summary), tokenBytes)
	assert.Less(t, len(summary), len(before)/5)
	assert.Greater(t, tokenBytes, len(summary))

	detailedRes, err := listClusterHandler(context.Background(), m, true, false, 50, 0)
	require.NoError(t, err)
	detailed := toolText(t, detailedRes)
	assert.NotContains(t, detailed, lastAppliedAnnotation)
	assert.Contains(t, detailed, "keep-me")
	assert.NotContains(t, detailed, "bearer-marker")
}

func itoaSmall(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
