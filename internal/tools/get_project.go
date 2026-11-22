package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetProjectTool defines the get_project tool schema
var GetProjectTool = mcp.NewTool("get_project",
	mcp.WithDescription("Retrieves detailed information about a specific ArgoCD project."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the project to retrieve."),
	),
)

// makeGetProject returns the handler for this tool bound to the shared client in d.
func makeGetProject(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract required parameter
		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("Project name is required"), nil
		}

		argoClient := d.Client

		// Use the handler function with the real client
		return getProjectHandler(ctx, argoClient, name)
	}
}

// getProjectHandler handles the core logic for getting a project.
// This is separated out to enable testing with mocked clients.
func getProjectHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
) (*mcp.CallToolResult, error) {
	project, err := argoClient.GetProject(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get project: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
