package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationResourceTool returns the live manifest of one resource in an application.
var GetApplicationResourceTool = mcp.NewTool("get_application_resource",
	mcp.WithDescription("Returns the live manifest of one Kubernetes resource managed by an ArgoCD application (ApplicationService.GetResource). The body is compact JSON. metadata.managedFields and kubectl.kubernetes.io/last-applied-configuration are removed. A nil manifest is omitted. Argo CD replaces Secret data before returning the manifest."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("Name of the Application."),
	),
	mcp.WithString("resource_name",
		mcp.Required(),
		mcp.Description("Name of the Kubernetes resource."),
	),
	mcp.WithString("kind",
		mcp.Required(),
		mcp.Description("Resource kind, for example Deployment or ConfigMap."),
	),
	mcp.WithString("version",
		mcp.Required(),
		mcp.Description("Kubernetes API version of the resource, for example v1. This is not a git revision."),
	),
	mcp.WithString("group",
		mcp.Description("API group of the resource. Leave empty for core resources such as ConfigMap or Pod."),
	),
	mcp.WithString("resource_namespace",
		mcp.Description("Namespace of the Kubernetes resource. Required when the application has resources of the same name in more than one namespace."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
	mcp.WithString("project",
		mcp.Description("Project of the Application. Looked up from the Application when omitted."),
	),
)

// makeGetApplicationResource returns the handler bound to the shared client in d.
func makeGetApplicationResource(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return getApplicationResourceHandler(ctx, d.Client, client.ResourceQuery{
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

func getApplicationResourceHandler(
	ctx context.Context,
	argoClient client.Interface,
	q client.ResourceQuery,
) (*mcp.CallToolResult, error) {
	switch {
	case q.Name == "":
		return mcp.NewToolResultError("name is required"), nil
	case q.ResourceName == "":
		return mcp.NewToolResultError("resource_name is required"), nil
	case q.Kind == "":
		return mcp.NewToolResultError("kind is required"), nil
	case q.Version == "":
		return mcp.NewToolResultError("version is required"), nil
	}
	resp, err := argoClient.GetApplicationResource(ctx, q)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application resource: %v", err)), nil
	}
	if resp != nil && resp.Manifest != nil {
		stripped := stripManifestNoise(*resp.Manifest)
		resp.Manifest = &stripped
	}
	return writeCompact(resp)
}
