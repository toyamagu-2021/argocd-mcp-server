package client

import (
	"context"
	"testing"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	applicationsetpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/applicationset"
	eventspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeAppService records the requests sent to the Argo CD application service.
type fakeAppService struct {
	applicationpkg.ApplicationServiceClient
	gets      []*applicationpkg.ApplicationQuery
	list      *applicationpkg.ApplicationQuery
	del       *applicationpkg.ApplicationDeleteRequest
	sync      *applicationpkg.ApplicationSyncRequest
	manifests *applicationpkg.ApplicationManifestQuery
	logs      *applicationpkg.ApplicationPodLogsQuery
	events    *applicationpkg.ApplicationResourceEventsQuery
	managed   *applicationpkg.ResourcesQuery
	resource  *applicationpkg.ApplicationResourceRequest
}

func (f *fakeAppService) ListResourceEvents(_ context.Context, q *applicationpkg.ApplicationResourceEventsQuery, _ ...grpc.CallOption) (*eventspkg.EventList, error) {
	f.events = q
	return &eventspkg.EventList{}, nil
}

func (f *fakeAppService) Get(_ context.Context, q *applicationpkg.ApplicationQuery, _ ...grpc.CallOption) (*v1alpha1.Application, error) {
	f.gets = append(f.gets, q)
	ns := "argocd"
	if q.AppNamespace != nil {
		ns = *q.AppNamespace
	}
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: q.GetName(), Namespace: ns},
		Spec:       v1alpha1.ApplicationSpec{Project: "proj"},
	}, nil
}

func (f *fakeAppService) List(_ context.Context, q *applicationpkg.ApplicationQuery, _ ...grpc.CallOption) (*v1alpha1.ApplicationList, error) {
	f.list = q
	return &v1alpha1.ApplicationList{}, nil
}

func (f *fakeAppService) Delete(_ context.Context, r *applicationpkg.ApplicationDeleteRequest, _ ...grpc.CallOption) (*applicationpkg.ApplicationResponse, error) {
	f.del = r
	return &applicationpkg.ApplicationResponse{}, nil
}

func (f *fakeAppService) Sync(_ context.Context, r *applicationpkg.ApplicationSyncRequest, _ ...grpc.CallOption) (*v1alpha1.Application, error) {
	f.sync = r
	return &v1alpha1.Application{}, nil
}

func (f *fakeAppService) GetManifests(_ context.Context, q *applicationpkg.ApplicationManifestQuery, _ ...grpc.CallOption) (*repoapiclient.ManifestResponse, error) {
	f.manifests = q
	return nil, nil
}

func (f *fakeAppService) ManagedResources(_ context.Context, q *applicationpkg.ResourcesQuery, _ ...grpc.CallOption) (*applicationpkg.ManagedResourcesResponse, error) {
	f.managed = q
	return &applicationpkg.ManagedResourcesResponse{}, nil
}

func (f *fakeAppService) GetResource(_ context.Context, q *applicationpkg.ApplicationResourceRequest, _ ...grpc.CallOption) (*applicationpkg.ApplicationResourceResponse, error) {
	f.resource = q
	manifest := `{"kind":"Deployment"}`
	return &applicationpkg.ApplicationResourceResponse{Manifest: &manifest}, nil
}

func (f *fakeAppService) PodLogs(_ context.Context, q *applicationpkg.ApplicationPodLogsQuery, _ ...grpc.CallOption) (applicationpkg.ApplicationService_PodLogsClient, error) {
	f.logs = q
	return nil, nil
}

type fakeAppSetService struct {
	applicationsetpkg.ApplicationSetServiceClient
	del *applicationsetpkg.ApplicationSetDeleteRequest
}

func (f *fakeAppSetService) Delete(_ context.Context, r *applicationsetpkg.ApplicationSetDeleteRequest, _ ...grpc.CallOption) (*applicationsetpkg.ApplicationSetResponse, error) {
	f.del = r
	return &applicationsetpkg.ApplicationSetResponse{}, nil
}

func newFakeClient() (*Client, *fakeAppService, *fakeAppSetService) {
	a, s := &fakeAppService{}, &fakeAppSetService{}
	return &Client{appClient: a, appSetClient: s}, a, s
}

