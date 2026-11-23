package tools

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncAppTool_Schema(t *testing.T) {
	assert.Equal(t, "sync_application", SyncAppTool.Name)
	assert.NotEmpty(t, SyncAppTool.Description)
	assert.Equal(t, "object", SyncAppTool.InputSchema.Type)
	assert.Equal(t, []string{"name"}, SyncAppTool.InputSchema.Required)

	props := SyncAppTool.InputSchema.Properties
	for _, key := range []string{"name", "prune", "dry_run", "app_namespace", "revision", "revisions", "resources", "sync_options"} {
		prop, ok := props[key].(map[string]any)
		require.True(t, ok, "property %s", key)
		assert.NotEmpty(t, prop["description"], key)
	}

	rev, _ := props["revision"].(map[string]any)
	assert.Equal(t, "string", rev["type"])
	assert.Contains(t, rev["description"], "single-source")

	revs, _ := props["revisions"].(map[string]any)
	assert.Equal(t, "array", revs["type"])
	assert.Contains(t, revs["description"], "spec.sources")

	resources, _ := props["resources"].(map[string]any)
	assert.Equal(t, "array", resources["type"])
	items, ok := resources["items"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"kind", "name"}, items["required"])

	opts, _ := props["sync_options"].(map[string]any)
	assert.Equal(t, "array", opts["type"])
	assert.Contains(t, opts["description"], "replace")
	assert.Contains(t, opts["description"], "not merged")

	require.NotNil(t, SyncAppTool.Annotations.DestructiveHint)
	assert.True(t, *SyncAppTool.Annotations.DestructiveHint)
	require.NotNil(t, SyncAppTool.Annotations.ReadOnlyHint)
	assert.False(t, *SyncAppTool.Annotations.ReadOnlyHint)
}

func TestSyncApplicationHandler(t *testing.T) {
	app := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app1"}}
	tests := []struct {
		name    string
		opts    client.SyncOptions
		expect  func(m *mock.MockInterface)
		wantErr string
	}{
		{
			name: "omitted revision still syncs",
			opts: client.SyncOptions{Name: "app1"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().SyncApplication(gomock.Any(), client.SyncOptions{Name: "app1"}).Return(app, nil)
			},
		},
		{
			name: "revision, resources, multi-source revisions, and sync options",
			opts: client.SyncOptions{
				Name: "app1", Revision: "abc123", Revisions: []string{"v1", ""},
				Prune: true, DryRun: true, AppNamespace: "team-a",
				Resources: []client.SyncResource{{Group: "apps", Kind: "Deployment", Name: "web", Namespace: "prod"}},
				Options:   []string{"CreateNamespace=true"},
			},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().SyncApplication(gomock.Any(), client.SyncOptions{
					Name: "app1", Revision: "abc123", Revisions: []string{"v1", ""},
					Prune: true, DryRun: true, AppNamespace: "team-a",
					Resources: []client.SyncResource{{Group: "apps", Kind: "Deployment", Name: "web", Namespace: "prod"}},
					Options:   []string{"CreateNamespace=true"},
				}).Return(app, nil)
			},
		},
		{
			name:    "name required",
			opts:    client.SyncOptions{Revision: "abc"},
			wantErr: "Application name is required",
		},
		{
			name:    "resource needs kind and name",
			opts:    client.SyncOptions{Name: "app1", Resources: []client.SyncResource{{Group: "apps"}}},
			wantErr: "resources[0] requires kind and name",
		},
		{
			name: "client error",
			opts: client.SyncOptions{Name: "app1", Revision: "abc"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().SyncApplication(gomock.Any(), client.SyncOptions{Name: "app1", Revision: "abc"}).
					Return(nil, assert.AnError)
			},
			wantErr: "Failed to sync application",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mock.NewMockInterface(ctrl)
			if tt.expect != nil {
				tt.expect(m)
			}
			res, err := syncApplicationHandler(context.Background(), m, tt.opts)
			require.NoError(t, err)
			require.NotNil(t, res)
			if tt.wantErr != "" {
				require.True(t, res.IsError)
				text, ok := mcp.AsTextContent(res.Content[0])
				require.True(t, ok)
				assert.Contains(t, text.Text, tt.wantErr)
				return
			}
			assert.False(t, res.IsError)
			text, ok := mcp.AsTextContent(res.Content[0])
			require.True(t, ok)
			assert.Contains(t, text.Text, "app1")
		})
	}
}

func TestSyncApplication_ArgumentValidation(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := newTestServer(t, Deps{Client: mock.NewMockInterface(ctrl)})

	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"resources not an array", map[string]any{"name": "app1", "resources": "deploy"}, "resources must be an array"},
		{"resource not an object", map[string]any{"name": "app1", "resources": []any{"deploy"}}, "resources[0] must be an object"},
		{"resource missing kind", map[string]any{"name": "app1", "resources": []any{map[string]any{"name": "web"}}}, "resources[0] requires kind and name"},
		{"kind not a string", map[string]any{"name": "app1", "resources": []any{map[string]any{"kind": 1, "name": "web"}}}, "kind must be a string"},
		{"revisions not an array", map[string]any{"name": "app1", "revisions": "main"}, "revisions must be an array of strings"},
		{"revision entry not a string", map[string]any{"name": "app1", "revisions": []any{1}}, "revisions[0] must be a string"},
		{"empty sync option", map[string]any{"name": "app1", "sync_options": []any{""}}, "sync_options[0] must not be empty"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			res := callTool(t, s, "sync_application", tt.args)
			require.True(t, res.IsError)
			text, ok := mcp.AsTextContent(res.Content[0])
			require.True(t, ok)
			assert.Contains(t, text.Text, tt.want)
		})
	}
}
