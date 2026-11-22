package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationResourceTreeTool defines the tool for retrieving application resource tree
var GetApplicationResourceTreeTool = mcp.NewTool("get_application_resource_tree",
	mcp.WithDescription("Get the resource tree of an ArgoCD application, showing all resources and their relationships"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("Name of the application"),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the application (optional, will be auto-detected if not provided)"),
	),
	mcp.WithString("project",
		mcp.Description("Project of the application (optional, will be auto-detected if not provided)"),
	),
)

// makeGetApplicationResourceTree returns the handler for this tool bound to the shared client in d.
func makeGetApplicationResourceTree(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters
		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("name is required"), nil
		}

		appNamespace := request.GetString("app_namespace", "")
		project := request.GetString("project", "")

		argoClient := d.Client

		// Use the handler function with the real client
		return getApplicationResourceTreeHandler(ctx, argoClient, name, appNamespace, project)
	}
}

// getApplicationResourceTreeHandler handles the core logic for the tool.
// This is separated out to enable testing with mocked clients.
func getApplicationResourceTreeHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
	appNamespace string,
	project string,
) (*mcp.CallToolResult, error) {
	// Get the application resource tree
	tree, err := argoClient.GetApplicationResourceTree(ctx, name, appNamespace, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application resource tree: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
