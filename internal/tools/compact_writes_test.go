package tools

import (
	"context"
	"encoding/json"
	"testing"

	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncApplicationCompactSize(t *testing.T) {
	app := sampleFatApplication()
	before, err := json.MarshalIndent(app, "", "  ")
	require.NoError(t, err)
	stripped := app.DeepCopy()
	stripObjectMeta(&stripped.ObjectMeta)
	indentedStripped, err := json.MarshalIndent(stripped, "", "  ")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().SyncApplication(gomock.Any(), gomock.Any()).Return(app, nil)

	text := toolText(t, mustSync(t, m))
	assertCompactApplication(t, text)
	assert.Contains(t, text, "helm-values-marker")
	t.Logf("before_bytes=%d indented_stripped_bytes=%d after_bytes=%d", len(before), len(indentedStripped), len(text))
	assert.Less(t, len(text), len(before))
	assert.Less(t, len(text), len(indentedStripped))
}

func TestWriteHandlersReturnCompactObjects(t *testing.T) {
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	app := noisyApplication()
	m.EXPECT().CreateApplication(gomock.Any(), gomock.Any(), false).Return(app.DeepCopy(), nil)
	m.EXPECT().RefreshApplication(gomock.Any(), "guestbook", "normal", "").Return(app.DeepCopy(), nil)
	m.EXPECT().RollbackApplication(gomock.Any(), gomock.Any()).Return(app.DeepCopy(), nil)

	created, err := createApplicationHandler(context.Background(), m, CreateAppParams{
		Name: "guestbook", RepoURL: "https://example.com/repo", DestNamespace: "default",
	})
	require.NoError(t, err)
	assertCompactApplication(t, toolText(t, created))

	refreshed, err := refreshApplicationHandler(context.Background(), m, "guestbook", false, "")
	require.NoError(t, err)
	assertCompactApplication(t, toolText(t, refreshed))

	rolled, err := rollbackApplicationHandler(context.Background(), m, client.RollbackOptions{Name: "guestbook", ID: 1})
	require.NoError(t, err)
	assertCompactApplication(t, toolText(t, rolled))

	spec := &v1alpha1.ApplicationSpec{
		Project: "default",
		Source: &v1alpha1.ApplicationSource{
			RepoURL: "https://example.com/repo",
			Helm:    &v1alpha1.ApplicationSourceHelm{Values: "helm-values-marker"},
		},
	}
	m.EXPECT().UpdateApplicationSpec(gomock.Any(), gomock.Any()).Return(spec, nil)
	updated, err := updateApplicationHandler(context.Background(), m, client.UpdateSpecOptions{
		Name: "guestbook", Spec: spec, Validate: true,
	})
	require.NoError(t, err)
	updateText := toolText(t, updated)
	assertOneLineJSON(t, updateText, &v1alpha1.ApplicationSpec{})
	assert.Contains(t, updateText, "helm-values-marker")

	project := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "team",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
			Annotations:   map[string]string{lastAppliedAnnotation: "blob", "keep-me": "yes"},
		},
	}
	m.EXPECT().CreateProject(gomock.Any(), gomock.Any(), false).Return(project, nil)
	createdProject, err := createProjectHandler(context.Background(), m, project, false)
	require.NoError(t, err)
	projectText := toolText(t, createdProject)
	assertOneLineJSON(t, projectText, &v1alpha1.AppProject{})
	assert.NotContains(t, projectText, "managedFields")
	assert.NotContains(t, projectText, lastAppliedAnnotation)
	assert.Contains(t, projectText, "keep-me")

	appSet := &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "set",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
			Annotations:   map[string]string{lastAppliedAnnotation: "blob", "keep-me": "yes"},
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Template: v1alpha1.ApplicationSetTemplate{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{
						Helm: &v1alpha1.ApplicationSourceHelm{Values: "helm-values-marker"},
					},
				},
			},
		},
	}
	m.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), false, false).Return(appSet, nil)
	createdSet, err := createApplicationSetHandler(
		context.Background(), m, "set", "argocd", "default",
		"[]", `{"metadata":{"name":"app"}}`, "", "", false, false, false,
	)
	require.NoError(t, err)
	setText := toolText(t, createdSet)
	assertOneLineJSON(t, setText, &v1alpha1.ApplicationSet{})
	assert.NotContains(t, setText, "managedFields")
	assert.NotContains(t, setText, lastAppliedAnnotation)
	assert.Contains(t, setText, "helm-values-marker")

	cluster := &v1alpha1.Cluster{
		Server: "https://kubernetes.default.svc",
		Name:   "in-cluster",
		Config: v1alpha1.ClusterConfig{BearerToken: "bearer-marker"},
		Annotations: map[string]string{
			lastAppliedAnnotation: "blob",
			"keep-me":             "yes",
		},
	}
	m.EXPECT().GetCluster(gomock.Any(), cluster.Server).Return(cluster, nil)
	gotCluster, err := getClusterHandler(context.Background(), m, cluster.Server)
	require.NoError(t, err)
	clusterText := toolText(t, gotCluster)
	assertOneLineJSON(t, clusterText, &v1alpha1.Cluster{})
	assert.NotContains(t, clusterText, "bearer-marker")
	assert.NotContains(t, clusterText, lastAppliedAnnotation)
	assert.Contains(t, clusterText, "keep-me")

	repo := &v1alpha1.Repository{Repo: "https://example.com/repo", Username: "git-user", Name: "demo"}
	m.EXPECT().GetRepository(gomock.Any(), repo.Repo).Return(repo, nil)
	gotRepo, err := getRepositoryHandler(context.Background(), m, repo.Repo)
	require.NoError(t, err)
	repoText := toolText(t, gotRepo)
	assertOneLineJSON(t, repoText, &v1alpha1.Repository{})
	assert.Contains(t, repoText, "git-user")
	assert.Contains(t, repoText, "https://example.com/repo")

	m.EXPECT().GetUserInfo(gomock.Any()).Return(&sessionpkg.GetUserInfoResponse{
		LoggedIn: true, Username: "ada", Iss: "argocd", Groups: []string{"admin"},
	}, nil)
	info, err := getUserInfoHandler(context.Background(), m)
	require.NoError(t, err)
	infoText := toolText(t, info)
	assertOneLineJSON(t, infoText, &UserInfo{})
	assert.Contains(t, infoText, `"username":"ada"`)
}

func mustSync(t *testing.T, m *mock.MockInterface) *mcp.CallToolResult {
	t.Helper()
	res, err := syncApplicationHandler(context.Background(), m, client.SyncOptions{Name: "guestbook"})
	require.NoError(t, err)
	return res
}

func noisyApplication() *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "guestbook",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
			Annotations:   map[string]string{lastAppliedAnnotation: "blob", "keep-me": "yes"},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://example.com/repo",
				Helm:    &v1alpha1.ApplicationSourceHelm{Values: "helm-values-marker"},
			},
		},
	}
}

func assertCompactApplication(t *testing.T, text string) {
	t.Helper()
	assertOneLineJSON(t, text, &v1alpha1.Application{})
	assert.NotContains(t, text, "managedFields")
	assert.NotContains(t, text, lastAppliedAnnotation)
	assert.Contains(t, text, "keep-me")
	assert.Contains(t, text, "helm-values-marker")
}

func assertOneLineJSON(t *testing.T, text string, dest any) {
	t.Helper()
	assert.NotContains(t, text, "\n")
	require.NoError(t, json.Unmarshal([]byte(text), dest))
}
