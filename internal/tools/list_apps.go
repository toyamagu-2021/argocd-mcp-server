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

// ListAppsTool defines the list_application tool schema
var ListAppsTool = mcp.NewTool("list_application",
	mcp.WithDescription("Lists ArgoCD applications with optional filters. detailed=false (the default) and output_format=tsv (the default) return a summary table. name_only=true returns names and wins over detailed. detailed=true returns a compact page of full Applications and can be large. limit defaults to 50 and offset to 0. A short page ends with a notice. Narrow with project, cluster, namespace, app_namespace, or selector."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("project",
		mcp.Description("Filter applications by project name."),
	),
	mcp.WithString("cluster",
		mcp.Description("Filter applications by destination cluster name or URL."),
	),
	mcp.WithString("namespace",
		mcp.Description("Filter applications by destination namespace (spec.destination.namespace). Not to be confused with app_namespace."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Only list Application resources in this namespace (their metadata.namespace), for Argo CD apps-in-any-namespace. Unrelated to the destination 'namespace' filter."),
	),
	mcp.WithString("selector",
		mcp.Description("Filter applications by a label selector (e.g., 'key=value')."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns complete application details including all resource information (can be very large). If false (default), returns only essential fields. Recommended: keep this as false to avoid fetching excessive resource data."),
	),
	mcp.WithBoolean("name_only",
		mcp.Description("If true, returns only application names. Takes precedence over 'detailed' option. Useful for getting a quick list of application names."),
	),
	mcp.WithString("output_format",
		mcp.Description("Output format for the response. Options: 'tsv' (default), 'json'. TSV format reduces response size by ~50% for large datasets."),
	),
	mcp.WithString("optional_fields",
		mcp.Description("Comma-separated additional fields to include in TSV output. Available options: 'namespace', 'source' (includes repoURL, path, targetRevision, chart), 'destination' (includes server, namespace), 'operation' (includes phase, message, startedAt), or individual fields like 'source-repo', 'dest-namespace'. Defaults to minimal output (name, project, syncStatus, healthStatus)."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of applications to return (default: 50). Use with offset to page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of matching applications to skip (default: 0)."),
	),
)

// makeListApplications returns the handler for this tool bound to the shared client in d.
func makeListApplications(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Safely extract optional parameters from mcp.CallToolRequest
		project := request.GetString("project", "")
		cluster := request.GetString("cluster", "")
		namespace := request.GetString("namespace", "")
		appNamespace := request.GetString("app_namespace", "")
		selector := request.GetString("selector", "")
		detailed := request.GetBool("detailed", false)
		nameOnly := request.GetBool("name_only", false)
		outputFormat := request.GetString("output_format", "tsv")
		optionalFieldsStr := request.GetString("optional_fields", "")
		var optionalFields []string
		if optionalFieldsStr != "" {
			optionalFields = strings.Split(optionalFieldsStr, ",")
			// Trim whitespace from each field
			for i, field := range optionalFields {
				optionalFields[i] = strings.TrimSpace(field)
			}
		}

		limit, offset, err := pageArgs(request, defaultPageLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return listApplicationsHandler(ctx, d.Client, project, cluster, namespace, selector, detailed, nameOnly, outputFormat, optionalFields, appNamespace, limit, offset)
	}
}

// ApplicationSummary represents a simplified view of an application.
// Fields below OperationStatus are set by get_application only.
// The list builder leaves them empty, and omitempty keeps them off a list row.
type ApplicationSummary struct {
	Name            string                      `json:"name"`
	Namespace       string                      `json:"namespace"`
	Project         string                      `json:"project"`
	Source          ApplicationSourceBrief      `json:"source"`
	Sources         []ApplicationSourceBrief    `json:"sources,omitempty"`
	Destination     ApplicationDestination      `json:"destination"`
	SyncStatus      string                      `json:"syncStatus"`
	HealthStatus    string                      `json:"healthStatus"`
	OperationStatus *ApplicationOperation       `json:"operationStatus,omitempty"`
	SyncRevision    string                      `json:"syncRevision,omitempty"`
	SyncRevisions   []string                    `json:"syncRevisions,omitempty"`
	HealthMessage   string                      `json:"healthMessage,omitempty"`
	HistoryCount    int                         `json:"historyCount,omitempty"`
	HealthCounts    map[string]int              `json:"healthCounts,omitempty"`
	SyncCounts      map[string]int              `json:"syncCounts,omitempty"`
	Conditions      []applicationConditionBrief `json:"conditions,omitempty"`
}

// applicationConditionBrief is one status.conditions entry on the get summary.
type applicationConditionBrief struct {
	Type    string `json:"type,omitempty"`
	Message string `json:"message,omitempty"`
}

// ApplicationSourceBrief contains essential source information
type ApplicationSourceBrief struct {
	RepoURL        string `json:"repoURL"`
	Path           string `json:"path,omitempty"`
	TargetRevision string `json:"targetRevision,omitempty"`
	Chart          string `json:"chart,omitempty"`
}

// ApplicationDestination contains destination information
type ApplicationDestination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
}

// ApplicationOperation contains operation status information
type ApplicationOperation struct {
	Phase     string `json:"phase,omitempty"`
	Message   string `json:"message,omitempty"`
	StartedAt string `json:"startedAt,omitempty"`
}

const listApplicationFilters = "project, cluster, namespace, app_namespace, or selector"

// listApplicationsHandler handles the core logic for listing applications.
// This is separated out to enable testing with mocked clients.
func listApplicationsHandler(
	ctx context.Context,
	argoClient client.Interface,
	project, cluster, namespace, selector string,
	detailed bool,
	nameOnly bool,
	outputFormat string,
	optionalFields []string,
	appNamespace string,
	limit, offset int,
) (*mcp.CallToolResult, error) {
	opts := client.ListApplicationsOptions{Selector: selector, AppNamespace: appNamespace}
	if project != "" {
		opts.Projects = []string{project}
	}
	appList, err := argoClient.ListApplications(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list applications: %v", err)), nil
	}

	apps := appList.Items

	// Filter by project, cluster, namespace if specified
	var filteredApps []v1alpha1.Application
	for _, app := range apps {
		if project != "" && app.Spec.Project != project {
			continue
		}
		if cluster != "" && app.Spec.Destination.Server != cluster {
			continue
		}
		if namespace != "" && app.Spec.Destination.Namespace != namespace {
			continue
		}
		filteredApps = append(filteredApps, app)
	}

	if filteredApps == nil {
		filteredApps = []v1alpha1.Application{}
	}
	pageApps, total, hasMore := pageSlice(filteredApps, limit, offset)
	notice := pageNotice(offset, len(pageApps), total, hasMore, listApplicationFilters)

	if nameOnly && (outputFormat == "tsv" || outputFormat == "") {
		return generateNameOnlyTSV(pageApps, notice), nil
	}
	if nameOnly {
		names := make([]string, len(pageApps))
		for i, app := range pageApps {
			names[i] = app.Name
		}
		return writeCompact(page{Items: names, TotalItems: total, HasMore: hasMore, Notice: notice})
	}
	if detailed {
		items := make([]v1alpha1.Application, len(pageApps))
		for i, app := range pageApps {
			items[i] = stripListedApplication(app)
		}
		return writeCompact(page{Items: items, TotalItems: total, HasMore: hasMore, Notice: notice})
	}
	if outputFormat == "tsv" || outputFormat == "" {
		return generateSummaryTSV(pageApps, optionalFields, notice), nil
	}
	summaries := make([]ApplicationSummary, len(pageApps))
	for i, app := range pageApps {
		summaries[i] = applicationSummary(app)
	}
	return writeCompact(page{Items: summaries, TotalItems: total, HasMore: hasMore, Notice: notice})
}

func stripListedApplication(app v1alpha1.Application) v1alpha1.Application {
	if app.Annotations != nil {
		ann := make(map[string]string, len(app.Annotations))
		for k, v := range app.Annotations {
			ann[k] = v
		}
		app.Annotations = ann
	}
	stripObjectMeta(&app.ObjectMeta)
	return app
}

func applicationSummary(app v1alpha1.Application) ApplicationSummary {
	summary := ApplicationSummary{
		Name:      app.Name,
		Namespace: app.Namespace,
		Project:   app.Spec.Project,
		Destination: ApplicationDestination{
			Server:    app.Spec.Destination.Server,
			Namespace: app.Spec.Destination.Namespace,
		},
	}
	if app.Spec.Source != nil {
		summary.Source = applicationSourceBrief(*app.Spec.Source)
	}
	if app.Status.Sync.Status != "" {
		summary.SyncStatus = string(app.Status.Sync.Status)
	}
	if app.Status.Health.Status != "" {
		summary.HealthStatus = string(app.Status.Health.Status)
	}
	if app.Status.OperationState != nil {
		summary.OperationStatus = &ApplicationOperation{
			Phase:   string(app.Status.OperationState.Phase),
			Message: app.Status.OperationState.Message,
		}
		if !app.Status.OperationState.StartedAt.IsZero() {
			summary.OperationStatus.StartedAt = app.Status.OperationState.StartedAt.String()
		}
	}
	return summary
}

func applicationSourceBrief(src v1alpha1.ApplicationSource) ApplicationSourceBrief {
	return ApplicationSourceBrief{
		RepoURL:        src.RepoURL,
		Path:           src.Path,
		TargetRevision: src.TargetRevision,
		Chart:          src.Chart,
	}
}
