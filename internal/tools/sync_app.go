package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// SyncAppTool defines the sync_application tool schema
var SyncAppTool = mcp.NewTool("sync_application",
	mcp.WithDescription("Triggers a sync operation for a specific ArgoCD application."),
	mcp.WithDestructiveHintAnnotation(true),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to sync."),
	),
	mcp.WithBoolean("prune",
		mcp.Description("Whether to delete resources that are no longer defined in the source (default: false)."),
	),
	mcp.WithBoolean("dry_run",
		mcp.Description("Preview the sync operation without making actual changes (default: false)."),
	),
)

// makeSyncApplication returns the handler for this tool bound to the shared client in d.
func makeSyncApplication(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters from mcp.CallToolRequest
		appName := request.GetString("name", "")
		prune := request.GetBool("prune", false)
		dryRun := request.GetBool("dry_run", false)

		argoClient := d.Client

		// Use the handler function with the real client
		return syncApplicationHandler(ctx, argoClient, appName, prune, dryRun)
	}
}

// syncApplicationHandler handles the core logic for syncing an application.
// This is separated out to enable testing with mocked clients.
func syncApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	prune bool,
	dryRun bool,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	// Sync with empty revision to use latest
	app, err := argoClient.SyncApplication(ctx, appName, "", prune, dryRun)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to sync application: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
