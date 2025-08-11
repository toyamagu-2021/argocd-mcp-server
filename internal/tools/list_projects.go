package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListProjectsTool defines the list_project tool schema
var ListProjectsTool = mcp.NewTool("list_project",
	mcp.WithDescription("Lists all ArgoCD projects."),
)

// HandleListProjects processes list_project tool requests
func HandleListProjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Create gRPC client and list projects
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
	return listProjectsHandler(ctx, argoClient)
}

// listProjectsHandler handles the core logic for listing projects.
// This is separated out to enable testing with mocked clients.
func listProjectsHandler(
	ctx context.Context,
	argoClient client.Interface,
) (*mcp.CallToolResult, error) {
	projectList, err := argoClient.ListProjects(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list projects: %v", err)), nil
	}

	if len(projectList.Items) == 0 {
		return mcp.NewToolResultText("No projects found."), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(projectList.Items, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
