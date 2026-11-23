package tools

import (
	"context"
	"encoding/json"
	"fmt"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationManagedResourcesTool lists resources Argo CD is managing for an application.
var GetApplicationManagedResourcesTool = mcp.NewTool("get_application_managed_resources",
	mcp.WithDescription("Lists resources managed by an ArgoCD application (ApplicationService.ManagedResources). Without a resource filter and with detailed=false (the default), returns compact JSON: group, kind, namespace, name, resourceVersion, hook, and modified. The four manifest strings (targetState, liveState, predictedLiveState, normalizedLiveState) and the diff patch are omitted. This API has no health field and no Kubernetes API version on each item. Pass detailed=true, or set any of resource_namespace, resource_name, version, group, or kind, to include those strings. Detailed strings have metadata.managedFields and kubectl.kubernetes.io/last-applied-configuration removed. limit defaults to 50 and offset to 0. The response includes totalItems and hasMore. Secret data in the states is hidden by the Argo CD controller before it is cached."),
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
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of resources to return (default: 50). Use with offset to page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of matching resources to skip (default: 0)."),
	),
)

// makeGetApplicationManagedResources returns the handler bound to the shared client in d.
func makeGetApplicationManagedResources(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit, offset, err := managedResourcesPageArgs(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return getApplicationManagedResourcesHandler(ctx, d.Client, client.ManagedResourcesOptions{
			Name:              request.GetString("name", ""),
			ResourceNamespace: request.GetString("resource_namespace", ""),
			ResourceName:      request.GetString("resource_name", ""),
			Version:           request.GetString("version", ""),
			Group:             request.GetString("group", ""),
			Kind:              request.GetString("kind", ""),
			AppNamespace:      request.GetString("app_namespace", ""),
			Project:           request.GetString("project", ""),
		}, request.GetBool("detailed", false), limit, offset)
	}
}

func managedResourcesPageArgs(request mcp.CallToolRequest) (int, int, error) {
	limit, err := nonNegativeIntArg(request, "limit", defaultManagedResourcesLimit)
	if err != nil {
		return 0, 0, err
	}
	if limit < 1 {
		return 0, 0, fmt.Errorf("limit must be at least 1")
	}
	offset, err := nonNegativeIntArg(request, "offset", 0)
	if err != nil {
		return 0, 0, err
	}
	return limit, offset, nil
}

func nonNegativeIntArg(request mcp.CallToolRequest, key string, def int) (int, error) {
	val, ok := request.GetArguments()[key]
	if !ok || val == nil {
		return def, nil
	}
	switch n := val.(type) {
	case int:
		if n < 0 {
			return 0, fmt.Errorf("%s must be zero or greater", key)
		}
		return n, nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("%s must be zero or greater", key)
		}
		return int(n), nil
	case float64:
		if n < 0 || n != float64(int64(n)) {
			return 0, fmt.Errorf("%s must be a non-negative integer", key)
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("%s must be a non-negative integer", key)
	}
}

const defaultManagedResourcesLimit = 50

const lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

func getApplicationManagedResourcesHandler(
	ctx context.Context,
	argoClient client.Interface,
	opts client.ManagedResourcesOptions,
	detailed bool,
	limit, offset int,
) (*mcp.CallToolResult, error) {
	if opts.Name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	if limit < 1 {
		return mcp.NewToolResultError("limit must be at least 1"), nil
	}
	if offset < 0 {
		return mcp.NewToolResultError("offset must be zero or greater"), nil
	}
	resp, err := argoClient.GetApplicationManagedResources(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get managed resources: %v", err)), nil
	}
	payload := formatManagedResources(resp, opts, detailed, limit, offset)
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

type managedResourcesPage struct {
	Items      any    `json:"items"`
	TotalItems int    `json:"totalItems"`
	HasMore    bool   `json:"hasMore"`
	Notice     string `json:"notice,omitempty"`
}

func formatManagedResources(resp *applicationpkg.ManagedResourcesResponse, opts client.ManagedResourcesOptions, detailed bool, limit, offset int) managedResourcesPage {
	var items []*v1alpha1.ResourceDiff
	if resp != nil {
		items = resp.Items
	}
	page, total, hasMore := pageResourceDiffs(items, limit, offset)
	notice := managedResourcesNotice(offset, len(page), total, hasMore)
	if includeManagedResourceState(opts, detailed) {
		detailedItems := make([]*v1alpha1.ResourceDiff, len(page))
		for i, item := range page {
			detailedItems[i] = stripResourceDiff(item)
		}
		return managedResourcesPage{Items: detailedItems, TotalItems: total, HasMore: hasMore, Notice: notice}
	}
	return managedResourcesPage{Items: summarizeResourceDiffs(page), TotalItems: total, HasMore: hasMore, Notice: notice}
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

func summarizeResourceDiffs(items []*v1alpha1.ResourceDiff) []managedResourceSummary {
	out := make([]managedResourceSummary, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, managedResourceSummary{
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

func pageResourceDiffs(items []*v1alpha1.ResourceDiff, limit, offset int) ([]*v1alpha1.ResourceDiff, int, bool) {
	cleaned := make([]*v1alpha1.ResourceDiff, 0, len(items))
	for _, item := range items {
		if item != nil {
			cleaned = append(cleaned, item)
		}
	}
	total := len(cleaned)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return cleaned[offset:end], total, end < total
}

func managedResourcesNotice(offset, returned, total int, hasMore bool) string {
	if offset == 0 && !hasMore {
		return ""
	}
	if returned == 0 {
		return fmt.Sprintf("offset %d is past %d items. Narrow with kind, resource_name, resource_namespace, group, or version.", offset, total)
	}
	end := offset + returned
	if hasMore {
		return fmt.Sprintf("Showing items %d-%d of %d. Pass offset=%d to fetch more, or narrow with kind, resource_name, resource_namespace, group, or version.", offset, end-1, total, end)
	}
	return fmt.Sprintf("Showing items %d-%d of %d.", offset, end-1, total)
}

func stripResourceDiff(in *v1alpha1.ResourceDiff) *v1alpha1.ResourceDiff {
	if in == nil {
		return nil
	}
	out := *in
	out.TargetState = stripManifestNoise(in.TargetState)
	out.LiveState = stripManifestNoise(in.LiveState)
	out.PredictedLiveState = stripManifestNoise(in.PredictedLiveState)
	out.NormalizedLiveState = stripManifestNoise(in.NormalizedLiveState)
	return &out
}

func stripManifestNoise(raw string) string {
	if raw == "" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return raw
	}
	if meta, ok := obj["metadata"].(map[string]any); ok {
		delete(meta, "managedFields")
		if ann, ok := meta["annotations"].(map[string]any); ok {
			delete(ann, lastAppliedAnnotation)
			if len(ann) == 0 {
				delete(meta, "annotations")
			}
		}
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return raw
	}
	return string(out)
}
