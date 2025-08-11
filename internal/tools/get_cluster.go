package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetClusterTool provides MCP tool for retrieving cluster details from ArgoCD
var GetClusterTool = mcp.NewTool("get_cluster",
	mcp.WithDescription("Retrieves detailed information about a specific ArgoCD cluster"),
	mcp.WithString("server",
		mcp.Required(),
		mcp.Description("The server URL of the cluster to retrieve (e.g., https://kubernetes.default.svc)"),
	),
)

// HandleGetCluster handles MCP tool requests for retrieving cluster information
func HandleGetCluster(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	server := request.GetString("server", "")
	if server == "" {
		return mcp.NewToolResultError("server is required"), nil
	}

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

	return getClusterHandler(ctx, argoClient, server)
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

	jsonData, err := json.MarshalIndent(cluster, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
