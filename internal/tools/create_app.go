package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateAppTool defines the create_application tool schema
var CreateAppTool = mcp.NewTool("create_application",
	mcp.WithDescription("Creates a new ArgoCD application with specified source and destination configuration."),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to create."),
	),
	mcp.WithString("namespace",
		mcp.Description("The namespace where the application resource will be created (default: argocd)."),
	),
	mcp.WithString("project",
		mcp.Description("The ArgoCD project the application belongs to (default: default)."),
	),
	mcp.WithString("repo_url",
		mcp.Required(),
		mcp.Description("The Git repository URL containing the application manifests."),
	),
	mcp.WithString("path",
		mcp.Description("The path within the repository to the application manifests (default: .)."),
	),
	mcp.WithString("target_revision",
		mcp.Description("The target revision (branch, tag, or commit) to deploy (default: HEAD)."),
	),
	mcp.WithString("dest_server",
		mcp.Description("The destination cluster server URL (default: https://kubernetes.default.svc)."),
	),
	mcp.WithString("dest_namespace",
		mcp.Required(),
		mcp.Description("The destination namespace where the application will be deployed."),
	),
	mcp.WithBoolean("upsert",
		mcp.Description("Whether to update the application if it already exists (default: false)."),
	),
	mcp.WithBoolean("auto_sync",
		mcp.Description("Whether to enable automatic sync for the application (default: false)."),
	),
	mcp.WithBoolean("self_heal",
		mcp.Description("Whether to enable self-healing for the application (default: false)."),
	),
)

// HandleCreateApplication processes create_application tool requests
func HandleCreateApplication(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	params := CreateAppParams{
		Name:           request.GetString("name", ""),
		Namespace:      request.GetString("namespace", "argocd"),
		Project:        request.GetString("project", "default"),
		RepoURL:        request.GetString("repo_url", ""),
		Path:           request.GetString("path", "."),
		TargetRevision: request.GetString("target_revision", "HEAD"),
		DestServer:     request.GetString("dest_server", "https://kubernetes.default.svc"),
		DestNamespace:  request.GetString("dest_namespace", ""),
		Upsert:         request.GetBool("upsert", false),
		AutoSync:       request.GetBool("auto_sync", false),
		SelfHeal:       request.GetBool("self_heal", false),
	}

	// Create gRPC client
	config := &client.Config{
		ServerAddr:      os.Getenv("ARGOCD_SERVER"),
		AuthToken:       os.Getenv("ARGOCD_AUTH_TOKEN"),
		Insecure:        os.Getenv("ARGOCD_INSECURE") == "true",
		PlainText:       os.Getenv("ARGOCD_PLAINTEXT") == "true",
		GRPCWeb:         os.Getenv("ARGOCD_GRPC_WEB") == "true",
		GRPCWebRootPath: os.Getenv("ARGOCD_GRPC_WEB_ROOT_PATH"),
	}

	argoClient, err := client.New(config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create gRPC client: %v", err)), nil
	}
	defer func() { _ = argoClient.Close() }()

	// Use the handler function with the real client
	return createApplicationHandler(ctx, argoClient, params)
}

// CreateAppParams contains parameters for creating an application
type CreateAppParams struct {
	Name           string
	Namespace      string
	Project        string
	RepoURL        string
	Path           string
	TargetRevision string
	DestServer     string
	DestNamespace  string
	Upsert         bool
	AutoSync       bool
	SelfHeal       bool
}

// createApplicationHandler handles the core logic for creating an application.
// This is separated out to enable testing with mocked clients.
func createApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	params CreateAppParams,
) (*mcp.CallToolResult, error) {
	// Validate required parameters
	if params.Name == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}
	if params.RepoURL == "" {
		return mcp.NewToolResultError("Repository URL is required"), nil
	}
	if params.DestNamespace == "" {
		return mcp.NewToolResultError("Destination namespace is required"), nil
	}

	// Build the application spec
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      params.Name,
			Namespace: params.Namespace,
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: params.Project,
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        params.RepoURL,
				Path:           params.Path,
				TargetRevision: params.TargetRevision,
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    params.DestServer,
				Namespace: params.DestNamespace,
			},
		},
	}

	// Configure sync policy if auto_sync or self_heal is enabled
	if params.AutoSync || params.SelfHeal {
		app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{}

		if params.AutoSync {
			app.Spec.SyncPolicy.Automated = &v1alpha1.SyncPolicyAutomated{}

			if params.SelfHeal {
				app.Spec.SyncPolicy.Automated.SelfHeal = true
			}
		}
	}

	// Create the application
	createdApp, err := argoClient.CreateApplication(ctx, app, params.Upsert)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create application: %v", err)), nil
	}

	// Convert to JSON for response
	jsonData, err := json.MarshalIndent(createdApp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
