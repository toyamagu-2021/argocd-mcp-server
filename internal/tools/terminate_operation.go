package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// Define the tool schema
var TerminateOperationTool = mcp.NewTool("terminate_operation",
	mcp.WithDescription("Terminates the currently running operation (sync, refresh, etc.) on an ArgoCD application"),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application whose operation should be terminated"),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Optional. The namespace where the ArgoCD application resource is located (for multi-tenant setups)"),
	),
	mcp.WithString("project",
		mcp.Description("Optional. The ArgoCD project the application belongs to"),
	),
)

// HandleTerminateOperation processes terminate_operation tool requests
func HandleTerminateOperation(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	return terminateOperationHandler(ctx, argoClient, name, appNamespace, project)
}

// terminateOperationHandler handles the core logic for the tool.
// This is separated out to enable testing with mocked clients.
func terminateOperationHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
	appNamespace string,
	project string,
) (*mcp.CallToolResult, error) {
	// Terminate the operation
	err := argoClient.TerminateOperation(ctx, name, appNamespace, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to terminate operation: %v", err)), nil
	}

	// Return success message
	message := fmt.Sprintf("Successfully terminated operation for application '%s'", name)
	if appNamespace != "" {
		message += fmt.Sprintf(" in namespace '%s'", appNamespace)
	}
	if project != "" {
		message += fmt.Sprintf(" (project: %s)", project)
	}

	return mcp.NewToolResultText(message), nil
}
