package tools

import (
	"context"
	"testing"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

func TestGetApplicationResource_Schema(t *testing.T) {
	assert.Equal(t, "get_application_resource", GetApplicationResourceTool.Name)
	assert.Contains(t, GetApplicationResourceTool.Description, "GetResource")
	assert.Contains(t, GetApplicationResourceTool.Description, "Secret")
	assert.ElementsMatch(t, []string{"name", "resource_name", "kind", "version"}, GetApplicationResourceTool.InputSchema.Required)
	require.NotNil(t, GetApplicationResourceTool.Annotations.ReadOnlyHint)
	assert.True(t, *GetApplicationResourceTool.Annotations.ReadOnlyHint)

	props := GetApplicationResourceTool.InputSchema.Properties
	version, ok := props["version"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, version["description"], "not a git revision")
	group, ok := props["group"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, group["description"], "core")
}

func TestGetApplicationResourceHandler(t *testing.T) {
	q := client.ResourceQuery{
		Name: "app", ResourceName: "web", Version: "v1", Group: "apps", Kind: "Deployment",
		ResourceNamespace: "prod", AppNamespace: "team-a", Project: "proj",
	}
	manifest := `{"kind":"Deployment","metadata":{"name":"web"}}`
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationResource(gomock.Any(), q).Return(&applicationpkg.ApplicationResourceResponse{
		Manifest: &manifest,
	}, nil)

	res, err := getApplicationResourceHandler(context.Background(), m, q)
	require.NoError(t, err)
	assert.False(t, res.IsError)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Contains(t, text.Text, "Deployment")

	for _, missing := range []client.ResourceQuery{
		{},
		{Name: "app"},
		{Name: "app", ResourceName: "web"},
		{Name: "app", ResourceName: "web", Kind: "Deployment"},
	} {
		res, err = getApplicationResourceHandler(context.Background(), m, missing)
		require.NoError(t, err)
		assert.True(t, res.IsError)
	}
}
