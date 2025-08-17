package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListApplicationSetTool defines the list_applicationset tool schema
var ListApplicationSetTool = mcp.NewTool("list_applicationset",
	mcp.WithDescription("Lists all ArgoCD ApplicationSets with optional filters"),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("project",
		mcp.Description("Filter ApplicationSets by project name."),
	),
	mcp.WithString("selector",
		mcp.Description("Filter ApplicationSets by a label selector (e.g., 'key=value')."),
	),
)

// HandleListApplicationSets processes list_applicationset tool requests
func HandleListApplicationSets(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	project := request.GetString("project", "")
	selector := request.GetString("selector", "")

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

	return listApplicationSetsHandler(ctx, argoClient, project, selector)
}

// listApplicationSetsHandler handles the core logic for listing ApplicationSets.
// This is separated out to enable testing with mocked clients.
func listApplicationSetsHandler(
	ctx context.Context,
	argoClient client.Interface,
	project, selector string,
) (*mcp.CallToolResult, error) {
	appSetList, err := argoClient.ListApplicationSets(ctx, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list ApplicationSets: %v", err)), nil
	}

	appSets := appSetList.Items

	// Filter by selector if specified
	var filteredAppSets []v1alpha1.ApplicationSet
	if selector != "" {
		parsedSelector, err := parseSelector(selector)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid selector: %v", err)), nil
		}
		for _, appSet := range appSets {
			if matchesSelector(appSet.Labels, parsedSelector) {
				filteredAppSets = append(filteredAppSets, appSet)
			}
		}
	} else {
		filteredAppSets = appSets
	}

	if len(filteredAppSets) == 0 {
		return mcp.NewToolResultText("No ApplicationSets found matching the criteria."), nil
	}

	jsonData, err := json.MarshalIndent(filteredAppSets, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

// parseSelector parses a simple selector string (e.g., "key=value")
func parseSelector(selector string) (map[string]string, error) {
	result := make(map[string]string)
	if selector == "" {
		return result, nil
	}

	parts := strings.Split(selector, "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid selector format, expected 'key=value'")
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return nil, fmt.Errorf("selector key cannot be empty")
	}

	result[key] = value
	return result, nil
}

// matchesSelector checks if labels match the selector
func matchesSelector(labels map[string]string, selector map[string]string) bool {
	for key, value := range selector {
		if labelValue, ok := labels[key]; !ok || labelValue != value {
			return false
		}
	}
	return true
}
