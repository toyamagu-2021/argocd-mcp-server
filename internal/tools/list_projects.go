package tools

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListProjectsTool defines the list_project tool schema.
var ListProjectsTool = mcp.NewTool("list_project",
	mcp.WithDescription("Lists ArgoCD projects. detailed=false (the default) returns a page of summaries with name, description, sourceRepoCount, destinationCount, and roleCount. name_only=true returns name strings and wins over detailed. detailed=true returns full projects without managedFields or the last-applied annotation. Roles, tokens, sync windows, and allow-lists are detailed-only. limit defaults to 50 and offset to 0."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithBoolean("name_only",
		mcp.Description("If true, returns only project names inside items. Takes precedence over detailed."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns full projects (can be very large). If false (default), returns summaries."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of projects to return (default: 50). Use with offset to page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of projects to skip (default: 0)."),
	),
)

type projectSummary struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	SourceRepoCount  int    `json:"sourceRepoCount"`
	DestinationCount int    `json:"destinationCount"`
	RoleCount        int    `json:"roleCount"`
}

// makeListProjects returns the handler for this tool bound to the shared client in d.
func makeListProjects(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit, offset, err := pageArgs(request, defaultPageLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return listProjectsHandler(ctx, d.Client, request.GetBool("name_only", false), request.GetBool("detailed", false), limit, offset)
	}
}

func listProjectsHandler(
	ctx context.Context,
	argoClient client.Interface,
	nameOnly bool,
	detailed bool,
	limit, offset int,
) (*mcp.CallToolResult, error) {
	projectList, err := argoClient.ListProjects(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list projects: %v", err)), nil
	}
	var items []v1alpha1.AppProject
	if projectList != nil {
		items = projectList.Items
	}
	if items == nil {
		items = []v1alpha1.AppProject{}
	}
	sliced, total, hasMore := pageSlice(items, limit, offset)
	notice := pageNotice(offset, len(sliced), total, hasMore, "")
	if nameOnly {
		names := make([]string, len(sliced))
		for i, project := range sliced {
			names[i] = project.Name
		}
		return writeCompact(page{Items: names, TotalItems: total, HasMore: hasMore, Notice: notice})
	}
	if detailed {
		full := make([]v1alpha1.AppProject, len(sliced))
		for i, project := range sliced {
			full[i] = stripProject(project)
		}
		return writeCompact(page{Items: full, TotalItems: total, HasMore: hasMore, Notice: notice})
	}
	summaries := make([]projectSummary, len(sliced))
	for i, project := range sliced {
		summaries[i] = projectSummary{
			Name:             project.Name,
			Description:      project.Spec.Description,
			SourceRepoCount:  len(project.Spec.SourceRepos),
			DestinationCount: len(project.Spec.Destinations),
			RoleCount:        len(project.Spec.Roles),
		}
	}
	return writeCompact(page{Items: summaries, TotalItems: total, HasMore: hasMore, Notice: notice})
}

func stripProject(project v1alpha1.AppProject) v1alpha1.AppProject {
	if project.Annotations != nil {
		ann := make(map[string]string, len(project.Annotations))
		for k, v := range project.Annotations {
			ann[k] = v
		}
		project.Annotations = ann
	}
	stripObjectMeta(&project.ObjectMeta)
	return project
}
