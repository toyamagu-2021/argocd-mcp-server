package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"

	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	applicationsetpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	projectpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	repositorypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repository"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/grpcwebproxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Client provides a gRPC client for ArgoCD server operations
type Client struct {
	config     *Config
	conn       *grpc.ClientConn
	httpClient *http.Client

	// gRPC-Web proxy support
	grpcWebProxy *grpcwebproxy.GRPCWebProxy
	proxyCloser  io.Closer

	// Service clients
	appClient     applicationpkg.ApplicationServiceClient
	appSetClient  applicationsetpkg.ApplicationSetServiceClient
	clusterClient clusterpkg.ClusterServiceClient
	projectClient projectpkg.ProjectServiceClient
	repoClient    repositorypkg.RepositoryServiceClient
}

// New creates a new ArgoCD gRPC client with the provided configuration
func New(config *Config) (*Client, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	client := &Client{
		config:     config,
		httpClient: config.NewHTTPClient(),
	}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return client, nil
}

func (c *Client) connect() error {
	var opts []grpc.DialOption

	// Add authentication
	opts = append(opts, grpc.WithPerRPCCredentials(newJWTCredentials(c.config.AuthToken)))

	var serverAddr string = c.config.ServerAddr

	// Use gRPC-Web proxy if enabled
	if c.config.GRPCWeb {
		// Create gRPC-Web proxy
		c.grpcWebProxy = grpcwebproxy.NewGRPCWebProxy(
			c.config.ServerAddr,
			c.config.PlainText,
			c.httpClient,
			c.config.GRPCWebRootPath,
			c.config.Headers,
		)

		// Start proxy and get Unix socket address
		addr, closer, err := c.grpcWebProxy.UseProxy()
		if err != nil {
			return fmt.Errorf("failed to start gRPC-Web proxy: %w", err)
		}

		c.proxyCloser = closer
		// Unix socket addresses need the unix:// scheme for gRPC dialing
		serverAddr = fmt.Sprintf("unix://%s", addr.String())

		// Force Unix socket connection to be insecure
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		// Configure TLS for direct gRPC connection
		if c.config.PlainText {
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			var tlsConfig *tls.Config
			if c.config.Insecure {
				tlsConfig = &tls.Config{
					InsecureSkipVerify: true,
				}
			} else {
				tlsConfig = &tls.Config{}
			}

			if c.config.ClientCertFile != "" && c.config.ClientCertKeyFile != "" {
				cert, err := tls.LoadX509KeyPair(c.config.ClientCertFile, c.config.ClientCertKeyFile)
				if err != nil {
					return fmt.Errorf("failed to load client certificates: %w", err)
				}
				tlsConfig.Certificates = []tls.Certificate{cert}
			}

			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		}
	}

	// Add user agent
	if c.config.UserAgent != "" {
		opts = append(opts, grpc.WithUserAgent(c.config.UserAgent))
	}

	// Establish connection
	conn, err := grpc.NewClient(serverAddr, opts...)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	c.conn = conn

	// Initialize service clients
	c.appClient = applicationpkg.NewApplicationServiceClient(conn)
	c.appSetClient = applicationsetpkg.NewApplicationSetServiceClient(conn)
	c.clusterClient = clusterpkg.NewClusterServiceClient(conn)
	c.projectClient = projectpkg.NewProjectServiceClient(conn)
	c.repoClient = repositorypkg.NewRepositoryServiceClient(conn)

	return nil
}

// Close closes the gRPC connection and any associated resources
func (c *Client) Close() error {
	var err error

	// Close gRPC connection
	if c.conn != nil {
		if connErr := c.conn.Close(); connErr != nil {
			err = connErr
		}
	}

	// Close gRPC-Web proxy if running
	if c.proxyCloser != nil {
		if proxyErr := c.proxyCloser.Close(); proxyErr != nil && err == nil {
			err = proxyErr
		}
	}

	return err
}

// Application operations

// GetApplication retrieves a single ArgoCD application by name
func (c *Client) GetApplication(ctx context.Context, name string) (*v1alpha1.Application, error) {
	req := &applicationpkg.ApplicationQuery{
		Name: &name,
	}
	resp, err := c.appClient.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get application: %w", err)
	}
	return resp, nil
}

