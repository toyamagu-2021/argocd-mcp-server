package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
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
	// Extract required parameters
	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	repoURL := request.GetString("repo_url", "")
	if repoURL == "" {
		return mcp.NewToolResultError("Repository URL is required"), nil
	}

	destNamespace := request.GetString("dest_namespace", "")
	if destNamespace == "" {
		return mcp.NewToolResultError("Destination namespace is required"), nil
	}

	// Extract optional parameters with defaults
	namespace := request.GetString("namespace", "argocd")
	project := request.GetString("project", "default")
	path := request.GetString("path", ".")
	targetRevision := request.GetString("target_revision", "HEAD")
	destServer := request.GetString("dest_server", "https://kubernetes.default.svc")
	upsert := request.GetBool("upsert", false)
	autoSync := request.GetBool("auto_sync", false)
	selfHeal := request.GetBool("self_heal", false)

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
	defer argoClient.Close()

	// Build the application spec
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: project,
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        repoURL,
				Path:           path,
				TargetRevision: targetRevision,
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    destServer,
				Namespace: destNamespace,
			},
		},
	}

	// Configure sync policy if auto_sync or self_heal is enabled
	if autoSync || selfHeal {
		app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{}

		if autoSync {
			app.Spec.SyncPolicy.Automated = &v1alpha1.SyncPolicyAutomated{}

			if selfHeal {
				app.Spec.SyncPolicy.Automated.SelfHeal = true
			}
		}
	}

	// Create the application
	createdApp, err := argoClient.CreateApplication(ctx, app, upsert)
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
