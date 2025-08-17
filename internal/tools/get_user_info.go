package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetUserInfoTool defines the get_user_info tool schema
var GetUserInfoTool = mcp.NewTool("get_user_info",
	mcp.WithDescription("Get current user information from ArgoCD. Returns details about the currently authenticated user including username, groups, and authentication status."),
	mcp.WithDestructiveHintAnnotation(false),
)

// HandleGetUserInfo processes get_user_info tool requests
func HandleGetUserInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
	return getUserInfoHandler(ctx, argoClient)
}

// UserInfo represents the user information response
type UserInfo struct {
	LoggedIn bool     `json:"loggedIn"`
	Username string   `json:"username"`
	Issuer   string   `json:"issuer,omitempty"`
	Groups   []string `json:"groups,omitempty"`
}

// getUserInfoHandler handles the core logic for getting user info.
// This is separated out to enable testing with mocked clients.
func getUserInfoHandler(
	ctx context.Context,
	argoClient client.Interface,
) (*mcp.CallToolResult, error) {
	// Get user info
	userInfo, err := argoClient.GetUserInfo(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get user info: %v", err)), nil
	}

	// Convert to our UserInfo type for cleaner JSON output
	info := UserInfo{
		LoggedIn: userInfo.LoggedIn,
		Username: userInfo.Username,
		Issuer:   userInfo.Iss,
		Groups:   userInfo.Groups,
	}

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
