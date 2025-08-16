package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	repository "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mockApplicationService struct {
	application.UnimplementedApplicationServiceServer
}

func (s *mockApplicationService) List(ctx context.Context, req *application.ApplicationQuery) (*v1alpha1.ApplicationList, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	apps := &v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-1",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/test/repo1",
						Path:           "manifests",
						TargetRevision: "main",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
				Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.HealthStatus{
						Status:  "Healthy",
						Message: "All resources are healthy",
					},
					Sync: v1alpha1.SyncStatus{
						Status:   "Synced",
						Revision: "abc123",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app-2",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "production",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/test/repo2",
						Path:           "charts/app",
						TargetRevision: "v1.0.0",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://production.cluster.local",
						Namespace: "prod",
					},
				},
				Status: v1alpha1.ApplicationStatus{
					Health: v1alpha1.HealthStatus{
						Status:  "Progressing",
						Message: "Deployment is progressing",
					},
					Sync: v1alpha1.SyncStatus{
						Status:   "OutOfSync",
						Revision: "def456",
					},
				},
			},
		},
	}

	if req.Name != nil && *req.Name != "" {
		filtered := []v1alpha1.Application{}
		for _, app := range apps.Items {
			if app.Name == *req.Name {
				filtered = append(filtered, app)
			}
		}
		apps.Items = filtered
	}

	if len(req.Projects) > 0 {
		filtered := []v1alpha1.Application{}
		for _, app := range apps.Items {
			for _, proj := range req.Projects {
				if app.Spec.Project == proj {
					filtered = append(filtered, app)
					break
				}
			}
		}
		apps.Items = filtered
	}

	return apps, nil
}

func (s *mockApplicationService) Get(ctx context.Context, req *application.ApplicationQuery) (*v1alpha1.Application, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Name == nil || *req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "application name is required")
	}

	switch *req.Name {
	case "test-app-1":
		return &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app-1",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/test/repo1",
					Path:           "manifests",
					TargetRevision: "main",
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "default",
				},
				SyncPolicy: &v1alpha1.SyncPolicy{
					Automated: &v1alpha1.SyncPolicyAutomated{
						Prune:    true,
						SelfHeal: true,
					},
				},
			},
			Status: v1alpha1.ApplicationStatus{
				Health: v1alpha1.HealthStatus{
					Status:  "Healthy",
					Message: "All resources are healthy",
				},
				Sync: v1alpha1.SyncStatus{
					Status:   "Synced",
					Revision: "abc123",
				},
				Resources: []v1alpha1.ResourceStatus{
					{
						Name:      "test-deployment",
						Kind:      "Deployment",
						Namespace: "default",
						Status:    "Synced",
						Health: &v1alpha1.HealthStatus{
							Status: "Healthy",
						},
					},
				},
			},
		}, nil
	case "test-app-2":
		return &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-app-2",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "production",
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/test/repo2",
					Path:           "charts/app",
					TargetRevision: "v1.0.0",
				},
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://production.cluster.local",
					Namespace: "prod",
				},
			},
			Status: v1alpha1.ApplicationStatus{
				Health: v1alpha1.HealthStatus{
					Status:  "Progressing",
					Message: "Deployment is progressing",
				},
				Sync: v1alpha1.SyncStatus{
					Status:   "OutOfSync",
					Revision: "def456",
				},
			},
		}, nil
	default:
		return nil, status.Error(codes.NotFound, fmt.Sprintf("application %s not found", *req.Name))
	}
}

func (s *mockApplicationService) Sync(ctx context.Context, req *application.ApplicationSyncRequest) (*v1alpha1.Application, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Name == nil || *req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "application name is required")
	}

	if *req.Name != "test-app-1" && *req.Name != "test-app-2" {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("application %s not found", *req.Name))
	}

	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *req.Name,
			Namespace: "argocd",
		},
		Status: v1alpha1.ApplicationStatus{
			OperationState: &v1alpha1.OperationState{
				Phase:     "Running",
				Message:   "Sync operation initiated",
				StartedAt: metav1.NewTime(time.Now()),
				Operation: v1alpha1.Operation{
					Sync: &v1alpha1.SyncOperation{
						Prune:  *req.Prune,
						DryRun: *req.DryRun,
					},
				},
			},
		},
	}

	if *req.DryRun {
		app.Status.OperationState.Phase = "Succeeded"
		app.Status.OperationState.Message = "Dry run completed successfully"
	}

	return app, nil
}

