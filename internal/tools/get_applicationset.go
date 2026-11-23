package tools

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationSetTool defines the get_applicationset tool schema.
var GetApplicationSetTool = mcp.NewTool("get_applicationset",
	mcp.WithDescription("Retrieves an ArgoCD ApplicationSet. detailed=false (the default) returns one compact JSON summary with name, namespace, project, and generatorCount. Generators and the template, including Helm values, are omitted. detailed=true returns the full ApplicationSet as compact JSON without managedFields or the last-applied annotation."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the ApplicationSet to retrieve."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns the full ApplicationSet, including generators and Helm values (can be very large). If false (default), returns the summary."),
	),
)

// makeGetApplicationSet returns the handler for this tool bound to the shared client in d.
func makeGetApplicationSet(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("name is required"), nil
		}
		return getApplicationSetHandler(ctx, d.Client, name, request.GetBool("detailed", false))
	}
}

// getApplicationSetHandler handles the core logic for getting an ApplicationSet.
// This is separated out to enable testing with mocked clients.
func getApplicationSetHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
	detailed bool,
) (*mcp.CallToolResult, error) {
	appSet, err := argoClient.GetApplicationSet(ctx, name)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get ApplicationSet: %v", err)), nil
	}
	if appSet == nil {
		return writeCompact(nil)
	}
	if detailed {
		stripped := stripApplicationSet(*appSet)
		return writeCompact(&stripped)
	}
	return writeCompact(applicationSetSummary{
		Name:           appSet.Name,
		Namespace:      appSet.Namespace,
		Project:        appSet.Spec.Template.Spec.Project,
		GeneratorCount: len(appSet.Spec.Generators),
	})
}
