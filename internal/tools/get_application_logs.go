package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationLogsToolDefinition defines the schema for the get_application_logs tool
var GetApplicationLogsToolDefinition = mcp.NewTool("get_application_logs",
	mcp.WithDescription("Retrieves logs from pods in an ArgoCD application. Returns log entries from the specified pod or container."),
	// Required parameters
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve logs from"),
	),
	// Optional parameters
	mcp.WithString("pod_name",
		mcp.Description("Optional. The name of the pod to retrieve logs from. If not specified, returns logs from the first available pod"),
	),
	mcp.WithString("container",
		mcp.Description("Optional. The name of the container to retrieve logs from. If not specified, uses the default container"),
	),
	mcp.WithString("namespace",
		mcp.Description("Optional. The namespace of the pod. If not specified, uses the application's destination namespace"),
	),
	mcp.WithString("resource_name",
		mcp.Description("Optional. The name of the resource (for resources that manage pods like Deployment, StatefulSet)"),
	),
	mcp.WithString("kind",
		mcp.Description("Optional. The kind of the resource (e.g., 'Pod', 'Deployment', 'StatefulSet')"),
	),
	mcp.WithString("group",
		mcp.Description("Optional. The API group of the resource (e.g., 'apps' for Deployments)"),
	),
	mcp.WithNumber("tail_lines",
		mcp.Description("Optional. Number of lines from the end of the logs to show. Defaults to 100"),
	),
	mcp.WithNumber("since_seconds",
		mcp.Description("Optional. Only return logs newer than this many seconds"),
	),
	mcp.WithBoolean("follow",
		mcp.Description("Optional. Whether to follow the log stream. Default is false"),
	),
	mcp.WithBoolean("previous",
		mcp.Description("Optional. Return logs from the previous terminated container. Default is false"),
	),
	mcp.WithString("filter",
		mcp.Description("Optional. Filter log lines using this regular expression"),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Optional. The namespace where the ArgoCD application resource is located (for multi-tenant setups)"),
	),
	mcp.WithString("project",
		mcp.Description("Optional. The ArgoCD project the application belongs to"),
	),
)

// HandleGetApplicationLogs processes get_application_logs tool requests
func HandleGetApplicationLogs(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Extract required parameters
	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	// Extract optional parameters
	podName := request.GetString("pod_name", "")
	container := request.GetString("container", "")
	namespace := request.GetString("namespace", "")
	resourceName := request.GetString("resource_name", "")
	kind := request.GetString("kind", "")
	group := request.GetString("group", "")
	filter := request.GetString("filter", "")
	appNamespace := request.GetString("app_namespace", "")
	project := request.GetString("project", "")

	// Extract numeric parameters with defaults
	tailLines := int64(request.GetInt("tail_lines", 100))

	var sinceSeconds *int64
	if ss := request.GetInt("since_seconds", 0); ss > 0 {
		sinceSecondsVal := int64(ss)
		sinceSeconds = &sinceSecondsVal
	}

	// Extract boolean parameters
	follow := request.GetBool("follow", false)
	previous := request.GetBool("previous", false)

	// Create gRPC client
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
	return getApplicationLogsHandler(ctx, argoClient, name, podName, container, namespace,
		resourceName, kind, group, tailLines, sinceSeconds, follow, previous, filter,
		appNamespace, project)
}

// getApplicationLogsHandler handles the core logic for retrieving application logs.
// This is separated out to enable testing with mocked clients.
func getApplicationLogsHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
	podName string,
	container string,
	namespace string,
	resourceName string,
	kind string,
	group string,
	tailLines int64,
	sinceSeconds *int64,
	follow bool,
	previous bool,
	filter string,
	appNamespace string,
	project string,
) (*mcp.CallToolResult, error) {
	// Call the GetApplicationLogs method
	logs, err := argoClient.GetApplicationLogs(ctx, name, podName, container, namespace,
		resourceName, kind, group, tailLines, sinceSeconds, follow, previous, filter,
		appNamespace, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application logs: %v", err)), nil
	}

	// Define types for the response
	type LogEntry struct {
		Timestamp string `json:"timestamp,omitempty"`
		PodName   string `json:"pod_name,omitempty"`
		Content   string `json:"content"`
	}

	type LogResponse struct {
		Application string     `json:"application"`
		PodName     string     `json:"pod_name,omitempty"`
		Container   string     `json:"container,omitempty"`
		TotalLines  int        `json:"total_lines"`
		Logs        []LogEntry `json:"logs"`
	}

	response := LogResponse{
		Application: name,
		PodName:     podName,
		Container:   container,
		Logs:        []LogEntry{},
	}

	// Collect log entries
	for {
		entry, err := logs.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// If we have some logs already, return them with a warning
			if len(response.Logs) > 0 {
				response.Logs = append(response.Logs, LogEntry{
					Content: fmt.Sprintf("[Warning: Log stream ended with error: %v]", err),
				})
				break
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read log stream: %v", err)), nil
		}

		// Check if this is the last entry
		if entry.GetLast() {
			break
		}

		logEntry := LogEntry{
			Content: strings.TrimRight(entry.GetContent(), "\n"),
		}

		if entry.TimeStampStr != nil && *entry.TimeStampStr != "" {
			logEntry.Timestamp = *entry.TimeStampStr
		}

		if entry.PodName != nil && *entry.PodName != "" {
			logEntry.PodName = *entry.PodName
		}

		response.Logs = append(response.Logs, logEntry)
	}

	response.TotalLines = len(response.Logs)

	// Convert to JSON for better readability in MCP responses
	jsonData, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
