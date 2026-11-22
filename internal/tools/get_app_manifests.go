package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetAppManifestsTool defines the get_application_manifests tool schema
var GetAppManifestsTool = mcp.NewTool("get_application_manifests",
	mcp.WithDescription("Retrieves the rendered Kubernetes manifests for an ArgoCD application. This shows what resources will be applied to the cluster."),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve manifests for."),
	),
	mcp.WithString("revision",
		mcp.Description("The git revision to retrieve manifests for. If not specified, uses the currently deployed revision."),
	),
)

// makeGetApplicationManifests returns the handler for this tool bound to the shared client in d.
func makeGetApplicationManifests(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters from mcp.CallToolRequest
		appName := request.GetString("name", "")
		revision := request.GetString("revision", "")

		argoClient := d.Client

		// Use the handler function with the real client
		return getApplicationManifestsHandler(ctx, argoClient, appName, revision)
	}
}

// getApplicationManifestsHandler handles the core logic for getting application manifests.
// This is separated out to enable testing with mocked clients.
func getApplicationManifestsHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	revision string,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	manifests, err := argoClient.GetApplicationManifests(ctx, appName, revision)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application manifests: %v", err)), nil
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(manifests, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
