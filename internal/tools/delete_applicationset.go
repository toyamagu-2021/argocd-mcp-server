package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// DeleteApplicationSetTool defines the delete_applicationset tool schema
var DeleteApplicationSetTool = mcp.NewTool("delete_applicationset",
	mcp.WithDescription("Deletes an ArgoCD ApplicationSet. Use with caution as this operation is destructive and will delete all applications managed by the ApplicationSet."),
	mcp.WithDestructiveHintAnnotation(true),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the ApplicationSet to delete."),
	),
	mcp.WithString("appsetNamespace",
		mcp.Description("The namespace of the ApplicationSet. If not specified, uses the ArgoCD control plane namespace."),
	),
)

// HandleDeleteApplicationSet processes delete_applicationset tool requests
func HandleDeleteApplicationSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters from mcp.CallToolRequest
	appSetName := request.GetString("name", "")
	appSetNamespace := request.GetString("appsetNamespace", "")

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
	return deleteApplicationSetHandler(ctx, argoClient, appSetName, appSetNamespace)
}

// deleteApplicationSetHandler handles the core logic for deleting an ApplicationSet.
// This is separated out to enable testing with mocked clients.
func deleteApplicationSetHandler(
	ctx context.Context,
	argoClient client.Interface,
	appSetName string,
	appSetNamespace string,
) (*mcp.CallToolResult, error) {
	if appSetName == "" {
		return mcp.NewToolResultError("ApplicationSet name is required"), nil
	}

	err := argoClient.DeleteApplicationSet(ctx, appSetName, appSetNamespace)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete ApplicationSet: %v", err)), nil
	}

	// Return success message
	message := fmt.Sprintf("ApplicationSet '%s' deleted successfully", appSetName)
	if appSetNamespace != "" {
		message = fmt.Sprintf("ApplicationSet '%s' in namespace '%s' deleted successfully", appSetName, appSetNamespace)
	}
	message += " (Note: All applications managed by this ApplicationSet will be deleted)"

	return mcp.NewToolResultText(message), nil
}
