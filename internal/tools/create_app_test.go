package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

func TestCreateAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if CreateAppTool.Name != "create_application" {
		t.Errorf("Expected tool name 'create_application', got %s", CreateAppTool.Name)
	}

	assert.Contains(t, CreateAppTool.Description, "get_application")
	assert.Contains(t, CreateAppTool.Description, "compact")

	// Check input schema exists
	if CreateAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", CreateAppTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(CreateAppTool.InputSchema.Properties) == 0 {
		t.Error("Tool schema should have properties defined")
	}

	// Check required fields are marked as required
	expectedRequired := []string{"name", "repo_url", "dest_namespace"}
	if CreateAppTool.InputSchema.Required == nil {
		t.Error("Tool schema should have required fields defined")
	} else {
		for _, req := range expectedRequired {
			found := false
			for _, actual := range CreateAppTool.InputSchema.Required {
				if actual == req {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected required field '%s' not found in schema", req)
			}
		}
	}
}

func TestCreateApplication_DefaultsAndSpec(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), false).
		DoAndReturn(func(_ interface{}, app *v1alpha1.Application, _ bool) (*v1alpha1.Application, error) {
			assert.Equal(t, "my-app", app.Name)
			assert.Equal(t, "argocd", app.Namespace)
			assert.Equal(t, "default", app.Spec.Project)
			require.NotNil(t, app.Spec.Source)
			assert.Equal(t, "https://github.com/example/repo", app.Spec.Source.RepoURL)
			assert.Equal(t, ".", app.Spec.Source.Path)
			assert.Equal(t, "HEAD", app.Spec.Source.TargetRevision)
			assert.Equal(t, "https://kubernetes.default.svc", app.Spec.Destination.Server)
			assert.Equal(t, "web", app.Spec.Destination.Namespace)
			assert.Nil(t, app.Spec.SyncPolicy)
			return app, nil
		})

	res := callTool(t, newTestServer(t, Deps{Client: m}), "create_application", map[string]interface{}{
		"name": "my-app", "repo_url": "https://github.com/example/repo", "dest_namespace": "web",
	})
	assert.False(t, res.IsError)
}

func TestCreateApplication_AllArguments(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), true).
		DoAndReturn(func(_ interface{}, app *v1alpha1.Application, _ bool) (*v1alpha1.Application, error) {
			assert.Equal(t, "team-a", app.Namespace)
			assert.Equal(t, "proj", app.Spec.Project)
			assert.Equal(t, "charts/web", app.Spec.Source.Path)
			assert.Equal(t, "v1.2.3", app.Spec.Source.TargetRevision)
			assert.Equal(t, "https://other.example", app.Spec.Destination.Server)
			return app, nil
		})

	res := callTool(t, newTestServer(t, Deps{Client: m}), "create_application", map[string]interface{}{
		"name": "my-app", "namespace": "team-a", "project": "proj",
		"repo_url": "https://github.com/example/repo", "path": "charts/web",
		"target_revision": "v1.2.3", "dest_server": "https://other.example",
		"dest_namespace": "web", "upsert": true,
	})
	assert.False(t, res.IsError)
}

func TestCreateApplicationHandler_SyncPolicy(t *testing.T) {
	tests := []struct {
		name               string
		autoSync, selfHeal bool
		check              func(t *testing.T, p *v1alpha1.SyncPolicy)
	}{
		{"manual", false, false, func(t *testing.T, p *v1alpha1.SyncPolicy) { assert.Nil(t, p) }},
		{"auto sync", true, false, func(t *testing.T, p *v1alpha1.SyncPolicy) {
			require.NotNil(t, p)
			require.NotNil(t, p.Automated)
			assert.Nil(t, p.Automated.SelfHeal)
		}},
		{"auto sync with self heal", true, true, func(t *testing.T, p *v1alpha1.SyncPolicy) {
			require.NotNil(t, p)
			require.NotNil(t, p.Automated)
			require.NotNil(t, p.Automated.SelfHeal)
			assert.True(t, *p.Automated.SelfHeal)
		}},
		{"self heal alone does not enable auto sync", false, true, func(t *testing.T, p *v1alpha1.SyncPolicy) {
			if p != nil {
				assert.Nil(t, p.Automated)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := mock.NewMockInterface(ctrl)
			m.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), false).
				DoAndReturn(func(_ interface{}, app *v1alpha1.Application, _ bool) (*v1alpha1.Application, error) {
					tt.check(t, app.Spec.SyncPolicy)
					return app, nil
				})
			res, err := createApplicationHandler(context.Background(), m, CreateAppParams{
				Name: "a", RepoURL: "r", DestNamespace: "ns", AutoSync: tt.autoSync, SelfHeal: tt.selfHeal,
			})
			require.NoError(t, err)
			assert.False(t, res.IsError)
		})
	}
}

func TestCreateApplicationHandler_Errors(t *testing.T) {
	valid := CreateAppParams{Name: "a", RepoURL: "r", DestNamespace: "ns"}
	for _, tt := range []struct {
		name   string
		mutate func(p *CreateAppParams)
		want   string
	}{
		{"missing name", func(p *CreateAppParams) { p.Name = "" }, "Application name is required"},
		{"missing repo", func(p *CreateAppParams) { p.RepoURL = "" }, "Repository URL is required"},
		{"missing dest namespace", func(p *CreateAppParams) { p.DestNamespace = "" }, "Destination namespace is required"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			p := valid
			tt.mutate(&p)
			res, err := createApplicationHandler(context.Background(), mock.NewMockInterface(ctrl), p)
			require.NoError(t, err)
			require.True(t, res.IsError)
			text, _ := mcp.AsTextContent(res.Content[0])
			assert.Contains(t, text.Text, tt.want)
		})
	}

	t.Run("client error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		m := mock.NewMockInterface(ctrl)
		m.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), false).Return(nil, errors.New("already exists"))
		res, err := createApplicationHandler(context.Background(), m, valid)
		require.NoError(t, err)
		require.True(t, res.IsError)
		text, _ := mcp.AsTextContent(res.Content[0])
		assert.Contains(t, text.Text, "already exists")
	})
}