func (s *mockApplicationService) Create(ctx context.Context, req *application.ApplicationCreateRequest) (*v1alpha1.Application, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Application == nil {
		return nil, status.Error(codes.InvalidArgument, "application is required")
	}

	if req.Application.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "application name is required")
	}

	// Create a new application based on the request
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Application.Name,
			Namespace: "argocd",
		},
		Spec: req.Application.Spec,
		Status: v1alpha1.ApplicationStatus{
			Health: v1alpha1.HealthStatus{
				Status:  "Healthy",
				Message: "Application created successfully",
			},
			Sync: v1alpha1.SyncStatus{
				Status:   "Synced",
				Revision: "abc123",
			},
			OperationState: &v1alpha1.OperationState{
				Phase:      "Succeeded",
				Message:    "Application created successfully",
				FinishedAt: &metav1.Time{Time: time.Now()},
			},
		},
	}

	return app, nil
}

func (s *mockApplicationService) Delete(ctx context.Context, req *application.ApplicationDeleteRequest) (*application.ApplicationResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Name == nil || *req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "application name is required")
	}

	if *req.Name != "test-app-1" && *req.Name != "test-app-2" {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("application %s not found", *req.Name))
	}

	return &application.ApplicationResponse{}, nil
}

func (s *mockApplicationService) GetManifests(ctx context.Context, req *application.ApplicationManifestQuery) (*repository.ManifestResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Name == nil || *req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "application name is required")
	}

	// Return mock manifests based on application name
	switch *req.Name {
	case "test-app-1", "test-app-2", "test-app-new":
		return &repository.ManifestResponse{
			Manifests: []string{
				`apiVersion: v1
kind: Service
metadata:
  name: test-service
  namespace: default
spec:
  selector:
    app: test
  ports:
  - port: 80
    targetPort: 8080`,
				`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: nginx:latest
        ports:
        - containerPort: 8080`,
			},
			Namespace: "default",
			Server:    "https://kubernetes.default.svc",
			Revision:  "abc123",
		}, nil
	default:
		return nil, status.Error(codes.NotFound, fmt.Sprintf("application %s not found", *req.Name))
	}
}

func (s *mockApplicationService) ListResourceEvents(ctx context.Context, req *application.ApplicationResourceEventsQuery) (*v1.EventList, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	// Return mock events for any application
	events := &v1.EventList{
		Items: []v1.Event{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-event-1",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				},
				InvolvedObject: v1.ObjectReference{
					Kind:      "Pod",
					Name:      "test-pod-1",
					Namespace: "default",
				},
				Type:    "Normal",
				Reason:  "Scheduled",
				Message: "Successfully assigned default/test-pod-1 to node-1",
				Source: v1.EventSource{
					Component: "default-scheduler",
				},
				FirstTimestamp: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				LastTimestamp:  metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				Count:          1,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-event-2",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				},
				InvolvedObject: v1.ObjectReference{
					Kind:      "Pod",
					Name:      "test-pod-1",
					Namespace: "default",
				},
				Type:    "Normal",
				Reason:  "Pulled",
				Message: "Container image \"nginx:latest\" already present on machine",
				Source: v1.EventSource{
					Component: "kubelet",
					Host:      "node-1",
				},
				FirstTimestamp: metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				LastTimestamp:  metav1.Time{Time: time.Now().Add(-5 * time.Minute)},
				Count:          1,
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-event-3",
					Namespace:         "default",
					CreationTimestamp: metav1.Time{Time: time.Now().Add(-4 * time.Minute)},
				},
				InvolvedObject: v1.ObjectReference{
					Kind:      "Pod",
					Name:      "test-pod-1",
					Namespace: "default",
				},
				Type:    "Normal",
				Reason:  "Created",
				Message: "Created container test",
				Source: v1.EventSource{
					Component: "kubelet",
					Host:      "node-1",
				},
				FirstTimestamp: metav1.Time{Time: time.Now().Add(-4 * time.Minute)},
				LastTimestamp:  metav1.Time{Time: time.Now().Add(-4 * time.Minute)},
				Count:          1,
			},
		},
	}

	return events, nil
}

