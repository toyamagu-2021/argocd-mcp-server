package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// RefreshAppTool defines the refresh_application tool schema
var RefreshAppTool = mcp.NewTool("refresh_application",
	mcp.WithDescription("Refreshes the status of a specific ArgoCD application by fetching the latest state from Git and the cluster."),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to refresh."),
	),
	mcp.WithBoolean("hard",
		mcp.Description("Forces a hard refresh, which triggers a full reconciliation (default: false)."),
	),
)

// HandleRefreshApplication processes refresh_application tool requests
func HandleRefreshApplication(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters from mcp.CallToolRequest
	appName := request.GetString("name", "")
	hardRefresh := request.GetBool("hard", false)

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
	return refreshApplicationHandler(ctx, argoClient, appName, hardRefresh)
}

// refreshApplicationHandler handles the core logic for refreshing an application.
// This is separated out to enable testing with mocked clients.
func refreshApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	hardRefresh bool,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	// Call RefreshApplication to trigger a refresh
	refreshType := "normal"
	if hardRefresh {
		refreshType = "hard"
	}

	app, err := argoClient.RefreshApplication(ctx, appName, refreshType)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to refresh application: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
