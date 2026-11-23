package tools

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationLogsToolDefinition defines the schema for the get_application_logs tool.
var GetApplicationLogsToolDefinition = mcp.NewTool("get_application_logs",
	mcp.WithDescription("Retrieves logs from pods in an ArgoCD application. detailed=false (the default) returns plain text, not JSON. tail_lines defaults to 20. The server does not report the total log size. Pass a larger tail_lines to see more. detailed=true returns compact JSON with the same tail. Streaming (follow=true) is not supported over stdio and is rejected. Use tail_lines or since_seconds instead."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve logs from"),
	),
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
		mcp.Description("Optional. Number of lines from the end of the logs to show. Defaults to 20"),
	),
	mcp.WithNumber("since_seconds",
		mcp.Description("Optional. Only return logs newer than this many seconds"),
	),
	mcp.WithBoolean("follow",
		mcp.Description("Not supported: must be false or omitted. follow=true would never return over the stdio transport and is rejected with an error."),
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
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns compact JSON. If false (default), returns plain text. detailed does not change tail_lines."),
	),
)

// makeGetApplicationLogs returns the handler for this tool bound to the shared client in d.
func makeGetApplicationLogs(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("name is required"), nil
		}
		if request.GetBool("follow", false) {
			return mcp.NewToolResultError(errFollowUnsupported), nil
		}

		var sinceSeconds *int64
		if ss := request.GetInt("since_seconds", 0); ss > 0 {
			sinceSecondsVal := int64(ss)
			sinceSeconds = &sinceSecondsVal
		}

		return getApplicationLogsHandler(ctx, d.Client, client.LogsOptions{
			Name:         name,
			PodName:      request.GetString("pod_name", ""),
			Container:    request.GetString("container", ""),
			Namespace:    request.GetString("namespace", ""),
			ResourceName: request.GetString("resource_name", ""),
			Kind:         request.GetString("kind", ""),
			Group:        request.GetString("group", ""),
			TailLines:    int64(request.GetInt("tail_lines", defaultTailLines)),
			SinceSeconds: sinceSeconds,
			Follow:       false,
			Previous:     request.GetBool("previous", false),
			Filter:       request.GetString("filter", ""),
			AppNamespace: request.GetString("app_namespace", ""),
			Project:      request.GetString("project", ""),
		}, request.GetBool("detailed", false))
	}
}

// errFollowUnsupported is returned for follow=true (design D18): a never-ending
// log stream would block the stdio tool call forever.
const errFollowUnsupported = "follow=true is not supported: streaming logs cannot be returned over the stdio MCP transport. Omit follow (or set it to false) and use tail_lines / since_seconds instead."

type logEntry struct {
	Timestamp string `json:"timestamp,omitempty"`
	PodName   string `json:"pod_name,omitempty"`
	Content   string `json:"content"`
}

type logResponse struct {
	Application string     `json:"application"`
	PodName     string     `json:"pod_name,omitempty"`
	Container   string     `json:"container,omitempty"`
	TotalLines  int        `json:"total_lines"`
	Logs        []logEntry `json:"logs"`
	Notice      string     `json:"notice,omitempty"`
}

// getApplicationLogsHandler handles the core logic for retrieving application logs.
func getApplicationLogsHandler(
	ctx context.Context,
	argoClient client.Interface,
	opts client.LogsOptions,
	detailed bool,
) (*mcp.CallToolResult, error) {
	if opts.Follow {
		return mcp.NewToolResultError(errFollowUnsupported), nil
	}

	logs, err := argoClient.GetApplicationLogs(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application logs: %v", err)), nil
	}

	response := logResponse{
		Application: opts.Name,
		PodName:     opts.PodName,
		Container:   opts.Container,
		Logs:        []logEntry{},
	}

	for {
		entry, err := logs.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if len(response.Logs) > 0 {
				response.Logs = append(response.Logs, logEntry{
					Content: fmt.Sprintf("[Warning: Log stream ended with error: %v]", err),
				})
				break
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to read log stream: %v", err)), nil
		}
		if entry.GetLast() {
			break
		}

		logLine := logEntry{Content: strings.TrimRight(entry.GetContent(), "\n")}
		if entry.TimeStampStr != nil && *entry.TimeStampStr != "" {
			logLine.Timestamp = *entry.TimeStampStr
		}
		if entry.PodName != nil && *entry.PodName != "" {
			logLine.PodName = *entry.PodName
		}
		response.Logs = append(response.Logs, logLine)
	}

	response.TotalLines = len(response.Logs)
	notice := tailNotice(response.TotalLines, int(opts.TailLines))
	if detailed {
		response.Notice = notice
		return writeCompact(response)
	}
	return mcp.NewToolResultText(formatLogText(response, notice)), nil
}

func formatLogText(response logResponse, notice string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "application=%s pod=%s container=%s lines=%d\n", response.Application, response.PodName, response.Container, response.TotalLines)
	for _, line := range response.Logs {
		fmt.Fprintf(&b, "%s\t%s\t%s\n", line.Timestamp, line.PodName, line.Content)
	}
	if notice != "" {
		b.WriteString(notice)
		b.WriteByte('\n')
	}
	return b.String()
}