type mockProjectService struct {
	project.UnimplementedProjectServiceServer
}

func (s *mockProjectService) List(ctx context.Context, req *project.ProjectQuery) (*v1alpha1.AppProjectList, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	projects := &v1alpha1.AppProjectList{
		Items: []v1alpha1.AppProject{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: "argocd",
				},
				Spec: v1alpha1.AppProjectSpec{
					Description: "Default project",
					SourceRepos: []string{"*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "*",
							Namespace: "*",
						},
					},
					ClusterResourceWhitelist: []metav1.GroupKind{
						{
							Group: "*",
							Kind:  "*",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "production",
					Namespace: "argocd",
				},
				Spec: v1alpha1.AppProjectSpec{
					Description: "Production project",
					SourceRepos: []string{
						"https://github.com/production/*",
						"https://gitlab.com/production/*",
					},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "https://production.cluster.local",
							Namespace: "prod-*",
						},
					},
					ClusterResourceWhitelist: []metav1.GroupKind{
						{
							Group: "apps",
							Kind:  "Deployment",
						},
						{
							Group: "",
							Kind:  "Service",
						},
						{
							Group: "",
							Kind:  "ConfigMap",
						},
					},
					Roles: []v1alpha1.ProjectRole{
						{
							Name: "admin",
							Policies: []string{
								"p, proj:production:admin, applications, *, production/*, allow",
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "development",
					Namespace: "argocd",
				},
				Spec: v1alpha1.AppProjectSpec{
					Description: "Development project",
					SourceRepos: []string{"*"},
					Destinations: []v1alpha1.ApplicationDestination{
						{
							Server:    "https://kubernetes.default.svc",
							Namespace: "dev-*",
						},
					},
				},
			},
		},
	}

	return projects, nil
}

func (s *mockProjectService) Get(ctx context.Context, req *project.ProjectQuery) (*v1alpha1.AppProject, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "project name is required")
	}

	switch req.Name {
	case "default":
		return &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "default",
				Namespace: "argocd",
			},
			Spec: v1alpha1.AppProjectSpec{
				Description: "Default project",
				SourceRepos: []string{"*"},
				Destinations: []v1alpha1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "*",
					},
				},
				ClusterResourceWhitelist: []metav1.GroupKind{
					{
						Group: "*",
						Kind:  "*",
					},
				},
			},
		}, nil
	case "production":
		return &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "production",
				Namespace: "argocd",
			},
			Spec: v1alpha1.AppProjectSpec{
				Description: "Production project",
				SourceRepos: []string{
					"https://github.com/production/*",
					"https://gitlab.com/production/*",
				},
				Destinations: []v1alpha1.ApplicationDestination{
					{
						Server:    "https://production.cluster.local",
						Namespace: "prod-*",
					},
				},
				ClusterResourceWhitelist: []metav1.GroupKind{
					{
						Group: "apps",
						Kind:  "Deployment",
					},
					{
						Group: "",
						Kind:  "Service",
					},
					{
						Group: "",
						Kind:  "ConfigMap",
					},
				},
				Roles: []v1alpha1.ProjectRole{
					{
						Name: "admin",
						Policies: []string{
							"p, proj:production:admin, applications, *, production/*, allow",
						},
					},
				},
			},
		}, nil
	case "development":
		return &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "development",
				Namespace: "argocd",
			},
			Spec: v1alpha1.AppProjectSpec{
				Description: "Development project",
				SourceRepos: []string{"*"},
				Destinations: []v1alpha1.ApplicationDestination{
					{
						Server:    "https://kubernetes.default.svc",
						Namespace: "dev-*",
					},
				},
			},
		}, nil
	default:
		return nil, status.Error(codes.NotFound, fmt.Sprintf("project %s not found", req.Name))
	}
}

