package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListRepositoryTool defines the list_repository tool schema
var ListRepositoryTool = mcp.NewTool("list_repository",
	mcp.WithDescription("Lists all configured Git repositories in ArgoCD."),
	mcp.WithDestructiveHintAnnotation(false),
)

// makeListRepository returns the handler for this tool bound to the shared client in d.
func makeListRepository(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		argoClient := d.Client

		// Use the handler function with the real client
		return listRepositoryHandler(ctx, argoClient)
	}
}

// listRepositoryHandler handles the core logic for listing repositories.
// This is separated out to enable testing with mocked clients.
func listRepositoryHandler(
	ctx context.Context,
	argoClient client.Interface,
) (*mcp.CallToolResult, error) {
	repositoryList, err := argoClient.ListRepositories(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list repositories: %v", err)), nil
	}

	if len(repositoryList.Items) == 0 {
		return mcp.NewToolResultText("No repositories found."), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(repositoryList.Items, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
