package client

import (
	"context"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	eventspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/events"
	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
)

//go:generate mockgen -source=interface.go -destination=mock/mock_client.go -package=mock

// LogStream is an interface for receiving log entries
type LogStream interface {
	Recv() (*applicationpkg.LogEntry, error)
}

// ListApplicationsOptions are the server-side filters for ListApplications.
// Destination cluster/namespace filtering is not supported by ApplicationQuery
// and is done by the caller.
type ListApplicationsOptions struct {
	Selector string
	// Projects limits results to these projects (ApplicationQuery.Projects).
	Projects []string
	// AppNamespace is the namespace of the Application resources (metadata.namespace).
	AppNamespace string
}

// EventsOptions are the parameters of GetApplicationEvents. Empty fields are
// not sent; AppNamespace and Project are looked up from the application when empty.
type EventsOptions struct {
	Name              string
	ResourceNamespace string
	ResourceName      string
	ResourceUID       string
	AppNamespace      string
	Project           string
}

// SyncResource selects one resource for a partial sync.
// Group is the API group and is empty for core resources such as ConfigMap.
type SyncResource struct {
	Group     string
	Kind      string
	Name      string
	Namespace string
}

// SyncOptions are the parameters of SyncApplication.
// Prune, DryRun, and Revision are always sent. An empty Revision tells Argo CD
// to use the revision in the Application spec. Revisions, Resources, and Options
// are sent only when non-empty. AppNamespace is sent only when non-empty.
//
// Revisions is one entry per source, in spec.sources order. The client pairs
// each entry with source position 1..N, because Argo CD applies
// ApplicationSyncRequest.Revisions only through SourcePositions.
// Options, when set, replace the Application's syncPolicy.syncOptions for this
// operation; they are not merged.
type SyncOptions struct {
	Name         string
	Revision     string
	Revisions    []string
	Prune        bool
	DryRun       bool
	AppNamespace string
	Resources    []SyncResource
	Options      []string
}

// LogsOptions are the parameters of GetApplicationLogs.
type LogsOptions struct {
	Name         string
	PodName      string
	Container    string
	Namespace    string
	ResourceName string
	Kind         string
	Group        string
	TailLines    int64
	SinceSeconds *int64
	Follow       bool
	Previous     bool
	Filter       string
	AppNamespace string
	Project      string
}

// Interface defines the contract for ArgoCD client operations
type Interface interface {
	// Application operations
	GetApplication(ctx context.Context, name string, appNamespace string) (*v1alpha1.Application, error)
	ListApplications(ctx context.Context, opts ListApplicationsOptions) (*v1alpha1.ApplicationList, error)
	CreateApplication(ctx context.Context, app *v1alpha1.Application, upsert bool) (*v1alpha1.Application, error)
	DeleteApplication(ctx context.Context, name string, cascade bool, appNamespace string) error
	SyncApplication(ctx context.Context, opts SyncOptions) (*v1alpha1.Application, error)
	RefreshApplication(ctx context.Context, name string, refreshType string, appNamespace string) (*v1alpha1.Application, error)
	GetApplicationManifests(ctx context.Context, name string, revision string, appNamespace string) (*repoapiclient.ManifestResponse, error)
	GetApplicationEvents(ctx context.Context, opts EventsOptions) (*eventspkg.EventList, error)
	GetApplicationLogs(ctx context.Context, opts LogsOptions) (LogStream, error)
	GetApplicationResourceTree(ctx context.Context, name string, appNamespace string, project string) (*v1alpha1.ApplicationTree, error)
	TerminateOperation(ctx context.Context, name string, appNamespace string, project string) error

	// Cluster operations
	ListClusters(ctx context.Context) (*v1alpha1.ClusterList, error)
	GetCluster(ctx context.Context, server string) (*v1alpha1.Cluster, error)

	// Project operations
	ListProjects(ctx context.Context) (*v1alpha1.AppProjectList, error)
	GetProject(ctx context.Context, name string) (*v1alpha1.AppProject, error)
	CreateProject(ctx context.Context, project *v1alpha1.AppProject, upsert bool) (*v1alpha1.AppProject, error)

	// Repository operations
	ListRepositories(ctx context.Context) (*v1alpha1.RepositoryList, error)
	GetRepository(ctx context.Context, repo string) (*v1alpha1.Repository, error)

	// ApplicationSet operations
	ListApplicationSets(ctx context.Context, project string) (*v1alpha1.ApplicationSetList, error)
	GetApplicationSet(ctx context.Context, name string) (*v1alpha1.ApplicationSet, error)
	CreateApplicationSet(ctx context.Context, appSet *v1alpha1.ApplicationSet, upsert bool, dryRun bool) (*v1alpha1.ApplicationSet, error)
	DeleteApplicationSet(ctx context.Context, name string, appNamespace string) error

	// Session operations
	GetUserInfo(ctx context.Context) (*sessionpkg.GetUserInfoResponse, error)

	// Connection management
	Close() error
}

// Ensure Client implements Interface
var _ Interface = (*Client)(nil)
