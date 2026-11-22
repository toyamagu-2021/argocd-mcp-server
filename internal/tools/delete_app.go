package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// DeleteAppTool defines the delete_application tool schema
var DeleteAppTool = mcp.NewTool("delete_application",
	mcp.WithDescription("Deletes an ArgoCD application. By default only the Application is deleted and the Kubernetes resources it manages are left in the cluster; pass cascade: true to delete those resources too. Note: this default differs from the Argo CD CLI, where cascade defaults to true."),
	mcp.WithDestructiveHintAnnotation(true),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to delete."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
	mcp.WithBoolean("cascade",
		mcp.Description("If true, also delete the Kubernetes resources managed by the application. Default: false (only the Application is deleted; its resources are left in the cluster). Unlike the Argo CD CLI, whose default is true."),
	),
)

// makeDeleteApplication returns the handler for this tool bound to the shared client in d.
func makeDeleteApplication(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters from mcp.CallToolRequest
		appName := request.GetString("name", "")
		cascade := request.GetBool("cascade", false)
		appNamespace := request.GetString("app_namespace", "")

		argoClient := d.Client

		// Use the handler function with the real client
		return deleteApplicationHandler(ctx, argoClient, appName, cascade, appNamespace)
	}
}

// deleteApplicationHandler handles the core logic for deleting an application.
// This is separated out to enable testing with mocked clients.
func deleteApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	cascade bool,
	appNamespace string,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	err := argoClient.DeleteApplication(ctx, appName, cascade, appNamespace)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete application: %v", err)), nil
	}

	// Return success message
	message := fmt.Sprintf("Application '%s' deleted successfully", appName)
	if !cascade {
		message += " (non-cascading - resources preserved)"
	} else {
		message += " (cascading - resources removed)"
	}

	return mcp.NewToolResultText(message), nil
}
