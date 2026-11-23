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
)

func TestGetClusterKeepsServerFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	cluster := clusterWithServerFields()
	m.EXPECT().GetCluster(gomock.Any(), cluster.Server).Return(cluster, nil)

	res, err := getClusterHandler(context.Background(), m, cluster.Server)
	require.NoError(t, err)
	text := toolText(t, res)

	assert.NotContains(t, text, "\n")
	assert.NotContains(t, text, "secret-token-marker")
	assert.NotContains(t, text, lastAppliedAnnotation)
	assertServerFields(t, text)
	assert.Equal(t, "blob", cluster.Annotations[lastAppliedAnnotation])
}

func TestListClusterDetailedKeepsServerFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	cluster := clusterWithServerFields()
	m.EXPECT().ListClusters(gomock.Any()).Return(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{*cluster}}, nil)

	res, err := listClusterHandler(context.Background(), m, true, false, 50, 0)
	require.NoError(t, err)
	text := toolText(t, res)

	var page struct {
		Items      []json.RawMessage `json:"items"`
		TotalItems int               `json:"totalItems"`
		HasMore    bool              `json:"hasMore"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &page))
	require.Len(t, page.Items, 1)
	assert.Equal(t, 1, page.TotalItems)
	assert.False(t, page.HasMore)
	assert.NotContains(t, text, "\n")
	assert.NotContains(t, text, "secret-token-marker")
	assert.NotContains(t, text, lastAppliedAnnotation)
	assertServerFields(t, string(page.Items[0]))
	assert.Equal(t, "blob", cluster.Annotations[lastAppliedAnnotation])
}

func TestListRepositoryDetailedKeepsConnectionState(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	repo := &v1alpha1.Repository{
		Repo:     "https://example.com/app.git",
		Type:     "git",
		Name:     "app",
		Project:  "default",
		Username: "git-user",
		Password: "secret-password-marker",
		ConnectionState: v1alpha1.ConnectionState{
			Status:  v1alpha1.ConnectionStatusFailed,
			Message: "authentication failed",
		},
	}
	m.EXPECT().ListRepositories(gomock.Any()).Return(&v1alpha1.RepositoryList{
		Items: v1alpha1.Repositories{repo},
	}, nil)

	res, err := listRepositoryHandler(context.Background(), m, true, 50, 0)
	require.NoError(t, err)
	text := toolText(t, res)

	assert.NotContains(t, text, "\n")
	assert.NotContains(t, text, "secret-password-marker")
	var page struct {
		Items []v1alpha1.Repository `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &page))
	require.Len(t, page.Items, 1)
	assert.Equal(t, v1alpha1.ConnectionStatusFailed, page.Items[0].ConnectionState.Status)
	assert.Equal(t, "authentication failed", page.Items[0].ConnectionState.Message)
	assert.Equal(t, "git-user", page.Items[0].Username)
	assert.Empty(t, page.Items[0].Password)
}

func clusterWithServerFields() *v1alpha1.Cluster {
	return &v1alpha1.Cluster{
		Server: "https://kubernetes.default.svc",
		Name:   "in-cluster",
		//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
		ServerVersion: "1.28.0",
		//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
		ConnectionState: v1alpha1.ConnectionState{
			Status:  v1alpha1.ConnectionStatusSuccessful,
			Message: "connected",
		},
		Annotations: map[string]string{
			"keep":                "yes",
			lastAppliedAnnotation: "blob",
		},
		Config: v1alpha1.ClusterConfig{
			BearerToken: "secret-token-marker",
			TLSClientConfig: v1alpha1.TLSClientConfig{
				Insecure:   true,
				ServerName: "kube.example",
			},
		},
		Info: v1alpha1.ClusterInfo{
			ServerVersion: "1.28.0",
			ConnectionState: v1alpha1.ConnectionState{
				Status:  v1alpha1.ConnectionStatusSuccessful,
				Message: "connected",
			},
		},
	}
}

func assertServerFields(t *testing.T, text string) {
	t.Helper()
	var got v1alpha1.Cluster
	require.NoError(t, json.Unmarshal([]byte(text), &got))
	//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
	assert.Equal(t, "1.28.0", got.ServerVersion)
	//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
	assert.Equal(t, v1alpha1.ConnectionStatusSuccessful, got.ConnectionState.Status)
	//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
	assert.Equal(t, "connected", got.ConnectionState.Message)
	assert.Equal(t, "yes", got.Annotations["keep"])
	assert.NotContains(t, got.Annotations, lastAppliedAnnotation)
	assert.Empty(t, got.Config.BearerToken)
	assert.True(t, got.Config.Insecure)
	assert.Equal(t, "kube.example", got.Config.ServerName)
	assert.False(t, strings.Contains(text, "secret-token-marker"))
}
