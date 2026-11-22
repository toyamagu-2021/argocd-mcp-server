package tools

// TSV rendering for list_application.

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
)

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
