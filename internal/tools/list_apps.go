package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListAppsTool defines the list_application tool schema
var ListAppsTool = mcp.NewTool("list_application",
	mcp.WithDescription("Lists all ArgoCD applications, with optional filters. Note: When detailed=true, this fetches complete application information including all resource details, which can be very large. It's recommended to use detailed=false (default) for better performance and to avoid excessive data transfer."),
	mcp.WithString("project",
		mcp.Description("Filter applications by project name."),
	),
	mcp.WithString("cluster",
		mcp.Description("Filter applications by destination cluster name or URL."),
	),
	mcp.WithString("namespace",
		mcp.Description("Filter applications by destination namespace."),
	),
	mcp.WithString("selector",
		mcp.Description("Filter applications by a label selector (e.g., 'key=value')."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns complete application details including all resource information (can be very large). If false (default), returns only essential fields. Recommended: keep this as false to avoid fetching excessive resource data."),
	),
)

// HandleListApplications processes list_application tool requests
func HandleListApplications(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Safely extract optional parameters from mcp.CallToolRequest
	project := request.GetString("project", "")
	cluster := request.GetString("cluster", "")
	namespace := request.GetString("namespace", "")
	selector := request.GetString("selector", "")
	detailed := request.GetBool("detailed", false)

	// Create gRPC client and list applications
	config := &client.Config{
		ServerAddr:      os.Getenv("ARGOCD_SERVER"),
		AuthToken:       os.Getenv("ARGOCD_AUTH_TOKEN"),
		Insecure:        os.Getenv("ARGOCD_INSECURE") == "true",
		PlainText:       os.Getenv("ARGOCD_PLAINTEXT") == "true",
		GRPCWeb:         os.Getenv("ARGOCD_GRPC_WEB") == "true",
		GRPCWebRootPath: os.Getenv("ARGOCD_GRPC_WEB_ROOT_PATH"),
	}

	argoClient, err := client.New(config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create gRPC client: %v", err)), nil
	}
	defer func() { _ = argoClient.Close() }()

	// Use the handler function with the real client
	return listApplicationsHandler(ctx, argoClient, project, cluster, namespace, selector, detailed)
}

// ApplicationSummary represents a simplified view of an application
type ApplicationSummary struct {
	Name            string                 `json:"name"`
	Namespace       string                 `json:"namespace"`
	Project         string                 `json:"project"`
	Source          ApplicationSourceBrief `json:"source"`
	Destination     ApplicationDestination `json:"destination"`
	SyncStatus      string                 `json:"syncStatus"`
	HealthStatus    string                 `json:"healthStatus"`
	OperationStatus *ApplicationOperation  `json:"operationStatus,omitempty"`
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

// listApplicationsHandler handles the core logic for listing applications.
// This is separated out to enable testing with mocked clients.
func listApplicationsHandler(
	ctx context.Context,
	argoClient client.Interface,
	project, cluster, namespace, selector string,
	detailed bool,
) (*mcp.CallToolResult, error) {
	appList, err := argoClient.ListApplications(ctx, selector)
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

	if len(filteredApps) == 0 {
		return mcp.NewToolResultText("No applications found matching the criteria."), nil
	}

	var jsonData []byte

	if detailed {
		// Return full application details
		jsonData, err = json.MarshalIndent(filteredApps, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
	} else {
		// Return summarized application information
		summaries := make([]ApplicationSummary, 0, len(filteredApps))
		for _, app := range filteredApps {
			summary := ApplicationSummary{
				Name:      app.Name,
				Namespace: app.Namespace,
				Project:   app.Spec.Project,
				Destination: ApplicationDestination{
					Server:    app.Spec.Destination.Server,
					Namespace: app.Spec.Destination.Namespace,
				},
			}

			// Add source information if available
			if app.Spec.Source != nil {
				summary.Source = ApplicationSourceBrief{
					RepoURL:        app.Spec.Source.RepoURL,
					Path:           app.Spec.Source.Path,
					TargetRevision: app.Spec.Source.TargetRevision,
					Chart:          app.Spec.Source.Chart,
				}
			}

			// Add sync status if available
			if app.Status.Sync.Status != "" {
				summary.SyncStatus = string(app.Status.Sync.Status)
			}

			// Add health status if available
			if app.Status.Health.Status != "" {
				summary.HealthStatus = string(app.Status.Health.Status)
			}

			// Add operation status if an operation is in progress
			if app.Status.OperationState != nil {
				summary.OperationStatus = &ApplicationOperation{
					Phase:   string(app.Status.OperationState.Phase),
					Message: app.Status.OperationState.Message,
				}
				if !app.Status.OperationState.StartedAt.IsZero() {
					summary.OperationStatus.StartedAt = app.Status.OperationState.StartedAt.String()
				}
			}

			summaries = append(summaries, summary)
		}

		jsonData, err = json.MarshalIndent(summaries, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
