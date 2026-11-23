package tools

import (
	"context"
	"fmt"
	"time"

	eventspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/events"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const eventFilters = "resource_name, resource_namespace, or resource_uid"

// GetAppEventsTool defines the get_application_events tool schema.
var GetAppEventsTool = mcp.NewTool("get_application_events",
	mcp.WithDescription("Gets Kubernetes events for resources belonging to an ArgoCD application. detailed=false (the default) returns a page of summaries with type, reason, message, count, firstTimestamp, lastTimestamp, involvedKind, involvedNamespace, involvedName, and sourceComponent. metadata is omitted and the message is not clipped. limit defaults to 50 and offset to 0. The response has totalItems, hasMore, and a notice when the page is not the whole list. Narrow with resource_name, resource_namespace, or resource_uid. detailed=true pages the full events with metadata.managedFields and kubectl.kubernetes.io/last-applied-configuration removed."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve events for."),
	),
	mcp.WithString("resource_namespace",
		mcp.Description("Optional. Filter events by resource namespace."),
	),
	mcp.WithString("resource_name",
		mcp.Description("Optional. Filter events by resource name."),
	),
	mcp.WithString("resource_uid",
		mcp.Description("Optional. Filter events by resource UID."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Optional. The namespace where the ArgoCD application resource is located (for multi-tenant setups)."),
	),
	mcp.WithString("project",
		mcp.Description("Optional. The ArgoCD project the application belongs to."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns full events with managedFields removed. If false (default), returns summaries without metadata."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of events to return (default: 50). Use with offset to page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of events to skip (default: 0)."),
	),
)

// makeGetApplicationEvents returns the handler for this tool bound to the shared client in d.
func makeGetApplicationEvents(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit, offset, err := pageArgs(request, defaultPageLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return getApplicationEventsHandler(ctx, d.Client, client.EventsOptions{
			Name:              request.GetString("name", ""),
			ResourceNamespace: request.GetString("resource_namespace", ""),
			ResourceName:      request.GetString("resource_name", ""),
			ResourceUID:       request.GetString("resource_uid", ""),
			AppNamespace:      request.GetString("app_namespace", ""),
			Project:           request.GetString("project", ""),
		}, request.GetBool("detailed", false), limit, offset)
	}
}

// getApplicationEventsHandler handles the core logic for getting application events.
func getApplicationEventsHandler(
	ctx context.Context,
	argoClient client.Interface,
	opts client.EventsOptions,
	detailed bool,
	limit, offset int,
) (*mcp.CallToolResult, error) {
	if opts.Name == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}
	events, err := argoClient.GetApplicationEvents(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application events: %v", err)), nil
	}
	return writeCompact(eventsPage(events, detailed, limit, offset))
}

type eventSummary struct {
	Type              string `json:"type,omitempty"`
	Reason            string `json:"reason,omitempty"`
	Message           string `json:"message,omitempty"`
	Count             int32  `json:"count,omitempty"`
	FirstTimestamp    string `json:"firstTimestamp,omitempty"`
	LastTimestamp     string `json:"lastTimestamp,omitempty"`
	InvolvedKind      string `json:"involvedKind,omitempty"`
	InvolvedNamespace string `json:"involvedNamespace,omitempty"`
	InvolvedName      string `json:"involvedName,omitempty"`
	SourceComponent   string `json:"sourceComponent,omitempty"`
}

func eventsPage(list *eventspkg.EventList, detailed bool, limit, offset int) page {
	var items []eventspkg.Event
	if list != nil {
		items = list.Items
	}
	sliced, total, hasMore := pageSlice(items, limit, offset)
	p := page{
		TotalItems: total,
		HasMore:    hasMore,
		Notice:     pageNotice(offset, len(sliced), total, hasMore, eventFilters),
	}
	if detailed {
		out := make([]eventspkg.Event, len(sliced))
		for i, ev := range sliced {
			out[i] = stripEvent(ev)
		}
		p.Items = out
		return p
	}
	summaries := make([]eventSummary, len(sliced))
	for i, ev := range sliced {
		summaries[i] = eventSummary{
			Type:              ev.Type,
			Reason:            ev.Reason,
			Message:           ev.Message,
			Count:             ev.Count,
			FirstTimestamp:    rfc3339(ev.FirstTimestamp),
			LastTimestamp:     rfc3339(ev.LastTimestamp),
			InvolvedKind:      ev.InvolvedObject.Kind,
			InvolvedNamespace: ev.InvolvedObject.Namespace,
			InvolvedName:      ev.InvolvedObject.Name,
			SourceComponent:   ev.Source.Component,
		}
	}
	p.Items = summaries
	return p
}

func stripEvent(ev eventspkg.Event) eventspkg.Event {
	if ev.Metadata.Annotations != nil {
		ann := make(map[string]string, len(ev.Metadata.Annotations))
		for k, v := range ev.Metadata.Annotations {
			ann[k] = v
		}
		ev.Metadata.Annotations = ann
	}
	stripObjectMeta(&ev.Metadata)
	return ev
}

func rfc3339(t metav1.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Time.Format(time.RFC3339)
}
