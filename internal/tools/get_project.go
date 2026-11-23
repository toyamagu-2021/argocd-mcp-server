package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetProjectTool defines the get_project tool schema.
var GetProjectTool = mcp.NewTool("get_project",
	mcp.WithDescription("Retrieves an ArgoCD project. detailed=false (the default) returns one compact JSON summary with name, description, sourceRepoCount, destinationCount, and roleCount. Roles, tokens, sync windows, and allow-lists are omitted. detailed=true returns the full project as compact JSON without managedFields or the last-applied annotation."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the project to retrieve."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns the full project (can be very large). If false (default), returns the summary."),
	),
)

// makeGetProject returns the handler for this tool bound to the shared client in d.
func makeGetProject(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("Project name is required"), nil
		}
		return getProjectHandler(ctx, d.Client, name, request.GetBool("detailed", false))
	}
}

// getProjectHandler handles the core logic for getting a project.
// This is separated out to enable testing with mocked clients.
func getProjectHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
	detailed bool,
) (*mcp.CallToolResult, error) {
	project, err := argoClient.GetProject(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get project: %v", err)), nil
	}
	if project == nil {
		return writeCompact(nil)
	}
	if detailed {
		stripped := stripProject(*project)
		return writeCompact(&stripped)
	}
	return writeCompact(projectSummary{
		Name:             project.Name,
		Description:      project.Spec.Description,
		SourceRepoCount:  len(project.Spec.SourceRepos),
		DestinationCount: len(project.Spec.Destinations),
		RoleCount:        len(project.Spec.Roles),
	})
}
