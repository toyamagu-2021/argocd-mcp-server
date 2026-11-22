package tools

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// publicTools is the locked list of public tools (design D1: 23 tools).
var publicTools = []string{
	"list_application",
	"get_application",
	"get_application_manifests",
	"get_application_events",
	"get_application_logs",
	"get_application_resource_tree",
	"create_application",
	"sync_application",
	"refresh_application",
	"delete_application",
	"terminate_operation",
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
	m.EXPECT().GetApplication(gomock.Any(), "app1").Return(&v1alpha1.Application{
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
				m.EXPECT().SyncApplication(gomock.Any(), "app1", gomock.Any(), true, true).
					Return(&v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app1"}}, nil)
			},
		},
		{
			name: "delete passes explicit cascade",
			tool: "delete_application",
			args: map[string]interface{}{"name": "app1", "cascade": false},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().DeleteApplication(gomock.Any(), "app1", false).Return(nil)
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
	"sync_application",
	"refresh_application",
	"delete_application",
	"terminate_operation",
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

	// D11: refresh is a non-destructive, idempotent write.
	r := RefreshAppTool.Annotations
	assert.False(t, *r.ReadOnlyHint)
	assert.False(t, *r.DestructiveHint)
	require.NotNil(t, r.IdempotentHint)
	assert.True(t, *r.IdempotentHint)
}
