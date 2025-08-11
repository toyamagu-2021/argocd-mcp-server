package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListClusterTool provides MCP tool for listing all ArgoCD clusters
var ListClusterTool = mcp.NewTool("list_cluster",
	mcp.WithDescription("Lists all ArgoCD clusters configured in the system"),
)

// HandleListCluster handles MCP tool requests for listing ArgoCD clusters
func HandleListCluster(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	return listClusterHandler(ctx, argoClient)
}

func listClusterHandler(
	ctx context.Context,
	argoClient client.Interface,
) (*mcp.CallToolResult, error) {
	clusters, err := argoClient.ListClusters(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list clusters: %v", err)), nil
	}

	jsonData, err := json.MarshalIndent(clusters, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
