package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// RefreshAppTool defines the refresh_application tool schema
var RefreshAppTool = mcp.NewTool("refresh_application",
	mcp.WithDescription("Refreshes the status of a specific ArgoCD application by fetching the latest state from Git and the cluster."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to refresh."),
	),
	mcp.WithBoolean("hard",
		mcp.Description("Forces a hard refresh, which triggers a full reconciliation (default: false)."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
)

// makeRefreshApplication returns the handler for this tool bound to the shared client in d.
func makeRefreshApplication(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters from mcp.CallToolRequest
		appName := request.GetString("name", "")
		hardRefresh := request.GetBool("hard", false)
		appNamespace := request.GetString("app_namespace", "")

		argoClient := d.Client

		// Use the handler function with the real client
		return refreshApplicationHandler(ctx, argoClient, appName, hardRefresh, appNamespace)
	}
}

// refreshApplicationHandler handles the core logic for refreshing an application.
// This is separated out to enable testing with mocked clients.
func refreshApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	hardRefresh bool,
	appNamespace string,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	// Call RefreshApplication to trigger a refresh
	refreshType := "normal"
	if hardRefresh {
		refreshType = "hard"
	}

	app, err := argoClient.RefreshApplication(ctx, appName, refreshType, appNamespace)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to refresh application: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
