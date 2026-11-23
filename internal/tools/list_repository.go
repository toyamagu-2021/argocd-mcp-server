package tools

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListRepositoryTool defines the list_repository tool schema.
var ListRepositoryTool = mcp.NewTool("list_repository",
	mcp.WithDescription("Lists configured repositories in ArgoCD. detailed=false (the default) returns a page of summaries with repo, type, name, project, and connectionStatus. detailed=true returns a page of sanitized repositories. limit defaults to 50 and offset to 0."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns sanitized repository objects (can include non-secret config). If false (default), returns summaries without credentials."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of repositories to return (default: 50). Use with offset to page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of repositories to skip (default: 0)."),
	),
)

type repositorySummary struct {
	Repo             string `json:"repo"`
	Type             string `json:"type,omitempty"`
	Name             string `json:"name,omitempty"`
	Project          string `json:"project,omitempty"`
	ConnectionStatus string `json:"connectionStatus,omitempty"`
}

// makeListRepository returns the handler for this tool bound to the shared client in d.
func makeListRepository(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit, offset, err := pageArgs(request, defaultPageLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return listRepositoryHandler(ctx, d.Client, request.GetBool("detailed", false), limit, offset)
	}
}

// listRepositoryHandler handles the core logic for listing repositories.
func listRepositoryHandler(
	ctx context.Context,
	argoClient client.Interface,
	detailed bool,
	limit, offset int,
) (*mcp.CallToolResult, error) {
	repositoryList, err := argoClient.ListRepositories(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list repositories: %v", err)), nil
	}
	var items v1alpha1.Repositories
	if repositoryList != nil {
		items = repositoryList.Items
	}
	if items == nil {
		items = v1alpha1.Repositories{}
	}
	sliced, total, hasMore := pageSlice(items, limit, offset)
	notice := pageNotice(offset, len(sliced), total, hasMore, "")
	if detailed {
		full := make([]*v1alpha1.Repository, len(sliced))
		for i, repo := range sliced {
			full[i] = client.SanitizeRepository(repo)
		}
		return writeCompact(page{Items: full, TotalItems: total, HasMore: hasMore, Notice: notice})
	}
	summaries := make([]repositorySummary, len(sliced))
	for i, repo := range sliced {
		summaries[i] = repositorySummary{
			Repo:             repo.Repo,
			Type:             repo.Type,
			Name:             repo.Name,
			Project:          repo.Project,
			ConnectionStatus: repo.ConnectionState.Status,
		}
	}
	return writeCompact(page{Items: summaries, TotalItems: total, HasMore: hasMore, Notice: notice})
}
