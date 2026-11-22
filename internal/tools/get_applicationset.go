package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationSetTool defines the get_applicationset tool schema
var GetApplicationSetTool = mcp.NewTool("get_applicationset",
	mcp.WithDescription("Gets detailed information about a specific ArgoCD ApplicationSet"),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the ApplicationSet to retrieve."),
	),
)

// makeGetApplicationSet returns the handler for this tool bound to the shared client in d.
func makeGetApplicationSet(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("name is required"), nil
		}

		argoClient := d.Client

		return getApplicationSetHandler(ctx, argoClient, name)
	}
}

// getApplicationSetHandler handles the core logic for getting an ApplicationSet.
// This is separated out to enable testing with mocked clients.
func getApplicationSetHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
) (*mcp.CallToolResult, error) {
	appSet, err := argoClient.GetApplicationSet(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get ApplicationSet: %v", err)), nil
	}

	jsonData, err := json.MarshalIndent(appSet, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
