package tools

import (
	"testing"

	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	testGenerators = `[{"list":{"elements":[{"cluster":"test"}]}}]`
	testTemplate   = `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`
)

// TestToolArgumentWiring checks, for every tool not covered elsewhere, that the
// MCP argument names reach the client call unchanged. A typo in an argument name
// (e.g. "app_namesapce") would otherwise only be caught by the e2e suites.
func TestToolArgumentWiring(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		args   map[string]interface{}
		expect func(m *mock.MockInterface)
	}{
		{
			name: "get_application_events passes every filter",
			tool: "get_application_events",
			args: map[string]interface{}{
				"name": "app", "resource_namespace": "rns", "resource_name": "rname",
				"resource_uid": "uid", "app_namespace": "ans", "project": "proj",
			},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationEvents(gomock.Any(), "app", "rns", "rname", "uid", "ans", "proj").
					Return(map[string]interface{}{"items": []interface{}{}}, nil)
			},
		},
		{
			name: "get_application_resource_tree",
			tool: "get_application_resource_tree",
			args: map[string]interface{}{"name": "app", "app_namespace": "ans", "project": "proj"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationResourceTree(gomock.Any(), "app", "ans", "proj").
					Return(&v1alpha1.ApplicationTree{}, nil)
			},
		},
		{
			name: "terminate_operation",
			tool: "terminate_operation",
			args: map[string]interface{}{"name": "app", "app_namespace": "ans", "project": "proj"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().TerminateOperation(gomock.Any(), "app", "ans", "proj").Return(nil)
			},
		},
		{
			name: "get_applicationset",
			tool: "get_applicationset",
			args: map[string]interface{}{"name": "set"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationSet(gomock.Any(), "set").
					Return(&v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{Name: "set"}}, nil)
			},
		},
		{
			name: "list_applicationset passes project",
			tool: "list_applicationset",
			args: map[string]interface{}{"project": "proj"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().ListApplicationSets(gomock.Any(), "proj").Return(&v1alpha1.ApplicationSetList{}, nil)
			},
		},
		{
			name: "create_applicationset passes name, namespace, project, upsert and dry_run",
			tool: "create_applicationset",
			args: map[string]interface{}{
				"name": "set", "namespace": "team-a", "project": "proj",
				"generators": testGenerators, "template": testTemplate,
				"go_template": true, "upsert": true, "dry_run": true,
			},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), true, true).
					DoAndReturn(func(_ interface{}, s *v1alpha1.ApplicationSet, _, _ bool) (*v1alpha1.ApplicationSet, error) {
						assert.Equal(t, "set", s.Name)
						assert.Equal(t, "team-a", s.Namespace)
						assert.True(t, s.Spec.GoTemplate)
						require.Len(t, s.Spec.Generators, 1)
						return s, nil
					})
			},
		},
		{
			name: "get_cluster",
			tool: "get_cluster",
			args: map[string]interface{}{"server": "https://kubernetes.default.svc"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().GetCluster(gomock.Any(), "https://kubernetes.default.svc").
					Return(&v1alpha1.Cluster{Server: "https://kubernetes.default.svc"}, nil)
			},
		},
		{
			name: "list_cluster",
			tool: "list_cluster",
			args: map[string]interface{}{"name_only": true},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().ListClusters(gomock.Any()).Return(&v1alpha1.ClusterList{}, nil)
			},
		},
		{
			name: "get_project",
			tool: "get_project",
			args: map[string]interface{}{"name": "proj"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().GetProject(gomock.Any(), "proj").
					Return(&v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: "proj"}}, nil)
			},
		},
		{
			name: "list_project",
			tool: "list_project",
			args: map[string]interface{}{"name_only": true},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().ListProjects(gomock.Any()).Return(&v1alpha1.AppProjectList{}, nil)
			},
		},
		{
			name: "get_repository",
			tool: "get_repository",
			args: map[string]interface{}{"repo": "https://github.com/example/repo"},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().GetRepository(gomock.Any(), "https://github.com/example/repo").
					Return(&v1alpha1.Repository{Repo: "https://github.com/example/repo"}, nil)
			},
		},
		{
			name: "list_repository",
			tool: "list_repository",
			args: map[string]interface{}{},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().ListRepositories(gomock.Any()).Return(&v1alpha1.RepositoryList{}, nil)
			},
		},
		{
			name: "get_user_info",
			tool: "get_user_info",
			args: map[string]interface{}{},
			expect: func(m *mock.MockInterface) {
				m.EXPECT().GetUserInfo(gomock.Any()).
					Return(&sessionpkg.GetUserInfoResponse{LoggedIn: true, Username: "admin"}, nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mock.NewMockInterface(ctrl)
			tt.expect(m)
			res := callTool(t, newTestServer(t, Deps{Client: m}), tt.tool, tt.args)
			assert.False(t, res.IsError, "unexpected tool error: %v", res.Content)
		})
	}
}

// Required arguments must be rejected before any client call is made.
func TestToolRequiredArguments(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"get_application_resource_tree": {},
		"terminate_operation":           {},
		"get_applicationset":            {},
		"get_cluster":                   {},
		"get_project":                   {},
		"get_repository":                {},
		"create_project":                {},
		"create_applicationset":         {"name": "set", "template": testTemplate},
		"create_application":            {"name": "app", "dest_namespace": "ns"},
	}
	for tool, args := range cases {
		t.Run(tool, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mock.NewMockInterface(ctrl) // no expectations: any client call fails the test
			res := callTool(t, newTestServer(t, Deps{Client: m}), tool, args)
			assert.True(t, res.IsError)
		})
	}
}
