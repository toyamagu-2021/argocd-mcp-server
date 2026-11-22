package client

import (
	"context"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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
	SyncApplication(ctx context.Context, name string, revision string, prune bool, dryRun bool, appNamespace string) (*v1alpha1.Application, error)
	RefreshApplication(ctx context.Context, name string, refreshType string, appNamespace string) (*v1alpha1.Application, error)
	GetApplicationManifests(ctx context.Context, name string, revision string, appNamespace string) (interface{}, error)
	GetApplicationEvents(ctx context.Context, name string, resourceNamespace string, resourceName string, resourceUID string, appNamespace string, project string) (interface{}, error)
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
