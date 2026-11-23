package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// RollbackApplicationTool rolls an application back to a history entry.
var RollbackApplicationTool = mcp.NewTool("rollback_application",
	mcp.WithDescription("Rolls an ArgoCD application back to a previous deployment recorded in status.history (ApplicationService.Rollback). id is that history entry's id, including 0. Argo CD refuses the call when auto-sync is enabled. dry_run and prune default to false; prune false does not delete resources that the target revision no longer defines. The result is the full Application as compact JSON, without managedFields or the last-applied annotation. Call get_application when only the summary is needed."),
	mcp.WithDestructiveHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("Name of the Application."),
	),
	mcp.WithNumber("id",
		mcp.Required(),
		mcp.Description("History id from status.history[].id. Zero is valid. This is not a git revision."),
	),
	mcp.WithBoolean("dry_run",
		mcp.Description("Preview the rollback without changing the cluster (default: false)."),
	),
	mcp.WithBoolean("prune",
		mcp.Description("Delete resources that are not in the target revision (default: false)."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
	mcp.WithString("project",
		mcp.Description("Project used for the rollback RBAC check. Omit to use the Application's current project."),
	),
)

// makeRollbackApplication returns the handler bound to the shared client in d.
func makeRollbackApplication(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := historyIDArg(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return rollbackApplicationHandler(ctx, d.Client, client.RollbackOptions{
			Name:         request.GetString("name", ""),
			ID:           id,
			DryRun:       request.GetBool("dry_run", false),
			Prune:        request.GetBool("prune", false),
			AppNamespace: request.GetString("app_namespace", ""),
			Project:      request.GetString("project", ""),
		})
	}
}

func rollbackApplicationHandler(ctx context.Context, argoClient client.Interface, opts client.RollbackOptions) (*mcp.CallToolResult, error) {
	if opts.Name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	if opts.ID < 0 {
		return mcp.NewToolResultError("id must be zero or greater"), nil
	}
	app, err := argoClient.RollbackApplication(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to rollback application: %v", err)), nil
	}
	if app == nil {
		return writeCompact(nil)
	}
	stripObjectMeta(&app.ObjectMeta)
	return writeCompact(app)
}

func historyIDArg(request mcp.CallToolRequest) (int64, error) {
	val, ok := request.GetArguments()["id"]
	if !ok || val == nil {
		return 0, fmt.Errorf("id is required")
	}
	switch n := val.(type) {
	case int:
		if n < 0 {
			return 0, fmt.Errorf("id must be zero or greater")
		}
		return int64(n), nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("id must be zero or greater")
		}
		return n, nil
	case float64:
		if n < 0 || n != float64(int64(n)) {
			return 0, fmt.Errorf("id must be a non-negative integer")
		}
		return int64(n), nil
	default:
		return 0, fmt.Errorf("id must be a non-negative integer")
	}
}