// ListApplications retrieves all ArgoCD applications with optional selector filtering
func (c *Client) ListApplications(ctx context.Context, selector string) (*v1alpha1.ApplicationList, error) {
	req := &applicationpkg.ApplicationQuery{}
	if selector != "" {
		req.Selector = &selector
	}
	resp, err := c.appClient.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}
	return resp, nil
}

// CreateApplication creates a new ArgoCD application
func (c *Client) CreateApplication(ctx context.Context, app *v1alpha1.Application, upsert bool) (*v1alpha1.Application, error) {
	validate := true
	req := &applicationpkg.ApplicationCreateRequest{
		Application: app,
		Upsert:      &upsert,
		Validate:    &validate,
	}
	resp, err := c.appClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create application: %w", err)
	}
	return resp, nil
}

// UpdateApplication updates an existing ArgoCD application
func (c *Client) UpdateApplication(ctx context.Context, app *v1alpha1.Application) (*v1alpha1.Application, error) {
	req := &applicationpkg.ApplicationUpdateRequest{
		Application: app,
	}
	resp, err := c.appClient.Update(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update application: %w", err)
	}
	return resp, nil
}

// DeleteApplication deletes an ArgoCD application
func (c *Client) DeleteApplication(ctx context.Context, name string, cascade bool) error {
	req := &applicationpkg.ApplicationDeleteRequest{
		Name:    &name,
		Cascade: &cascade,
	}
	_, err := c.appClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete application: %w", err)
	}
	return nil
}

// SyncApplication triggers a sync operation for an ArgoCD application
func (c *Client) SyncApplication(ctx context.Context, name string, revision string, prune bool, dryRun bool) (*v1alpha1.Application, error) {
	strategy := &v1alpha1.SyncStrategy{}
	req := &applicationpkg.ApplicationSyncRequest{
		Name:     &name,
		Revision: &revision,
		Prune:    &prune,
		DryRun:   &dryRun,
		Strategy: strategy,
	}
	resp, err := c.appClient.Sync(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to sync application: %w", err)
	}
	return resp, nil
}

// GetApplicationManifests retrieves the rendered manifests of an ArgoCD application
func (c *Client) GetApplicationManifests(ctx context.Context, name string, revision string) (interface{}, error) {
	// First get the application to retrieve its namespace and project
	// This is required for proper authorization in gRPC-Web mode
	appReq := &applicationpkg.ApplicationQuery{
		Name: &name,
	}
	app, err := c.appClient.Get(ctx, appReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get application details: %w", err)
	}

	// Now request manifests with the proper namespace and project
	namespace := app.ObjectMeta.Namespace
	project := app.Spec.Project

	req := &applicationpkg.ApplicationManifestQuery{
		Name:         &name,
		Revision:     &revision,
		AppNamespace: &namespace,
		Project:      &project,
	}
	resp, err := c.appClient.GetManifests(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get application manifests: %w", err)
	}
	return resp, nil
}

// GetApplicationEvents retrieves Kubernetes events for resources belonging to an ArgoCD application
func (c *Client) GetApplicationEvents(ctx context.Context, name string, resourceNamespace string, resourceName string, resourceUID string, appNamespace string, project string) (interface{}, error) {
	// Build the query with optional filters
	req := &applicationpkg.ApplicationResourceEventsQuery{
		Name: &name,
	}

	// Add optional filters if provided
	if resourceNamespace != "" {
		req.ResourceNamespace = &resourceNamespace
	}
	if resourceName != "" {
		req.ResourceName = &resourceName
	}
	if resourceUID != "" {
		req.ResourceUID = &resourceUID
	}
	if appNamespace != "" {
		req.AppNamespace = &appNamespace
	}
	if project != "" {
		req.Project = &project
	}

	// If appNamespace or project not provided, get them from the application
	if appNamespace == "" || project == "" {
		appReq := &applicationpkg.ApplicationQuery{
			Name: &name,
		}
		app, err := c.appClient.Get(ctx, appReq)
		if err != nil {
			return nil, fmt.Errorf("failed to get application details: %w", err)
		}

		if appNamespace == "" {
			namespace := app.ObjectMeta.Namespace
			req.AppNamespace = &namespace
		}
		if project == "" {
			projectName := app.Spec.Project
			req.Project = &projectName
		}
	}

	resp, err := c.appClient.ListResourceEvents(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get application events: %w", err)
	}
	return resp, nil
}

