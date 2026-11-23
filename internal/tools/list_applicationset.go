package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListApplicationSetTool defines the list_applicationset tool schema
var ListApplicationSetTool = mcp.NewTool("list_applicationset",
	mcp.WithDescription("Lists ArgoCD ApplicationSets. detailed=false (the default) returns a page of summaries with name, namespace, project, and generatorCount. Generators and the template, including Helm values, are omitted. detailed=true returns full ApplicationSets without managedFields or the last-applied annotation. limit defaults to 50 and offset to 0. Narrow with project or selector."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("project",
		mcp.Description("Filter ApplicationSets by project name."),
	),
	mcp.WithString("selector",
		mcp.Description("Filter ApplicationSets by a label selector (e.g., 'key=value')."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns full ApplicationSets, including generators and Helm values (can be very large). If false (default), returns summaries."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of ApplicationSets to return (default: 50). Use with offset to page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of matching ApplicationSets to skip (default: 0)."),
	),
)

// makeListApplicationSets returns the handler for this tool bound to the shared client in d.
func makeListApplicationSets(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit, offset, err := pageArgs(request, defaultPageLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return listApplicationSetsHandler(ctx, d.Client, request.GetString("project", ""), request.GetString("selector", ""), request.GetBool("detailed", false), limit, offset)
	}
}

// listApplicationSetsHandler handles the core logic for listing ApplicationSets.
// This is separated out to enable testing with mocked clients.
func listApplicationSetsHandler(
	ctx context.Context,
	argoClient client.Interface,
	project, selector string,
	detailed bool,
	limit, offset int,
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

	if filteredAppSets == nil {
		filteredAppSets = []v1alpha1.ApplicationSet{}
	}
	sliced, total, hasMore := pageSlice(filteredAppSets, limit, offset)
	notice := pageNotice(offset, len(sliced), total, hasMore, "project or selector")
	if detailed {
		full := make([]v1alpha1.ApplicationSet, len(sliced))
		for i, appSet := range sliced {
			full[i] = stripApplicationSet(appSet)
		}
		return writeCompact(page{Items: full, TotalItems: total, HasMore: hasMore, Notice: notice})
	}
	summaries := make([]applicationSetSummary, len(sliced))
	for i, appSet := range sliced {
		summaries[i] = applicationSetSummary{
			Name:           appSet.Name,
			Namespace:      appSet.Namespace,
			Project:        appSet.Spec.Template.Spec.Project,
			GeneratorCount: len(appSet.Spec.Generators),
		}
	}
	return writeCompact(page{Items: summaries, TotalItems: total, HasMore: hasMore, Notice: notice})
}

type applicationSetSummary struct {
	Name           string `json:"name"`
	Namespace      string `json:"namespace,omitempty"`
	Project        string `json:"project,omitempty"`
	GeneratorCount int    `json:"generatorCount"`
}

func stripApplicationSet(appSet v1alpha1.ApplicationSet) v1alpha1.ApplicationSet {
	if appSet.Annotations != nil {
		ann := make(map[string]string, len(appSet.Annotations))
		for k, v := range appSet.Annotations {
			ann[k] = v
		}
		appSet.Annotations = ann
	}
	stripObjectMeta(&appSet.ObjectMeta)
	return appSet
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
