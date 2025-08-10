package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetAppTool defines the get_application tool schema
var GetAppTool = mcp.NewTool("get_application",
	mcp.WithDescription("Retrieves detailed information about a specific ArgoCD application."),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve."),
	),
)

// HandleGetApplication processes get_application tool requests
func HandleGetApplication(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract required parameter from mcp.CallToolRequest
	appName := request.GetString("name", "")

	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	// Create gRPC client and get application
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
	defer argoClient.Close()

	app, err := argoClient.GetApplication(ctx, appName)
	if err != nil {
		// Return structured error information
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
