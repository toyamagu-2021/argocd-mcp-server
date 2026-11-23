package tools

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetApplicationResourceTreeTool defines the tool for retrieving application resource tree.
var GetApplicationResourceTreeTool = mcp.NewTool("get_application_resource_tree",
	mcp.WithDescription("Returns the resource tree of an ArgoCD application. detailed=false (the default) returns a page of node summaries: group, version, kind, namespace, name, uid, health status, and parent refs without uid. Orphaned nodes follow managed nodes and set orphaned to true. Hosts are names. info, networkingInfo, images, resourceVersion, and createdAt are omitted. limit defaults to 50 and offset to 0. The response has totalItems, hasMore, and a notice when the page is not the whole tree. detailed=true pages the full nodes and repeats full host info, including systemInfo, on every page. That result can be very large."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("Name of the application"),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the application (optional, will be auto-detected if not provided)"),
	),
	mcp.WithString("project",
		mcp.Description("Project of the application (optional, will be auto-detected if not provided)"),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns full nodes and full hosts, including systemInfo (can be very large). If false (default), returns node summaries and host names."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of nodes to return (default: 50). Use with offset to page. Hosts are repeated on every page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of nodes to skip (default: 0)."),
	),
)

// makeGetApplicationResourceTree returns the handler for this tool bound to the shared client in d.
func makeGetApplicationResourceTree(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit, offset, err := pageArgs(request, defaultPageLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return getApplicationResourceTreeHandler(
			ctx,
			d.Client,
			request.GetString("name", ""),
			request.GetString("app_namespace", ""),
			request.GetString("project", ""),
			request.GetBool("detailed", false),
			limit,
			offset,
		)
	}
}

// getApplicationResourceTreeHandler handles the core logic for the tool.
func getApplicationResourceTreeHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
	appNamespace string,
	project string,
	detailed bool,
	limit, offset int,
) (*mcp.CallToolResult, error) {
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	tree, err := argoClient.GetApplicationResourceTree(ctx, name, appNamespace, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application resource tree: %v", err)), nil
	}
	return writeCompact(resourceTreePage(tree, detailed, limit, offset))
}

// resourceTreeView is one page of the resource tree. Hosts is repeated on every page.
// Summary hosts are name strings. Detailed hosts are the full host records.
type resourceTreeView struct {
	page
	Hosts any `json:"hosts,omitempty"`
}

type resourceNodeSummary struct {
	Group      string              `json:"group,omitempty"`
	Version    string              `json:"version,omitempty"`
	Kind       string              `json:"kind,omitempty"`
	Namespace  string              `json:"namespace,omitempty"`
	Name       string              `json:"name,omitempty"`
	UID        string              `json:"uid,omitempty"`
	Health     string              `json:"health,omitempty"`
	ParentRefs []resourceParentRef `json:"parentRefs,omitempty"`
	Orphaned   bool                `json:"orphaned,omitempty"`
}

type resourceParentRef struct {
	Group     string `json:"group,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
}

type resourceNodeDetailed struct {
	v1alpha1.ResourceNode `json:",inline"`
	Orphaned              bool `json:"orphaned,omitempty"`
}

func resourceTreePage(tree *v1alpha1.ApplicationTree, detailed bool, limit, offset int) resourceTreeView {
	var nodes, orphaned []v1alpha1.ResourceNode
	var hosts []v1alpha1.HostInfo
	if tree != nil {
		nodes = tree.Nodes
		orphaned = tree.OrphanedNodes
		hosts = tree.Hosts
	}
	combined := make([]v1alpha1.ResourceNode, 0, len(nodes)+len(orphaned))
	combined = append(combined, nodes...)
	combined = append(combined, orphaned...)
	sliced, total, hasMore := pageSlice(combined, limit, offset)
	view := resourceTreeView{page: page{
		TotalItems: total,
		HasMore:    hasMore,
		Notice:     pageNotice(offset, len(sliced), total, hasMore, ""),
	}}
	orphanStart := len(nodes)
	if detailed {
		items := make([]resourceNodeDetailed, len(sliced))
		for i, node := range sliced {
			items[i] = resourceNodeDetailed{ResourceNode: node, Orphaned: offset+i >= orphanStart}
		}
		view.Items = items
		if len(hosts) > 0 {
			view.Hosts = hosts
		}
		return view
	}
	items := make([]resourceNodeSummary, len(sliced))
	for i, node := range sliced {
		items[i] = resourceNodeSummary{
			Group:      node.Group,
			Version:    node.Version,
			Kind:       node.Kind,
			Namespace:  node.Namespace,
			Name:       node.Name,
			UID:        node.UID,
			ParentRefs: parentRefsWithoutUID(node.ParentRefs),
			Orphaned:   offset+i >= orphanStart,
		}
		if node.Health != nil {
			items[i].Health = string(node.Health.Status)
		}
	}
	view.Items = items
	if len(hosts) > 0 {
		names := make([]string, len(hosts))
		for i := range hosts {
			names[i] = hosts[i].Name
		}
		view.Hosts = names
	}
	return view
}

func parentRefsWithoutUID(refs []v1alpha1.ResourceRef) []resourceParentRef {
	if len(refs) == 0 {
		return nil
	}
	out := make([]resourceParentRef, len(refs))
	for i, ref := range refs {
		out[i] = resourceParentRef{Group: ref.Group, Kind: ref.Kind, Namespace: ref.Namespace, Name: ref.Name}
	}
	return out
}
