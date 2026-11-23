package tools

import (
	"context"
	"testing"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

func TestGetApplicationManagedResources_Schema(t *testing.T) {
	assert.Equal(t, "get_application_managed_resources", GetApplicationManagedResourcesTool.Name)
	assert.Contains(t, GetApplicationManagedResourcesTool.Description, "ManagedResources")
	assert.Equal(t, []string{"name"}, GetApplicationManagedResourcesTool.InputSchema.Required)
	require.NotNil(t, GetApplicationManagedResourcesTool.Annotations.ReadOnlyHint)
	assert.True(t, *GetApplicationManagedResourcesTool.Annotations.ReadOnlyHint)
	require.NotNil(t, GetApplicationManagedResourcesTool.Annotations.DestructiveHint)
	assert.False(t, *GetApplicationManagedResourcesTool.Annotations.DestructiveHint)

	props := GetApplicationManagedResourcesTool.InputSchema.Properties
	for _, key := range []string{"name", "resource_namespace", "resource_name", "version", "group", "kind", "app_namespace", "project"} {
		prop, ok := props[key].(map[string]any)
		require.True(t, ok, key)
		assert.NotEmpty(t, prop["description"], key)
	}
	version, _ := props["version"].(map[string]any)
	assert.Contains(t, version["description"], "not a git revision")
}

func TestGetApplicationManagedResourcesHandler(t *testing.T) {
	opts := client.ManagedResourcesOptions{
		Name: "app", ResourceNamespace: "prod", ResourceName: "web",
		Version: "v1", Group: "apps", Kind: "Deployment",
		AppNamespace: "team-a", Project: "proj",
	}
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationManagedResources(gomock.Any(), opts).Return(&applicationpkg.ManagedResourcesResponse{
		Items: []*v1alpha1.ResourceDiff{{Kind: "Deployment", Name: "web"}},
	}, nil)

	res, err := getApplicationManagedResourcesHandler(context.Background(), m, opts)
	require.NoError(t, err)
	assert.False(t, res.IsError)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Contains(t, text.Text, "web")

	res, err = getApplicationManagedResourcesHandler(context.Background(), m, client.ManagedResourcesOptions{})
	require.NoError(t, err)
	assert.True(t, res.IsError)
}
