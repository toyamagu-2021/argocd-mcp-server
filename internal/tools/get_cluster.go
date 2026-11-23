package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetClusterTool provides MCP tool for retrieving cluster details from ArgoCD
var GetClusterTool = mcp.NewTool("get_cluster",
	mcp.WithDescription("Retrieves one ArgoCD cluster as compact JSON. Credentials are removed. The last-applied annotation is removed."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("server",
		mcp.Required(),
		mcp.Description("The server URL of the cluster to retrieve (e.g., https://kubernetes.default.svc)"),
	),
)

// HandleGetCluster handles MCP tool requests for retrieving cluster information
// makeGetCluster returns the handler for this tool bound to the shared client in d.
func makeGetCluster(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		server := request.GetString("server", "")
		if server == "" {
			return mcp.NewToolResultError("server is required"), nil
		}

		argoClient := d.Client

		return getClusterHandler(ctx, argoClient, server)
	}
}

func getClusterHandler(
	ctx context.Context,
	argoClient client.Interface,
	server string,
) (*mcp.CallToolResult, error) {
	cluster, err := argoClient.GetCluster(ctx, server)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get cluster: %v", err)), nil
	}

	if cluster == nil {
		return writeCompact(nil)
	}
	stripped := strippedCluster(*cluster)
	return writeCompact(&stripped)
}
