package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListRepositoryTool defines the list_repository tool schema
var ListRepositoryTool = mcp.NewTool("list_repository",
	mcp.WithDescription("Lists all configured Git repositories in ArgoCD."),
	mcp.WithDestructiveHintAnnotation(false),
)

// HandleListRepository processes list_repository tool requests
func HandleListRepository(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Create gRPC client and list repositories
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
	defer func() { _ = argoClient.Close() }()

	// Use the handler function with the real client
	return listRepositoryHandler(ctx, argoClient)
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