func (s *mockProjectService) Create(ctx context.Context, req *project.ProjectCreateRequest) (*v1alpha1.AppProject, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Project == nil {
		return nil, status.Error(codes.InvalidArgument, "project is required")
	}

	if req.Project.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "project name is required")
	}

	// Check if project already exists (for non-upsert)
	if !req.Upsert {
		existingProjects := []string{"default", "production", "development"}
		for _, existing := range existingProjects {
			if req.Project.Name == existing {
				return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("project %s already exists", req.Project.Name))
			}
		}
	}

	// Return the created/updated project
	createdProject := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Project.Name,
			Namespace: "argocd",
		},
		Spec: req.Project.Spec,
	}

	// Set defaults if not provided
	if len(createdProject.Spec.SourceRepos) == 0 {
		createdProject.Spec.SourceRepos = []string{"*"}
	}
	if len(createdProject.Spec.Destinations) == 0 {
		createdProject.Spec.Destinations = []v1alpha1.ApplicationDestination{
			{
				Server:    "https://kubernetes.default.svc",
				Namespace: "*",
			},
		}
	}

	return createdProject, nil
}

type mockApplicationSetService struct {
	applicationset.UnimplementedApplicationSetServiceServer
}

func (s *mockApplicationSetService) List(ctx context.Context, req *applicationset.ApplicationSetListQuery) (*v1alpha1.ApplicationSetList, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	appSets := &v1alpha1.ApplicationSetList{
		Items: []v1alpha1.ApplicationSet{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset-1",
					Namespace: "argocd",
					Labels: map[string]string{
						"env": "dev",
					},
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "default",
							Source: &v1alpha1.ApplicationSource{
								RepoURL:        "https://github.com/test/appset-repo",
								Path:           "manifests",
								TargetRevision: "main",
							},
							Destination: v1alpha1.ApplicationDestination{
								Server:    "https://kubernetes.default.svc",
								Namespace: "default",
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-appset-2",
					Namespace: "argocd",
					Labels: map[string]string{
						"env": "prod",
					},
				},
				Spec: v1alpha1.ApplicationSetSpec{
					Template: v1alpha1.ApplicationSetTemplate{
						Spec: v1alpha1.ApplicationSpec{
							Project: "production",
							Source: &v1alpha1.ApplicationSource{
								RepoURL:        "https://github.com/test/prod-appset",
								Path:           "charts",
								TargetRevision: "v1.0.0",
							},
							Destination: v1alpha1.ApplicationDestination{
								Server:    "https://production.cluster.local",
								Namespace: "prod",
							},
						},
					},
				},
			},
		},
	}

	// Filter by project if specified
	if len(req.Projects) > 0 {
		var filtered []v1alpha1.ApplicationSet
		for _, appSet := range appSets.Items {
			for _, project := range req.Projects {
				if appSet.Spec.Template.Spec.Project == project {
					filtered = append(filtered, appSet)
					break
				}
			}
		}
		appSets.Items = filtered
	}

	return appSets, nil
}

func (s *mockApplicationSetService) Get(ctx context.Context, req *applicationset.ApplicationSetGetQuery) (*v1alpha1.ApplicationSet, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "applicationset name is required")
	}

	// Mock data for specific ApplicationSets
	switch req.Name {
	case "test-appset-1":
		return &v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-appset-1",
				Namespace: "argocd",
				Labels: map[string]string{
					"env": "dev",
				},
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/test/appset-repo",
							Path:           "manifests",
							TargetRevision: "main",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					},
				},
			},
		}, nil
	case "test-appset-2":
		return &v1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-appset-2",
				Namespace: "argocd",
				Labels: map[string]string{
					"env": "prod",
				},
			},
			Spec: v1alpha1.ApplicationSetSpec{
				Template: v1alpha1.ApplicationSetTemplate{
					Spec: v1alpha1.ApplicationSpec{
						Project: "production",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/test/prod-appset",
							Path:           "charts",
							TargetRevision: "v1.0.0",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://production.cluster.local",
							Namespace: "prod",
						},
					},
				},
			},
		}, nil
	default:
		return nil, status.Error(codes.NotFound, fmt.Sprintf("applicationset %s not found", req.Name))
	}
}

