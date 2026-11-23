package tools

import (
	"context"
	"encoding/json"
	"fmt"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationManagedResourcesTool lists resources Argo CD is managing for an application.
var GetApplicationManagedResourcesTool = mcp.NewTool("get_application_managed_resources",
	mcp.WithDescription("Lists resources managed by an ArgoCD application (ApplicationService.ManagedResources). Without a resource filter and with detailed=false (the default), returns a compact summary per resource: group, kind, namespace, name, resourceVersion, hook, and modified. The four manifest strings (targetState, liveState, predictedLiveState, normalizedLiveState) and the diff patch are omitted. This API has no health field and no Kubernetes API version on each item. Pass detailed=true, or set any of resource_namespace, resource_name, version, group, or kind, to include those strings. Secret data in the states is hidden by the Argo CD controller before it is cached."),
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
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns each resource's targetState, liveState, predictedLiveState, normalizedLiveState, and diff (can be very large). If false (default), those strings are omitted unless a resource filter is set. Same meaning as list_application's detailed flag."),
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
		}, request.GetBool("detailed", false))
	}
}

func getApplicationManagedResourcesHandler(
	ctx context.Context,
	argoClient client.Interface,
	opts client.ManagedResourcesOptions,
	detailed bool,
) (*mcp.CallToolResult, error) {
	if opts.Name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	resp, err := argoClient.GetApplicationManagedResources(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get managed resources: %v", err)), nil
	}
	var payload any = resp
	if !includeManagedResourceState(opts, detailed) {
		payload = summarizeManagedResources(resp)
	}
	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

// includeManagedResourceState reports whether the four manifest strings should be returned.
// A resource filter already narrows the payload. detailed matches list_application.
func includeManagedResourceState(opts client.ManagedResourcesOptions, detailed bool) bool {
	if detailed {
		return true
	}
	return opts.ResourceNamespace != "" || opts.ResourceName != "" || opts.Version != "" || opts.Group != "" || opts.Kind != ""
}

type managedResourceSummary struct {
	Group           string `json:"group,omitempty"`
	Kind            string `json:"kind,omitempty"`
	Namespace       string `json:"namespace,omitempty"`
	Name            string `json:"name,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	Hook            bool   `json:"hook,omitempty"`
	Modified        bool   `json:"modified"`
}

type managedResourceSummaryList struct {
	Items []managedResourceSummary `json:"items"`
}

func summarizeManagedResources(resp *applicationpkg.ManagedResourcesResponse) managedResourceSummaryList {
	out := managedResourceSummaryList{Items: []managedResourceSummary{}}
	if resp == nil {
		return out
	}
	out.Items = make([]managedResourceSummary, 0, len(resp.Items))
	for _, item := range resp.Items {
		if item == nil {
			continue
		}
		out.Items = append(out.Items, managedResourceSummary{
			Group:           item.Group,
			Kind:            item.Kind,
			Namespace:       item.Namespace,
			Name:            item.Name,
			ResourceVersion: item.ResourceVersion,
			Hook:            item.Hook,
			Modified:        item.Modified,
		})
	}
	return out
}
