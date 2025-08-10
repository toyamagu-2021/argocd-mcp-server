package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// DeleteAppTool defines the delete_application tool schema
var DeleteAppTool = mcp.NewTool("delete_application",
	mcp.WithDescription("Deletes an ArgoCD application. Use with caution as this operation is destructive."),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to delete."),
	),
	mcp.WithBoolean("cascade",
		mcp.Description("Whether to perform a cascading delete to remove the application's resources from the cluster (default: true)."),
	),
)

// HandleDeleteApplication processes delete_application tool requests
func HandleDeleteApplication(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters from mcp.CallToolRequest
	appName := request.GetString("name", "")
	cascade := request.GetBool("cascade", true)

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
	return deleteApplicationHandler(ctx, argoClient, appName, cascade)
}

// deleteApplicationHandler handles the core logic for deleting an application.
// This is separated out to enable testing with mocked clients.
func deleteApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	cascade bool,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	err := argoClient.DeleteApplication(ctx, appName, cascade)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete application: %v", err)), nil
	}

	// Return success message
	message := fmt.Sprintf("Application '%s' deleted successfully", appName)
	if !cascade {
		message += " (non-cascading - resources preserved)"
	} else {
		message += " (cascading - resources removed)"
	}

	return mcp.NewToolResultText(message), nil
}
