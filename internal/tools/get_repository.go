package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetRepositoryTool defines the get_repository tool schema
var GetRepositoryTool = mcp.NewTool("get_repository",
	mcp.WithDescription("Retrieves detailed information about a specific Git repository configured in ArgoCD."),
	mcp.WithString("repo",
		mcp.Required(),
		mcp.Description("The URL of the repository to retrieve."),
	),
)

// HandleGetRepository processes get_repository tool requests
func HandleGetRepository(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract required parameter
	repo := request.GetString("repo", "")
	if repo == "" {
		return mcp.NewToolResultError("Repository URL is required"), nil
	}

	// Create gRPC client
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
	return getRepositoryHandler(ctx, argoClient, repo)
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

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(repository, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
