package tools

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetAppTool defines the get_application tool schema.
var GetAppTool = mcp.NewTool("get_application",
	mcp.WithDescription("Retrieves an ArgoCD application. detailed=false (the default) returns one compact JSON summary: name, namespace, project, source, sources, destination, syncStatus, healthStatus, operationStatus, syncRevision, syncRevisions, healthMessage, historyCount, healthCounts, syncCounts, and conditions. The summary omits spec, status.resources, and history bodies. Per-resource data is get_application_resource_tree and get_application_managed_resources. detailed=true returns the full Application as compact JSON with metadata.managedFields and kubectl.kubernetes.io/last-applied-configuration removed. Helm values stay, and that result can be very large."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns the full Application (can be very large). If false (default), returns the summary."),
	),
)

// makeGetApplication returns the handler for this tool bound to the shared client in d.
func makeGetApplication(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return getApplicationHandler(ctx, d.Client, request.GetString("name", ""), request.GetString("app_namespace", ""), request.GetBool("detailed", false))
	}
}

// getApplicationHandler handles the core logic for getting an application.
func getApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	appNamespace string,
	detailed bool,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	app, err := argoClient.GetApplication(ctx, appName, appNamespace)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application: %v", err)), nil
	}
	if app == nil {
		return writeCompact(nil)
	}
	if detailed {
		stripObjectMeta(&app.ObjectMeta)
		return writeCompact(app)
	}
	return writeCompact(applicationGetSummary(*app))
}

// applicationGetSummary is the default get_application body.
// It starts from the list row and adds the fields only get sets.
func applicationGetSummary(app v1alpha1.Application) ApplicationSummary {
	summary := applicationSummary(app)
	if n := len(app.Spec.Sources); n > 0 {
		summary.Sources = make([]ApplicationSourceBrief, n)
		for i := range app.Spec.Sources {
			summary.Sources[i] = applicationSourceBrief(app.Spec.Sources[i])
		}
	}
	summary.SyncRevision = app.Status.Sync.Revision
	if len(app.Status.Sync.Revisions) > 0 {
		summary.SyncRevisions = app.Status.Sync.Revisions
	}
	summary.HealthMessage = app.Status.Health.Message
	summary.HistoryCount = len(app.Status.History)
	if n := len(app.Status.Resources); n > 0 {
		summary.HealthCounts = make(map[string]int, n)
		summary.SyncCounts = make(map[string]int, n)
		for _, res := range app.Status.Resources {
			summary.SyncCounts[string(res.Status)]++
			healthStatus := ""
			if res.Health != nil {
				healthStatus = string(res.Health.Status)
			}
			summary.HealthCounts[healthStatus]++
		}
	}
	if n := len(app.Status.Conditions); n > 0 {
		summary.Conditions = make([]applicationConditionBrief, n)
		for i, c := range app.Status.Conditions {
			summary.Conditions[i] = applicationConditionBrief{Type: string(c.Type), Message: c.Message}
		}
	}
	return summary
}
