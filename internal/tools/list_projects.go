package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListProjectsTool defines the list_project tool schema
var ListProjectsTool = mcp.NewTool("list_project",
	mcp.WithDescription("Lists all ArgoCD projects. Use name_only=true to get just project names for a compact view."),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithBoolean("name_only",
		mcp.Description("If true, returns only project names. Useful for getting a quick list of project names."),
	),
)

// HandleListProjects processes list_project tool requests
func HandleListProjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Safely extract optional parameters from mcp.CallToolRequest
	nameOnly := request.GetBool("name_only", false)

	// Create gRPC client and list projects
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
	return listProjectsHandler(ctx, argoClient, nameOnly)
}

// ProjectNameList represents a list of project names
type ProjectNameList struct {
	Names []string `json:"names"`
	Count int      `json:"count"`
}

// listProjectsHandler handles the core logic for listing projects.
// This is separated out to enable testing with mocked clients.
func listProjectsHandler(
	ctx context.Context,
	argoClient client.Interface,
	nameOnly bool,
) (*mcp.CallToolResult, error) {
	projectList, err := argoClient.ListProjects(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list projects: %v", err)), nil
	}

	if len(projectList.Items) == 0 {
		return mcp.NewToolResultText("No projects found."), nil
	}

	var jsonData []byte

	if nameOnly {
		// Return only project names
		names := make([]string, 0, len(projectList.Items))
		for _, project := range projectList.Items {
			names = append(names, project.Name)
		}
		nameList := ProjectNameList{
			Names: names,
			Count: len(names),
		}
		jsonData, err = json.MarshalIndent(nameList, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
	} else {
		// Convert to JSON for better readability in MCP responses
		jsonData, err = json.MarshalIndent(projectList.Items, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
