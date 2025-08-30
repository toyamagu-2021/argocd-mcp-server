package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListAppsTool defines the list_application tool schema
var ListAppsTool = mcp.NewTool("list_application",
	mcp.WithDescription("Lists ArgoCD applications with optional filters. Use name_only=true for just names, detailed=true for full info (can be large), or default for summary view."),
	mcp.WithDestructiveHintAnnotation(false),
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
	mcp.WithBoolean("name_only",
		mcp.Description("If true, returns only application names. Takes precedence over 'detailed' option. Useful for getting a quick list of application names."),
	),
	mcp.WithString("output_format",
		mcp.Description("Output format for the response. Options: 'tsv' (default), 'json'. TSV format reduces response size by ~50% for large datasets."),
	),
	mcp.WithString("optional_fields",
		mcp.Description("Comma-separated additional fields to include in TSV output. Available options: 'namespace', 'source' (includes repoURL, path, targetRevision, chart), 'destination' (includes server, namespace), 'operation' (includes phase, message, startedAt), or individual fields like 'source-repo', 'dest-namespace'. Defaults to minimal output (name, project, syncStatus, healthStatus)."),
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
	return listApplicationsHandler(ctx, argoClient, project, cluster, namespace, selector, detailed, nameOnly, outputFormat, optionalFields)
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

// ApplicationNameList represents a list of application names
type ApplicationNameList struct {
	Names []string `json:"names"`
	Count int      `json:"count"`
}

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

	// Handle TSV format
	if outputFormat == "tsv" {
		if nameOnly {
			return generateNameOnlyTSV(filteredApps), nil
		} else if detailed {
			return generateDetailedTSV(filteredApps), nil
		} else {
			return generateSummaryTSV(filteredApps, optionalFields), nil
		}
	}

	// Handle JSON format (default)
	var jsonData []byte

	if nameOnly {
		// Return only application names
		names := make([]string, 0, len(filteredApps))
		for _, app := range filteredApps {
			names = append(names, app.Name)
		}
		nameList := ApplicationNameList{
			Names: names,
			Count: len(names),
		}
		jsonData, err = json.MarshalIndent(nameList, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
	} else if detailed {
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

// FieldConfig represents which fields to include in TSV output
type FieldConfig struct {
	IncludeNamespace   bool
	IncludeSourceRepo  bool
	IncludeSourcePath  bool
	IncludeSourceRev   bool
	IncludeSourceChart bool
	IncludeDestServer  bool
	IncludeDestNs      bool
	IncludeOpPhase     bool
	IncludeOpMessage   bool
	IncludeOpStartedAt bool
}

// buildFieldConfig determines which fields to include based on optional_fields parameter
func buildFieldConfig(optionalFields []string) FieldConfig {
	config := FieldConfig{} // Start with minimal config (name, project, syncStatus, healthStatus only)

	for _, field := range optionalFields {
		switch field {
		case "namespace":
			config.IncludeNamespace = true
		case "source":
			config.IncludeSourceRepo = true
			config.IncludeSourcePath = true
			config.IncludeSourceRev = true
			config.IncludeSourceChart = true
		case "source-repo":
			config.IncludeSourceRepo = true
		case "source-path":
			config.IncludeSourcePath = true
		case "source-revision":
			config.IncludeSourceRev = true
		case "source-chart":
			config.IncludeSourceChart = true
		case "destination":
			config.IncludeDestServer = true
			config.IncludeDestNs = true
		case "dest-server":
			config.IncludeDestServer = true
		case "dest-namespace":
			config.IncludeDestNs = true
		case "operation":
			config.IncludeOpPhase = true
			config.IncludeOpMessage = true
			config.IncludeOpStartedAt = true
		case "op-phase":
			config.IncludeOpPhase = true
		case "op-message":
			config.IncludeOpMessage = true
		case "op-started":
			config.IncludeOpStartedAt = true
		}
	}

	return config
}

// buildHeaders creates the TSV header based on field configuration
func buildHeaders(config FieldConfig) []string {
	headers := []string{"name", "project", "syncStatus", "healthStatus"} // Always include these minimal fields

	if config.IncludeNamespace {
		headers = append(headers, "namespace")
	}
	if config.IncludeSourceRepo {
		headers = append(headers, "repoURL")
	}
	if config.IncludeSourcePath {
		headers = append(headers, "path")
	}
	if config.IncludeSourceRev {
		headers = append(headers, "targetRevision")
	}
	if config.IncludeSourceChart {
		headers = append(headers, "chart")
	}
	if config.IncludeDestServer {
		headers = append(headers, "destServer")
	}
	if config.IncludeDestNs {
		headers = append(headers, "destNamespace")
	}
	if config.IncludeOpPhase {
		headers = append(headers, "opPhase")
	}
	if config.IncludeOpMessage {
		headers = append(headers, "opMessage")
	}
	if config.IncludeOpStartedAt {
		headers = append(headers, "opStartedAt")
	}

	return headers
}

// buildFieldValues extracts field values from an application based on configuration
func buildFieldValues(app v1alpha1.Application, config FieldConfig) []string {
	// Always include minimal fields
	syncStatus := ""
	if app.Status.Sync.Status != "" {
		syncStatus = string(app.Status.Sync.Status)
	}

	healthStatus := ""
	if app.Status.Health.Status != "" {
		healthStatus = string(app.Status.Health.Status)
	}

	fields := []string{
		escapeField(app.Name),
		escapeField(app.Spec.Project),
		escapeField(syncStatus),
		escapeField(healthStatus),
	}

	if config.IncludeNamespace {
		fields = append(fields, escapeField(app.Namespace))
	}
	if config.IncludeSourceRepo {
		if app.Spec.Source != nil {
			fields = append(fields, escapeField(app.Spec.Source.RepoURL))
		} else {
			fields = append(fields, "")
		}
	}
	if config.IncludeSourcePath {
		if app.Spec.Source != nil {
			fields = append(fields, escapeField(app.Spec.Source.Path))
		} else {
			fields = append(fields, "")
		}
	}
	if config.IncludeSourceRev {
		if app.Spec.Source != nil {
			fields = append(fields, escapeField(app.Spec.Source.TargetRevision))
		} else {
			fields = append(fields, "")
		}
	}
	if config.IncludeSourceChart {
		if app.Spec.Source != nil {
			fields = append(fields, escapeField(app.Spec.Source.Chart))
		} else {
			fields = append(fields, "")
		}
	}
	if config.IncludeDestServer {
		fields = append(fields, escapeField(app.Spec.Destination.Server))
	}
	if config.IncludeDestNs {
		fields = append(fields, escapeField(app.Spec.Destination.Namespace))
	}
	if config.IncludeOpPhase {
		if app.Status.OperationState != nil {
			fields = append(fields, escapeField(string(app.Status.OperationState.Phase)))
		} else {
			fields = append(fields, "")
		}
	}
	if config.IncludeOpMessage {
		if app.Status.OperationState != nil {
			fields = append(fields, escapeField(app.Status.OperationState.Message))
		} else {
			fields = append(fields, "")
		}
	}
	if config.IncludeOpStartedAt {
		if app.Status.OperationState != nil && !app.Status.OperationState.StartedAt.IsZero() {
			fields = append(fields, escapeField(app.Status.OperationState.StartedAt.String()))
		} else {
			fields = append(fields, "")
		}
	}

	return fields
}

// escapeField escapes tabs and newlines in TSV field values
func escapeField(field string) string {
	field = strings.ReplaceAll(field, "\t", "\\t")
	field = strings.ReplaceAll(field, "\n", "\\n")
	field = strings.ReplaceAll(field, "\r", "\\r")
	return field
}

// generateNameOnlyTSV generates TSV output for name-only mode
func generateNameOnlyTSV(apps []v1alpha1.Application) *mcp.CallToolResult {
	var result strings.Builder

	for i, app := range apps {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(escapeField(app.Name))
	}

	return mcp.NewToolResultText(result.String())
}

// generateSummaryTSV generates TSV output for summary mode with optional fields
func generateSummaryTSV(apps []v1alpha1.Application, optionalFields []string) *mcp.CallToolResult {
	var result strings.Builder

	// Determine which fields to include
	fieldConfig := buildFieldConfig(optionalFields)
	headers := buildHeaders(fieldConfig)
	result.WriteString(strings.Join(headers, "\t") + "\n")

	// Data rows
	for _, app := range apps {
		fields := buildFieldValues(app, fieldConfig)
		line := strings.Join(fields, "\t")
		result.WriteString(line + "\n")
	}

	return mcp.NewToolResultText(result.String())
}

// generateDetailedTSV generates TSV output for detailed mode
func generateDetailedTSV(apps []v1alpha1.Application) *mcp.CallToolResult {
	// For detailed mode, we'll serialize the full JSON structure as TSV is not suitable
	// for deeply nested objects. Instead, we'll use a more compact JSON representation.
	jsonData, err := json.Marshal(apps)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err))
	}

	return mcp.NewToolResultText(string(jsonData))
}