// RollbackApplication rolls back an ArgoCD application to a previous revision
func (c *Client) RollbackApplication(ctx context.Context, name string, id int64) (*v1alpha1.Application, error) {
	req := &applicationpkg.ApplicationRollbackRequest{
		Name: &name,
		Id:   &id,
	}
	resp, err := c.appClient.Rollback(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to rollback application: %w", err)
	}
	return resp, nil
}

// GetApplicationLogs retrieves logs from pods in an ArgoCD application
func (c *Client) GetApplicationLogs(ctx context.Context, name string, podName string, container string, namespace string, resourceName string, kind string, group string, tailLines int64, sinceSeconds *int64, follow bool, previous bool, filter string, appNamespace string, project string) (LogStream, error) {
	// Build the query
	req := &applicationpkg.ApplicationPodLogsQuery{
		Name:      &name,
		TailLines: &tailLines,
		Follow:    &follow,
		Previous:  &previous,
	}

	// Add optional parameters
	if podName != "" {
		req.PodName = &podName
	}
	if container != "" {
		req.Container = &container
	}
	if namespace != "" {
		req.Namespace = &namespace
	}
	if resourceName != "" {
		req.ResourceName = &resourceName
	}
	if kind != "" {
		req.Kind = &kind
	}
	if group != "" {
		req.Group = &group
	}
	if filter != "" {
		req.Filter = &filter
	}
	if sinceSeconds != nil {
		req.SinceSeconds = sinceSeconds
	}

	// If appNamespace or project not provided, get them from the application
	if appNamespace == "" || project == "" {
		appReq := &applicationpkg.ApplicationQuery{
			Name: &name,
		}
		app, err := c.appClient.Get(ctx, appReq)
		if err != nil {
			return nil, fmt.Errorf("failed to get application for logs: %w", err)
		}
		if appNamespace == "" {
			appNamespace = app.ObjectMeta.Namespace
		}
		if project == "" {
			project = app.Spec.Project
		}
	}

	// Set namespace and project for proper authorization
	if appNamespace != "" {
		req.AppNamespace = &appNamespace
	}
	if project != "" {
		req.Project = &project
	}

	// Get the log stream
	stream, err := c.appClient.PodLogs(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get application logs: %w", err)
	}

	return stream, nil
}

// Cluster operations

// ListClusters retrieves all ArgoCD clusters
func (c *Client) ListClusters(ctx context.Context) (*v1alpha1.ClusterList, error) {
	req := &clusterpkg.ClusterQuery{}
	resp, err := c.clusterClient.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}
	return resp, nil
}

// GetCluster retrieves a single ArgoCD cluster by server address
func (c *Client) GetCluster(ctx context.Context, server string) (*v1alpha1.Cluster, error) {
	// URL decode the server name
	server = strings.ReplaceAll(server, "%2F", "/")
	req := &clusterpkg.ClusterQuery{
		Server: server,
	}
	resp, err := c.clusterClient.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}
	return resp, nil
}

// CreateCluster creates a new ArgoCD cluster
func (c *Client) CreateCluster(ctx context.Context, cluster *v1alpha1.Cluster, upsert bool) (*v1alpha1.Cluster, error) {
	req := &clusterpkg.ClusterCreateRequest{
		Cluster: cluster,
		Upsert:  upsert,
	}
	resp, err := c.clusterClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}
	return resp, nil
}

// UpdateCluster updates an existing ArgoCD cluster
func (c *Client) UpdateCluster(ctx context.Context, cluster *v1alpha1.Cluster) (*v1alpha1.Cluster, error) {
	req := &clusterpkg.ClusterUpdateRequest{
		Cluster: cluster,
	}
	resp, err := c.clusterClient.Update(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update cluster: %w", err)
	}
	return resp, nil
}

// DeleteCluster deletes an ArgoCD cluster by server address
func (c *Client) DeleteCluster(ctx context.Context, server string) error {
	req := &clusterpkg.ClusterQuery{
		Server: server,
	}
	_, err := c.clusterClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}
	return nil
}

// Project operations

// ListProjects retrieves all ArgoCD projects
func (c *Client) ListProjects(ctx context.Context) (*v1alpha1.AppProjectList, error) {
	req := &projectpkg.ProjectQuery{}
	resp, err := c.projectClient.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	return resp, nil
}

