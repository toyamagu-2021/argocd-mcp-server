package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationManagedResourcesTool lists resources Argo CD is managing for an application.
var GetApplicationManagedResourcesTool = mcp.NewTool("get_application_managed_resources",
	mcp.WithDescription("Lists resources managed by an ArgoCD application, including target and live state. This is a read of ApplicationService.ManagedResources. Secret data in those states is hidden by the Argo CD controller before it is cached. Optional filters restrict the result to one resource identity."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("Name of the Application."),
	),
	mcp.WithString("resource_namespace",
		mcp.Description("Only return resources in this namespace."),
	),
	mcp.WithString("resource_name",
		mcp.Description("Only return the resource with this name."),
	),
	mcp.WithString("version",
		mcp.Description("Only return resources with this Kubernetes API version, for example v1. This is not a git revision."),
	),
	mcp.WithString("group",
		mcp.Description("Only return resources in this API group. Empty means no group filter. Core resources such as ConfigMap have an empty group."),
	),
	mcp.WithString("kind",
		mcp.Description("Only return resources of this kind, for example Deployment."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
	mcp.WithString("project",
		mcp.Description("Project of the Application. Looked up from the Application when omitted."),
	),
)

// makeGetApplicationManagedResources returns the handler bound to the shared client in d.
func makeGetApplicationManagedResources(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return getApplicationManagedResourcesHandler(ctx, d.Client, client.ManagedResourcesOptions{
			Name:              request.GetString("name", ""),
			ResourceNamespace: request.GetString("resource_namespace", ""),
			ResourceName:      request.GetString("resource_name", ""),
			Version:           request.GetString("version", ""),
			Group:             request.GetString("group", ""),
			Kind:              request.GetString("kind", ""),
			AppNamespace:      request.GetString("app_namespace", ""),
			Project:           request.GetString("project", ""),
		})
	}
}

func getApplicationManagedResourcesHandler(
	ctx context.Context,
	argoClient client.Interface,
	opts client.ManagedResourcesOptions,
) (*mcp.CallToolResult, error) {
	if opts.Name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	resp, err := argoClient.GetApplicationManagedResources(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get managed resources: %v", err)), nil
	}
	jsonData, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}
