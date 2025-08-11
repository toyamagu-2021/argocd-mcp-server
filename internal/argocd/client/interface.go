package client

import (
	"context"

	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

//go:generate mockgen -source=interface.go -destination=mock/mock_client.go -package=mock

// LogStream is an interface for receiving log entries
type LogStream interface {
	Recv() (*applicationpkg.LogEntry, error)
}

// Interface defines the contract for ArgoCD client operations
type Interface interface {
	// Application operations
	GetApplication(ctx context.Context, name string) (*v1alpha1.Application, error)
	ListApplications(ctx context.Context, selector string) (*v1alpha1.ApplicationList, error)
	CreateApplication(ctx context.Context, app *v1alpha1.Application, upsert bool) (*v1alpha1.Application, error)
	UpdateApplication(ctx context.Context, app *v1alpha1.Application) (*v1alpha1.Application, error)
	DeleteApplication(ctx context.Context, name string, cascade bool) error
	SyncApplication(ctx context.Context, name string, revision string, prune bool, dryRun bool) (*v1alpha1.Application, error)
	RollbackApplication(ctx context.Context, name string, id int64) (*v1alpha1.Application, error)
	GetApplicationManifests(ctx context.Context, name string, revision string) (interface{}, error)
	GetApplicationEvents(ctx context.Context, name string, resourceNamespace string, resourceName string, resourceUID string, appNamespace string, project string) (interface{}, error)
	GetApplicationLogs(ctx context.Context, name string, podName string, container string, namespace string, resourceName string, kind string, group string, tailLines int64, sinceSeconds *int64, follow bool, previous bool, filter string, appNamespace string, project string) (LogStream, error)

	// Cluster operations
	ListClusters(ctx context.Context) (*v1alpha1.ClusterList, error)
	GetCluster(ctx context.Context, server string) (*v1alpha1.Cluster, error)
	CreateCluster(ctx context.Context, cluster *v1alpha1.Cluster, upsert bool) (*v1alpha1.Cluster, error)
	UpdateCluster(ctx context.Context, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error)
	DeleteCluster(ctx context.Context, server string) error

	// Project operations
	ListProjects(ctx context.Context) (*v1alpha1.AppProjectList, error)
	GetProject(ctx context.Context, name string) (*v1alpha1.AppProject, error)
	CreateProject(ctx context.Context, project *v1alpha1.AppProject, upsert bool) (*v1alpha1.AppProject, error)
	UpdateProject(ctx context.Context, project *v1alpha1.AppProject) (*v1alpha1.AppProject, error)
	DeleteProject(ctx context.Context, name string) error

	// Repository operations
	ListRepositories(ctx context.Context) (*v1alpha1.RepositoryList, error)
	GetRepository(ctx context.Context, repo string) (*v1alpha1.Repository, error)
	CreateRepository(ctx context.Context, repo *v1alpha1.Repository, upsert bool) (*v1alpha1.Repository, error)
	UpdateRepository(ctx context.Context, repo *v1alpha1.Repository) (*v1alpha1.Repository, error)
	DeleteRepository(ctx context.Context, repo string) error

	// Connection management
	Close() error
}

// Ensure Client implements Interface
var _ Interface = (*Client)(nil)
