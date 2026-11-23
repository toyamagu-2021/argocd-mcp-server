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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRollbackApplication_Schema(t *testing.T) {
	assert.Equal(t, "rollback_application", RollbackApplicationTool.Name)
	assert.Contains(t, RollbackApplicationTool.Description, "Rollback")
	assert.Contains(t, RollbackApplicationTool.Description, "get_application")
	assert.Contains(t, RollbackApplicationTool.Description, "compact")
	assert.Contains(t, RollbackApplicationTool.Description, "default to false")
	assert.ElementsMatch(t, []string{"name", "id"}, RollbackApplicationTool.InputSchema.Required)
	require.NotNil(t, RollbackApplicationTool.Annotations.ReadOnlyHint)
	assert.False(t, *RollbackApplicationTool.Annotations.ReadOnlyHint)
	require.NotNil(t, RollbackApplicationTool.Annotations.DestructiveHint)
	assert.True(t, *RollbackApplicationTool.Annotations.DestructiveHint)
	require.NotNil(t, RollbackApplicationTool.Annotations.IdempotentHint)
	assert.False(t, *RollbackApplicationTool.Annotations.IdempotentHint)
}

func TestRollbackApplication_Wiring(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().RollbackApplication(gomock.Any(), client.RollbackOptions{
		Name: "app", ID: 0, DryRun: true, AppNamespace: "team-a", Project: "proj",
	}).Return(&v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app"}}, nil)

	res := callTool(t, newTestServer(t, Deps{Client: m}), "rollback_application", map[string]any{
		"name": "app", "id": float64(0), "dry_run": true, "app_namespace": "team-a", "project": "proj",
	})
	assert.False(t, res.IsError)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Contains(t, text.Text, "app")
}

func TestRollbackApplication_RejectsBadID(t *testing.T) {
	ctrl := gomock.NewController(t)
	s := newTestServer(t, Deps{Client: mock.NewMockInterface(ctrl)})
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"missing id", map[string]any{"name": "app"}, "id is required"},
		{"negative", map[string]any{"name": "app", "id": float64(-1)}, "id must be a non-negative integer"},
		{"fraction", map[string]any{"name": "app", "id": 1.5}, "id must be a non-negative integer"},
		{"string", map[string]any{"name": "app", "id": "1"}, "id must be a non-negative integer"},
		{"missing name", map[string]any{"id": float64(1)}, "name is required"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			res := callTool(t, s, "rollback_application", tt.args)
			require.True(t, res.IsError)
			text, ok := mcp.AsTextContent(res.Content[0])
			require.True(t, ok)
			assert.Contains(t, text.Text, tt.want)
		})
	}
}
