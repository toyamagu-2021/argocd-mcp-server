package tools

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// publicTools is the locked list of public tools (design D1, revised 2026-11-23: 27 tools).
var publicTools = []string{
	"list_application",
	"get_application",
	"get_application_manifests",
	"get_application_events",
	"get_application_logs",
	"get_application_resource_tree",
	"get_application_managed_resources",
	"get_application_resource",
	"create_application",
	"update_application",
	"sync_application",
	"refresh_application",
	"delete_application",
	"terminate_operation",
	"rollback_application",
	"list_project",
	"get_project",
	"create_project",
	"list_cluster",
	"get_cluster",
	"list_applicationset",
	"get_applicationset",
	"create_applicationset",
	"delete_applicationset",
	"list_repository",
	"get_repository",
	"get_user_info",
}

func newTestServer(t *testing.T, d Deps) *server.MCPServer {
	t.Helper()
	s := server.NewMCPServer("test", "0.0.0")
	Register(s, d)
	return s
}

func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]interface{}) *mcp.CallToolResult {
	t.Helper()
	st := s.GetTool(name)
	require.NotNil(t, st, "tool %s not registered", name)
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: args}}
	res, err := st.Handler(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	return res
}

func TestRegister_AllPublicTools(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := newTestServer(t, Deps{Client: mock.NewMockInterface(ctrl)})

	got := s.ListTools()
	assert.Len(t, got, len(publicTools))
	for _, name := range publicTools {
		assert.Contains(t, got, name)
	}
}

// The handlers must use the injected client and never close it.
func TestRegister_UsesInjectedClient(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	// No Close() expectation: gomock fails the test if a handler closes the client.
	m.EXPECT().GetApplication(gomock.Any(), "app1", "").Return(&v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "argocd"},
	}, nil).Times(2)

	s := newTestServer(t, Deps{Client: m})
	for i := 0; i < 2; i++ {
		res := callTool(t, s, "get_application", map[string]interface{}{"name": "app1"})
		assert.False(t, res.IsError)
	}
}

func TestRegister_ArgumentWiring(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		args   map[string]interface{}
		expect func(m *mock.MockInterface)
	}{
		{
			name: "sync passes prune and dry_run",
			tool: "sync_application",
			args: map[string]interface{}{"name": "app1", "prune": true, "dry_run": true},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().SyncApplication(gomock.Any(), client.SyncOptions{Name: "app1", Prune: true, DryRun: true}).
					Return(&v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app1"}}, nil)
			},
		},
		{
			name: "delete passes explicit cascade",
			tool: "delete_application",
			args: map[string]interface{}{"name": "app1", "cascade": false},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().DeleteApplication(gomock.Any(), "app1", false, "").Return(nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mock.NewMockInterface(ctrl)
			tt.expect(m)
			s := newTestServer(t, Deps{Client: m})
			res := callTool(t, s, tt.tool, tt.args)
			assert.False(t, res.IsError)
		})
	}
}

// Tools removed by ARGOCD_MCP_READ_ONLY=true (design §2).
var mutatingTools = []string{
	"create_application",
	"update_application",
	"sync_application",
	"delete_application",
	"terminate_operation",
	"rollback_application",
	"create_project",
	"create_applicationset",
	"delete_applicationset",
}

func TestRegister_ReadOnly(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := newTestServer(t, Deps{Client: mock.NewMockInterface(ctrl), ReadOnly: true})

	got := s.ListTools()
	assert.Len(t, got, len(publicTools)-len(mutatingTools))
	for _, name := range mutatingTools {
		assert.NotContains(t, got, name)
	}
	for name, st := range got {
		require.NotNil(t, st.Tool.Annotations.ReadOnlyHint, name)
		assert.True(t, *st.Tool.Annotations.ReadOnlyHint, "%s is registered in read-only mode but not marked readOnly", name)
	}
}

func TestToolHints(t *testing.T) {
	mutating := map[string]bool{}
	for _, n := range mutatingTools {
		mutating[n] = true
	}
	for _, e := range entries() {
		a := e.tool.Annotations
		name := e.tool.Name
		assert.Equal(t, mutating[name], e.mutating, "%s: mutating flag", name)
		require.NotNil(t, a.ReadOnlyHint, name)
		require.NotNil(t, a.DestructiveHint, name)
		if e.mutating {
			assert.False(t, *a.ReadOnlyHint, "%s must not be readOnly", name)
		} else {
			assert.True(t, *a.ReadOnlyHint, "%s must be readOnly", name)
			assert.False(t, *a.DestructiveHint, "%s must not be destructive", name)
		}
	}

	// D11 (revised): refresh is a Get with refresh=normal|hard; treated as read.
	r := RefreshAppTool.Annotations
	assert.True(t, *r.ReadOnlyHint)
	assert.False(t, *r.DestructiveHint)
	require.NotNil(t, r.IdempotentHint)
	assert.True(t, *r.IdempotentHint)
}

