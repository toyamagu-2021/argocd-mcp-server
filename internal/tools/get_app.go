package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetAppTool defines the get_application tool schema
var GetAppTool = mcp.NewTool("get_application",
	mcp.WithDescription("Retrieves detailed information about a specific ArgoCD application."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
)

// makeGetApplication returns the handler for this tool bound to the shared client in d.
func makeGetApplication(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract required parameter from mcp.CallToolRequest
		appName := request.GetString("name", "")
		appNamespace := request.GetString("app_namespace", "")

		argoClient := d.Client

		// Use the handler function with the real client
		return getApplicationHandler(ctx, argoClient, appName, appNamespace)
	}
}

// getApplicationHandler handles the core logic for getting an application.
// This is separated out to enable testing with mocked clients.
func getApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	appNamespace string,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	app, err := argoClient.GetApplication(ctx, appName, appNamespace)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