func TestListApplications_Query(t *testing.T) {
	ctx := context.Background()
	c, f, _ := newFakeClient()

	_, err := c.ListApplications(ctx, ListApplicationsOptions{})
	require.NoError(t, err)
	assert.Nil(t, f.list.Selector)
	assert.Empty(t, f.list.Projects)
	assert.Nil(t, f.list.AppNamespace)

	_, err = c.ListApplications(ctx, ListApplicationsOptions{Selector: "a=b", Projects: []string{"p1"}, AppNamespace: "team-a"})
	require.NoError(t, err)
	assert.Equal(t, "a=b", f.list.GetSelector())
	assert.Equal(t, []string{"p1"}, f.list.Projects)
	// D14: only Projects is used; the singular Project field must stay unset.
	assert.Empty(t, f.list.Project)
	assert.Equal(t, "team-a", f.list.GetAppNamespace())
}

func TestAppNamespace_Propagation(t *testing.T) {
	ctx := context.Background()

	t.Run("get", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplication(ctx, "app", "team-a")
		require.NoError(t, err)
		assert.Equal(t, "team-a", f.gets[0].GetAppNamespace())

		_, err = c.GetApplication(ctx, "app", "")
		require.NoError(t, err)
		assert.Nil(t, f.gets[1].AppNamespace)
	})

	t.Run("refresh", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.RefreshApplication(ctx, "app", "hard", "team-a")
		require.NoError(t, err)
		assert.Equal(t, "team-a", f.gets[0].GetAppNamespace())
		assert.Equal(t, "hard", f.gets[0].GetRefresh())
	})

	t.Run("delete", func(t *testing.T) {
		c, f, _ := newFakeClient()
		require.NoError(t, c.DeleteApplication(ctx, "app", false, "team-a"))
		assert.Equal(t, "team-a", f.del.GetAppNamespace())
		assert.False(t, f.del.GetCascade())
	})

	t.Run("sync", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.SyncApplication(ctx, SyncOptions{Name: "app", Prune: true, DryRun: true, AppNamespace: "team-a"})
		require.NoError(t, err)
		assert.Equal(t, "team-a", f.sync.GetAppNamespace())
		assert.Equal(t, "", f.sync.GetRevision())
		require.NotNil(t, f.sync.Revision)
		assert.Nil(t, f.sync.Resources)
		assert.Nil(t, f.sync.Revisions)
		assert.Nil(t, f.sync.SourcePositions)
		assert.Nil(t, f.sync.SyncOptions)
		require.NotNil(t, f.sync.Strategy)
		assert.Nil(t, f.sync.Strategy.Apply)
		assert.Nil(t, f.sync.Strategy.Hook)
	})

	t.Run("manifests looks up the app in its namespace", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplicationManifests(ctx, "app", "", "team-a")
		require.NoError(t, err)
		assert.Equal(t, "team-a", f.gets[0].GetAppNamespace())
		assert.Equal(t, "team-a", f.manifests.GetAppNamespace())
		assert.Equal(t, "proj", f.manifests.GetProject())
	})

	t.Run("logs", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplicationLogs(ctx, LogsOptions{Name: "app", TailLines: 10, Container: "c", AppNamespace: "team-a"})
		require.NoError(t, err)
		assert.Equal(t, "team-a", f.gets[0].GetAppNamespace())
		assert.Equal(t, "team-a", f.logs.GetAppNamespace())
		assert.Equal(t, "proj", f.logs.GetProject())
		assert.Equal(t, "c", f.logs.GetContainer())
		assert.Nil(t, f.logs.PodName)
		assert.Equal(t, int64(10), f.logs.GetTailLines())
	})

	t.Run("logs with namespace and project skips lookup", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplicationLogs(ctx, LogsOptions{Name: "app", AppNamespace: "ns", Project: "p"})
		require.NoError(t, err)
		assert.Empty(t, f.gets)
	})

	t.Run("events: filters, lookup in app namespace", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplicationEvents(ctx, EventsOptions{Name: "app", ResourceName: "pod-1", AppNamespace: "team-a"})
		require.NoError(t, err)
		require.Len(t, f.gets, 1)
		assert.Equal(t, "team-a", f.gets[0].GetAppNamespace())
		assert.Equal(t, "pod-1", f.events.GetResourceName())
		assert.Nil(t, f.events.ResourceNamespace)
		assert.Nil(t, f.events.ResourceUID)
		assert.Equal(t, "team-a", f.events.GetAppNamespace())
		assert.Equal(t, "proj", f.events.GetProject())
	})

	t.Run("events with namespace and project skips lookup", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplicationEvents(ctx, EventsOptions{Name: "app", AppNamespace: "ns", Project: "p"})
		require.NoError(t, err)
		assert.Empty(t, f.gets)
		assert.Equal(t, "p", f.events.GetProject())
	})

	t.Run("managed resources looks up namespace and project", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplicationManagedResources(ctx, ManagedResourcesOptions{
			Name: "app", Kind: "Deployment", AppNamespace: "team-a",
		})
		require.NoError(t, err)
		require.Len(t, f.gets, 1)
		assert.Equal(t, "team-a", f.gets[0].GetAppNamespace())
		assert.Equal(t, "app", f.managed.GetApplicationName())
		assert.Equal(t, "Deployment", f.managed.GetKind())
		assert.Nil(t, f.managed.Namespace)
		assert.Nil(t, f.managed.Name)
		assert.Nil(t, f.managed.Group)
		assert.Nil(t, f.managed.Version)
		assert.Equal(t, "team-a", f.managed.GetAppNamespace())
		assert.Equal(t, "proj", f.managed.GetProject())
	})

	t.Run("managed resources skips lookup when namespace and project are set", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplicationManagedResources(ctx, ManagedResourcesOptions{
			Name: "app", AppNamespace: "ns", Project: "p", ResourceName: "web",
		})
		require.NoError(t, err)
		assert.Empty(t, f.gets)
		assert.Equal(t, "web", f.managed.GetName())
		assert.Equal(t, "p", f.managed.GetProject())
	})

	t.Run("get resource", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.GetApplicationResource(ctx, ResourceQuery{
			Name: "app", ResourceName: "web", Version: "v1", Group: "apps", Kind: "Deployment",
			ResourceNamespace: "prod", AppNamespace: "team-a", Project: "proj",
		})
		require.NoError(t, err)
		assert.Empty(t, f.gets)
		assert.Equal(t, "app", f.resource.GetName())
		assert.Equal(t, "web", f.resource.GetResourceName())
		assert.Equal(t, "v1", f.resource.GetVersion())
		assert.Equal(t, "apps", f.resource.GetGroup())
		assert.Equal(t, "Deployment", f.resource.GetKind())
		assert.Equal(t, "prod", f.resource.GetNamespace())
		assert.Equal(t, "team-a", f.resource.GetAppNamespace())
		assert.Equal(t, "proj", f.resource.GetProject())
	})

	t.Run("sync forwards revision, partial resources, multi-source revisions, and sync options", func(t *testing.T) {
		c, f, _ := newFakeClient()
		_, err := c.SyncApplication(ctx, SyncOptions{
			Name:      "app",
			Revision:  "abc123",
			Revisions: []string{"v1.2.3", ""},
			Resources: []SyncResource{{
				Group: "apps", Kind: "Deployment", Name: "web", Namespace: "prod",
			}},
			Options: []string{"CreateNamespace=true"},
		})
		require.NoError(t, err)
		assert.Equal(t, "abc123", f.sync.GetRevision())
		assert.Equal(t, []string{"v1.2.3", ""}, f.sync.GetRevisions())
		assert.Equal(t, []int64{1, 2}, f.sync.GetSourcePositions())
		require.Len(t, f.sync.GetResources(), 1)
		assert.Equal(t, "apps", f.sync.Resources[0].Group)
		assert.Equal(t, "Deployment", f.sync.Resources[0].Kind)
		assert.Equal(t, "web", f.sync.Resources[0].Name)
		assert.Equal(t, "prod", f.sync.Resources[0].Namespace)
		require.NotNil(t, f.sync.SyncOptions)
		assert.Equal(t, []string{"CreateNamespace=true"}, f.sync.SyncOptions.Items)
		assert.Nil(t, f.sync.AppNamespace)
	})

	t.Run("applicationset delete", func(t *testing.T) {
		c, _, s := newFakeClient()
		require.NoError(t, c.DeleteApplicationSet(ctx, "set", "team-a"))
		assert.Equal(t, "team-a", s.del.AppsetNamespace)
	})
}