func TestRegister_AppNamespace(t *testing.T) {
	app := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "team-a"}}
	tests := []struct {
		tool   string
		args   map[string]interface{}
		expect func(m *mock.MockInterface)
	}{
		{"get_application", map[string]interface{}{"name": "app1", "app_namespace": "team-a"}, func(m *mock.MockInterface) {
			m.EXPECT().GetApplication(gomock.Any(), "app1", "team-a").Return(app, nil)
		}},
		{"sync_application", map[string]interface{}{"name": "app1", "app_namespace": "team-a"}, func(m *mock.MockInterface) {
			m.EXPECT().SyncApplication(gomock.Any(), client.SyncOptions{Name: "app1", AppNamespace: "team-a"}).Return(app, nil)
		}},
		{"sync_application", map[string]interface{}{
			"name": "app1", "revision": "abc123",
			"revisions":    []any{"v1", "v2"},
			"resources":    []any{map[string]any{"group": "apps", "kind": "Deployment", "name": "web", "namespace": "prod"}},
			"sync_options": []any{"CreateNamespace=true"},
		}, func(m *mock.MockInterface) {
			m.EXPECT().SyncApplication(gomock.Any(), client.SyncOptions{
				Name: "app1", Revision: "abc123", Revisions: []string{"v1", "v2"},
				Resources: []client.SyncResource{{Group: "apps", Kind: "Deployment", Name: "web", Namespace: "prod"}},
				Options:   []string{"CreateNamespace=true"},
			}).Return(app, nil)
		}},
		{"refresh_application", map[string]interface{}{"name": "app1", "app_namespace": "team-a"}, func(m *mock.MockInterface) {
			m.EXPECT().RefreshApplication(gomock.Any(), "app1", "normal", "team-a").Return(app, nil)
		}},
		{"delete_application", map[string]interface{}{"name": "app1", "cascade": true, "app_namespace": "team-a"}, func(m *mock.MockInterface) {
			m.EXPECT().DeleteApplication(gomock.Any(), "app1", true, "team-a").Return(nil)
		}},
		{"get_application_manifests", map[string]interface{}{"name": "app1", "app_namespace": "team-a"}, func(m *mock.MockInterface) {
			m.EXPECT().GetApplicationManifests(gomock.Any(), "app1", "", "team-a").Return(&repoapiclient.ManifestResponse{}, nil)
		}},
		{"list_application", map[string]interface{}{"project": "p1", "selector": "a=b", "app_namespace": "team-a", "output_format": "json"}, func(m *mock.MockInterface) {
			m.EXPECT().ListApplications(gomock.Any(), client.ListApplicationsOptions{
				Selector: "a=b", Projects: []string{"p1"}, AppNamespace: "team-a",
			}).Return(&v1alpha1.ApplicationList{}, nil)
		}},
		{"delete_applicationset", map[string]interface{}{"name": "set", "app_namespace": "team-a"}, func(m *mock.MockInterface) {
			m.EXPECT().DeleteApplicationSet(gomock.Any(), "set", "team-a").Return(nil)
		}},
		{"delete_applicationset", map[string]interface{}{"name": "set", "appsetNamespace": "legacy"}, func(m *mock.MockInterface) {
			m.EXPECT().DeleteApplicationSet(gomock.Any(), "set", "legacy").Return(nil)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mock.NewMockInterface(ctrl)
			tt.expect(m)
			res := callTool(t, newTestServer(t, Deps{Client: m}), tt.tool, tt.args)
			assert.False(t, res.IsError)
		})
	}
}

func TestDeleteApplicationSet_ConflictingNamespaces(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := newTestServer(t, Deps{Client: mock.NewMockInterface(ctrl)})
	res := callTool(t, s, "delete_applicationset", map[string]interface{}{
		"name": "set", "app_namespace": "a", "appsetNamespace": "b",
	})
	assert.True(t, res.IsError)
}

func TestResolveAppSetNamespace(t *testing.T) {
	for _, tt := range []struct{ app, legacy, want string }{
		{"", "", ""}, {"a", "", "a"}, {"", "b", "b"}, {"a", "a", "a"},
	} {
		got, err := resolveAppSetNamespace(tt.app, tt.legacy)
		require.NoError(t, err)
		assert.Equal(t, tt.want, got)
	}
	_, err := resolveAppSetNamespace("a", "b")
	assert.Error(t, err)
}

// D7: omitting cascade must not delete cluster resources.
func TestDeleteApplication_CascadeDefaultsToFalse(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().DeleteApplication(gomock.Any(), "app1", false, "").Return(nil)
	res := callTool(t, newTestServer(t, Deps{Client: m}), "delete_application", map[string]interface{}{"name": "app1"})
	assert.False(t, res.IsError)

	assert.Contains(t, DeleteAppTool.Description, "cascade: true")
	assert.Contains(t, DeleteAppTool.Description, "Argo CD CLI")
}

// D18: follow=true must fail fast instead of hanging the stdio call.
func TestGetApplicationLogs_FollowRejected(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl) // no GetApplicationLogs expectation: must not be called
	res := callTool(t, newTestServer(t, Deps{Client: m}), "get_application_logs", map[string]interface{}{
		"name": "app1", "follow": true,
	})
	require.True(t, res.IsError)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Contains(t, text.Text, "follow=true is not supported")

	res2, err := getApplicationLogsHandler(context.Background(), m, client.LogsOptions{Name: "app1", Follow: true})
	require.NoError(t, err)
	assert.True(t, res2.IsError)
	assert.Contains(t, GetApplicationLogsToolDefinition.Description, "follow=true")
}