// GetProject retrieves a single ArgoCD project by name
func (c *Client) GetProject(ctx context.Context, name string) (*v1alpha1.AppProject, error) {
	req := &projectpkg.ProjectQuery{
		Name: name,
	}
	resp, err := c.projectClient.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return resp, nil
}

// CreateProject creates a new ArgoCD project
func (c *Client) CreateProject(ctx context.Context, project *v1alpha1.AppProject, upsert bool) (*v1alpha1.AppProject, error) {
	req := &projectpkg.ProjectCreateRequest{
		Project: project,
		Upsert:  upsert,
	}
	resp, err := c.projectClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}
	return resp, nil
}

// UpdateProject updates an existing ArgoCD project
func (c *Client) UpdateProject(ctx context.Context, project *v1alpha1.AppProject) (*v1alpha1.AppProject, error) {
	req := &projectpkg.ProjectUpdateRequest{
		Project: project,
	}
	resp, err := c.projectClient.Update(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update project: %w", err)
	}
	return resp, nil
}

// DeleteProject deletes an ArgoCD project by name
func (c *Client) DeleteProject(ctx context.Context, name string) error {
	req := &projectpkg.ProjectQuery{
		Name: name,
	}
	_, err := c.projectClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	return nil
}

// Repository operations

// ListRepositories retrieves all ArgoCD repositories
func (c *Client) ListRepositories(ctx context.Context) (*v1alpha1.RepositoryList, error) {
	req := &repositorypkg.RepoQuery{}
	resp, err := c.repoClient.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	return resp, nil
}

// GetRepository retrieves a single ArgoCD repository by URL
func (c *Client) GetRepository(ctx context.Context, repo string) (*v1alpha1.Repository, error) {
	req := &repositorypkg.RepoQuery{
		Repo: repo,
	}
	resp, err := c.repoClient.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	return resp, nil
}

// CreateRepository creates a new ArgoCD repository
func (c *Client) CreateRepository(ctx context.Context, repo *v1alpha1.Repository, upsert bool) (*v1alpha1.Repository, error) {
	req := &repositorypkg.RepoCreateRequest{
		Repo:   repo,
		Upsert: upsert,
	}
	resp, err := c.repoClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}
	return resp, nil
}

// UpdateRepository updates an existing ArgoCD repository
func (c *Client) UpdateRepository(ctx context.Context, repo *v1alpha1.Repository) (*v1alpha1.Repository, error) {
	req := &repositorypkg.RepoUpdateRequest{
		Repo: repo,
	}
	resp, err := c.repoClient.Update(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update repository: %w", err)
	}
	return resp, nil
}

// DeleteRepository deletes an ArgoCD repository by URL
func (c *Client) DeleteRepository(ctx context.Context, repo string) error {
	req := &repositorypkg.RepoQuery{
		Repo: repo,
	}
	_, err := c.repoClient.Delete(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}
	return nil
}

// ListApplicationSets lists all ArgoCD ApplicationSets, optionally filtered by project
func (c *Client) ListApplicationSets(ctx context.Context, project string) (*v1alpha1.ApplicationSetList, error) {
	req := &applicationsetpkg.ApplicationSetListQuery{}

	// Add project filter if specified
	if project != "" {
		req.Projects = []string{project}
	}

	resp, err := c.appSetClient.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to list applicationsets: %w", err)
	}
	return resp, nil
}

// GetApplicationSet retrieves an ArgoCD ApplicationSet by name
func (c *Client) GetApplicationSet(ctx context.Context, name string) (*v1alpha1.ApplicationSet, error) {
	req := &applicationsetpkg.ApplicationSetGetQuery{
		Name: name,
	}
	resp, err := c.appSetClient.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get applicationset: %w", err)
	}
	return resp, nil
}

// CreateApplicationSet creates a new ArgoCD ApplicationSet
func (c *Client) CreateApplicationSet(ctx context.Context, appSet *v1alpha1.ApplicationSet, upsert bool, dryRun bool) (*v1alpha1.ApplicationSet, error) {
	req := &applicationsetpkg.ApplicationSetCreateRequest{
		Applicationset: appSet,
		Upsert:         upsert,
		DryRun:         dryRun,
	}
	resp, err := c.appSetClient.Create(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create applicationset: %w", err)
	}
	return resp, nil
}