func (s *mockApplicationSetService) Create(ctx context.Context, req *applicationset.ApplicationSetCreateRequest) (*v1alpha1.ApplicationSet, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Applicationset == nil {
		return nil, status.Error(codes.InvalidArgument, "applicationset is required")
	}

	if req.Applicationset.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "applicationset name is required")
	}

	// Check if ApplicationSet already exists (for non-upsert mode)
	if !req.Upsert {
		if req.Applicationset.Name == "test-appset-1" || req.Applicationset.Name == "test-appset-2" {
			return nil, status.Error(codes.AlreadyExists, fmt.Sprintf("applicationset %s already exists", req.Applicationset.Name))
		}
	}

	// For dry run, just return the input ApplicationSet
	if req.DryRun {
		return req.Applicationset, nil
	}

	// Return the created ApplicationSet
	return req.Applicationset, nil
}

type mockClusterService struct {
	cluster.UnimplementedClusterServiceServer
}

func (s *mockClusterService) List(ctx context.Context, req *cluster.ClusterQuery) (*v1alpha1.ClusterList, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	clusters := &v1alpha1.ClusterList{
		Items: []v1alpha1.Cluster{
			{
				Server: "https://kubernetes.default.svc",
				Name:   "in-cluster",
				Config: v1alpha1.ClusterConfig{
					TLSClientConfig: v1alpha1.TLSClientConfig{
						Insecure: false,
					},
				},
				ServerVersion: "1.28",
			},
			{
				Server: "https://external-cluster.example.com",
				Name:   "external-cluster",
				Config: v1alpha1.ClusterConfig{
					TLSClientConfig: v1alpha1.TLSClientConfig{
						Insecure: true,
					},
				},
				ServerVersion: "1.27",
			},
		},
	}

	return clusters, nil
}

func (s *mockClusterService) Get(ctx context.Context, req *cluster.ClusterQuery) (*v1alpha1.Cluster, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	auth := md.Get("authorization")
	if len(auth) == 0 || auth[0] != "Bearer test-token" {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization")
	}

	if req.Server == "" {
		return nil, status.Error(codes.InvalidArgument, "server is required")
	}

	// Mock clusters
	clusters := map[string]*v1alpha1.Cluster{
		"https://kubernetes.default.svc": {
			Server: "https://kubernetes.default.svc",
			Name:   "in-cluster",
			Config: v1alpha1.ClusterConfig{
				TLSClientConfig: v1alpha1.TLSClientConfig{
					Insecure: false,
				},
			},
			ServerVersion: "1.28",
		},
		"https://external-cluster.example.com": {
			Server: "https://external-cluster.example.com",
			Name:   "external-cluster",
			Config: v1alpha1.ClusterConfig{
				TLSClientConfig: v1alpha1.TLSClientConfig{
					Insecure: true,
				},
			},
			ServerVersion: "1.27",
		},
	}

	cluster, exists := clusters[req.Server]
	if !exists {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("cluster %s not found", req.Server))
	}

	return cluster, nil
}

func main() {
	port := flag.String("port", "50051", "gRPC server port")
	flag.Parse()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	application.RegisterApplicationServiceServer(s, &mockApplicationService{})
	applicationset.RegisterApplicationSetServiceServer(s, &mockApplicationSetService{})
	project.RegisterProjectServiceServer(s, &mockProjectService{})
	cluster.RegisterClusterServiceServer(s, &mockClusterService{})

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Start server in goroutine
	serverErrChan := make(chan error, 1)
	go func() {
		log.Printf("Mock ArgoCD gRPC server listening on port %s", *port)
		if err := s.Serve(lis); err != nil {
			serverErrChan <- err
		}
	}()

	// Wait for signal or server error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal %v, shutting down gracefully...", sig)
	case err := <-serverErrChan:
		log.Printf("Server error: %v", err)
	}

	// Graceful shutdown with timeout
	stopped := make(chan struct{})
	go func() {
		s.GracefulStop()
		close(stopped)
	}()

	// Wait for graceful stop or force stop after timeout
	select {
	case <-stopped:
		log.Println("Server gracefully stopped")
	case <-time.After(5 * time.Second):
		log.Println("Graceful shutdown timeout, forcing stop")
		s.Stop()
	}
}
