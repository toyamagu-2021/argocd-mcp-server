package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetRepositoryTool defines the get_repository tool schema
var GetRepositoryTool = mcp.NewTool("get_repository",
	mcp.WithDescription("Retrieves one sanitized Git repository from ArgoCD as compact JSON. The password is already omitted."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("repo",
		mcp.Required(),
		mcp.Description("The URL of the repository to retrieve."),
	),
)

// makeGetRepository returns the handler for this tool bound to the shared client in d.
func makeGetRepository(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract required parameter
		repo := request.GetString("repo", "")
		if repo == "" {
			return mcp.NewToolResultError("Repository URL is required"), nil
		}

		argoClient := d.Client

		// Use the handler function with the real client
		return getRepositoryHandler(ctx, argoClient, repo)
	}
}

// getRepositoryHandler handles the core logic for getting a repository.
// This is separated out to enable testing with mocked clients.
func getRepositoryHandler(
	ctx context.Context,
	argoClient client.Interface,
	repo string,
) (*mcp.CallToolResult, error) {
	repository, err := argoClient.GetRepository(ctx, repo)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get repository: %v", err)), nil
	}

	return writeCompact(repository)
}
