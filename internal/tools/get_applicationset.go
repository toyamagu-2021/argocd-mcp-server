package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationSetTool defines the get_applicationset tool schema
var GetApplicationSetTool = mcp.NewTool("get_applicationset",
	mcp.WithDescription("Gets detailed information about a specific ArgoCD ApplicationSet"),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the ApplicationSet to retrieve."),
	),
)

// HandleGetApplicationSet processes get_applicationset tool requests
func HandleGetApplicationSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
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

	return getApplicationSetHandler(ctx, argoClient, name)
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
