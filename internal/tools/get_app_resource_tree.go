package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationResourceTreeTool defines the tool for retrieving application resource tree
var GetApplicationResourceTreeTool = mcp.NewTool("get_application_resource_tree",
	mcp.WithDescription("Get the resource tree of an ArgoCD application, showing all resources and their relationships"),
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

// HandleGetApplicationResourceTree processes get_application_resource_tree tool requests
func HandleGetApplicationResourceTree(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters
	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	appNamespace := request.GetString("app_namespace", "")
	project := request.GetString("project", "")

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
	return getApplicationResourceTreeHandler(ctx, argoClient, name, appNamespace, project)
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
