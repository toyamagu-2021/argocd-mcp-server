package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListAppsTool defines the list_application tool schema
var ListAppsTool = mcp.NewTool("list_application",
	mcp.WithDescription("Lists all ArgoCD applications, with optional filters."),
	mcp.WithString("project",
		mcp.Description("Filter applications by project name."),
	),
	mcp.WithString("cluster",
		mcp.Description("Filter applications by destination cluster name or URL."),
	),
	mcp.WithString("namespace",
		mcp.Description("Filter applications by destination namespace."),
	),
	mcp.WithString("selector",
		mcp.Description("Filter applications by a label selector (e.g., 'key=value')."),
	),
)

// HandleListApplications processes list_application tool requests
func HandleListApplications(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Safely extract optional parameters from mcp.CallToolRequest
	project := request.GetString("project", "")
	cluster := request.GetString("cluster", "")
	namespace := request.GetString("namespace", "")
	selector := request.GetString("selector", "")

	// Create gRPC client and list applications
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

	// Use the handler function with the real client
	return listApplicationsHandler(ctx, argoClient, project, cluster, namespace, selector)
}

// listApplicationsHandler handles the core logic for listing applications.
// This is separated out to enable testing with mocked clients.
func listApplicationsHandler(
	ctx context.Context,
	argoClient client.Interface,
	project, cluster, namespace, selector string,
) (*mcp.CallToolResult, error) {
	appList, err := argoClient.ListApplications(ctx, selector)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list applications: %v", err)), nil
	}

	apps := appList.Items

	// Filter by project, cluster, namespace if specified
	var filteredApps []v1alpha1.Application
	for _, app := range apps {
		if project != "" && app.Spec.Project != project {
			continue
		}
		if cluster != "" && app.Spec.Destination.Server != cluster {
			continue
		}
		if namespace != "" && app.Spec.Destination.Namespace != namespace {
			continue
		}
		filteredApps = append(filteredApps, app)
	}

	if len(filteredApps) == 0 {
		return mcp.NewToolResultText("No applications found matching the criteria."), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(filteredApps, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
