package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetAppEventsTool defines the get_application_events tool schema
var GetAppEventsTool = mcp.NewTool("get_application_events",
	mcp.WithDescription("Gets Kubernetes events for resources belonging to an ArgoCD application. Returns events that help diagnose issues with application resources."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
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

// makeGetApplicationEvents returns the handler for this tool bound to the shared client in d.
func makeGetApplicationEvents(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters from mcp.CallToolRequest
		appName := request.GetString("name", "")
		resourceNamespace := request.GetString("resource_namespace", "")
		resourceName := request.GetString("resource_name", "")
		resourceUID := request.GetString("resource_uid", "")
		appNamespace := request.GetString("app_namespace", "")
		project := request.GetString("project", "")

		argoClient := d.Client

		// Use the handler function with the real client
		return getApplicationEventsHandler(ctx, argoClient, client.EventsOptions{
			Name:              appName,
			ResourceNamespace: resourceNamespace,
			ResourceName:      resourceName,
			ResourceUID:       resourceUID,
			AppNamespace:      appNamespace,
			Project:           project,
		})
	}
}

// getApplicationEventsHandler handles the core logic for getting application events.
// This is separated out to enable testing with mocked clients.
func getApplicationEventsHandler(
	ctx context.Context,
	argoClient client.Interface,
	opts client.EventsOptions,
) (*mcp.CallToolResult, error) {
	if opts.Name == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	events, err := argoClient.GetApplicationEvents(ctx, opts)
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
