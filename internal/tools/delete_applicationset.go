package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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
	mcp.WithString("app_namespace",
		mcp.Description("The namespace of the ApplicationSet resource (its metadata.namespace). If not specified, uses the Argo CD namespace."),
	),
	mcp.WithString("appsetNamespace",
		mcp.Description("Deprecated: use app_namespace. Still accepted during the 0.3 migration period."),
	),
)

// makeDeleteApplicationSet returns the handler for this tool bound to the shared client in d.
func makeDeleteApplicationSet(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract parameters from mcp.CallToolRequest
		appSetName := request.GetString("name", "")
		appSetNamespace, err := resolveAppSetNamespace(
			request.GetString("app_namespace", ""),
			request.GetString("appsetNamespace", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		argoClient := d.Client

		// Use the handler function with the real client
		return deleteApplicationSetHandler(ctx, argoClient, appSetName, appSetNamespace)
	}
}

// resolveAppSetNamespace merges app_namespace with its deprecated alias
// appsetNamespace (design D17). Conflicting values are rejected.
func resolveAppSetNamespace(appNamespace, legacy string) (string, error) {
	switch {
	case appNamespace == "":
		return legacy, nil
	case legacy == "" || legacy == appNamespace:
		return appNamespace, nil
	default:
		return "", fmt.Errorf("conflicting namespaces: app_namespace=%q, appsetNamespace=%q (appsetNamespace is deprecated; use app_namespace only)", appNamespace, legacy)
	}
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
