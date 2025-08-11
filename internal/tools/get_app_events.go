package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetAppEventsTool defines the get_application_events tool schema
var GetAppEventsTool = mcp.NewTool("get_application_events",
	mcp.WithDescription("Gets Kubernetes events for resources belonging to an ArgoCD application. Returns events that help diagnose issues with application resources."),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve events for."),
	),
	mcp.WithString("resource_namespace",
		mcp.Description("Optional. Filter events by resource namespace."),
	),
	mcp.WithString("resource_name",
		mcp.Description("Optional. Filter events by resource name."),
	),
	mcp.WithString("resource_uid",
		mcp.Description("Optional. Filter events by resource UID."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Optional. The namespace where the ArgoCD application resource is located (for multi-tenant setups)."),
	),
	mcp.WithString("project",
		mcp.Description("Optional. The ArgoCD project the application belongs to."),
	),
)

// HandleGetApplicationEvents processes get_application_events tool requests
func HandleGetApplicationEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract parameters from mcp.CallToolRequest
	appName := request.GetString("name", "")
	resourceNamespace := request.GetString("resource_namespace", "")
	resourceName := request.GetString("resource_name", "")
	resourceUID := request.GetString("resource_uid", "")
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
	return getApplicationEventsHandler(ctx, argoClient, appName, resourceNamespace, resourceName, resourceUID, appNamespace, project)
}

// getApplicationEventsHandler handles the core logic for getting application events.
// This is separated out to enable testing with mocked clients.
func getApplicationEventsHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	resourceNamespace string,
	resourceName string,
	resourceUID string,
	appNamespace string,
	project string,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	events, err := argoClient.GetApplicationEvents(ctx, appName, resourceNamespace, resourceName, resourceUID, appNamespace, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application events: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
