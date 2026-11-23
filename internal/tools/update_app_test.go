package tools

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

func TestUpdateApplication_Schema(t *testing.T) {
	assert.Equal(t, "update_application", UpdateApplicationTool.Name)
	assert.Contains(t, UpdateApplicationTool.Description, "UpdateSpec")
	assert.Contains(t, UpdateApplicationTool.Description, "replaces the whole spec")
	assert.ElementsMatch(t, []string{"name", "spec"}, UpdateApplicationTool.InputSchema.Required)
	require.NotNil(t, UpdateApplicationTool.Annotations.ReadOnlyHint)
	assert.False(t, *UpdateApplicationTool.Annotations.ReadOnlyHint)
	require.NotNil(t, UpdateApplicationTool.Annotations.DestructiveHint)
	assert.True(t, *UpdateApplicationTool.Annotations.DestructiveHint)
	require.NotNil(t, UpdateApplicationTool.Annotations.IdempotentHint)
	assert.True(t, *UpdateApplicationTool.Annotations.IdempotentHint)
	validate, ok := UpdateApplicationTool.InputSchema.Properties["validate"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, validate["description"], "default: true")
}

func TestUpdateApplication_Wiring(t *testing.T) {
	spec := map[string]any{
		"project": "default",
		"source": map[string]any{
			"repoURL": "https://github.com/argoproj/argocd-example-apps.git", "path": "guestbook", "targetRevision": "HEAD",
		},
		"destination":          map[string]any{"server": "https://kubernetes.default.svc", "namespace": "default"},
		"revisionHistoryLimit": int64(10),
	}
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().UpdateApplicationSpec(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ interface{}, opts client.UpdateSpecOptions) (*v1alpha1.ApplicationSpec, error) {
			assert.Equal(t, "app", opts.Name)
			assert.Equal(t, "team-a", opts.AppNamespace)
			assert.Equal(t, "default", opts.Project)
			assert.False(t, opts.Validate)
			require.NotNil(t, opts.Spec)
			assert.Equal(t, "default", opts.Spec.Project)
			require.NotNil(t, opts.Spec.Source)
			assert.Equal(t, "guestbook", opts.Spec.Source.Path)
			require.NotNil(t, opts.Spec.RevisionHistoryLimit)
			assert.Equal(t, int64(10), *opts.Spec.RevisionHistoryLimit)
			return opts.Spec, nil
		},
	)
	res := callTool(t, newTestServer(t, Deps{Client: m}), "update_application", map[string]any{
		"name": "app", "app_namespace": "team-a", "project": "default", "validate": false, "spec": spec,
	})
	assert.False(t, res.IsError)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Contains(t, text.Text, "guestbook")
}

func TestUpdateApplication_ValidateDefaultsTrue(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().UpdateApplicationSpec(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ interface{}, opts client.UpdateSpecOptions) (*v1alpha1.ApplicationSpec, error) {
			assert.True(t, opts.Validate)
			return opts.Spec, nil
		},
	)
	res := callTool(t, newTestServer(t, Deps{Client: m}), "update_application", map[string]any{
		"name": "app",
		"spec": map[string]any{
			"project": "default",
			"source":  map[string]any{"repoURL": "https://example.com", "path": "."},
		},
	})
	assert.False(t, res.IsError)
}

func TestUpdateApplication_RejectsPartialSpec(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := newTestServer(t, Deps{Client: mock.NewMockInterface(ctrl)})
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing spec", map[string]any{"name": "app"}, "spec is required"},
		{"spec not an object", map[string]any{"name": "app", "spec": "nope"}, "spec must be an object"},
		{"missing project", map[string]any{"name": "app", "spec": map[string]any{"source": map[string]any{"repoURL": "https://example.com"}}}, "spec.project is required"},
		{"missing source", map[string]any{"name": "app", "spec": map[string]any{"project": "default"}}, "repoURL is required"},
		{"empty source", map[string]any{"name": "app", "spec": map[string]any{"project": "default", "source": map[string]any{}}}, "repoURL is required"},
		{"empty sources entry", map[string]any{"name": "app", "spec": map[string]any{"project": "default", "sources": []any{map[string]any{}}}}, "repoURL is required"},
		{"validate not a bool", map[string]any{"name": "app", "validate": "yes", "spec": map[string]any{"project": "default", "source": map[string]any{"repoURL": "https://example.com"}}}, "validate must be a boolean"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			res := callTool(t, s, "update_application", tt.args)
			require.True(t, res.IsError)
			text, ok := mcp.AsTextContent(res.Content[0])
			require.True(t, ok)
			assert.Contains(t, text.Text, tt.want)
		})
	}
}
