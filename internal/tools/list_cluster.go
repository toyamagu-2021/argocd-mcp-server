package tools

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListClusterTool provides MCP tool for listing all ArgoCD clusters.
var ListClusterTool = mcp.NewTool("list_cluster",
	mcp.WithDescription("Lists ArgoCD clusters. detailed=false (the default) returns a page of summaries. name_only=true returns name and server objects and wins over detailed. detailed=true returns a page of full clusters after Sanitized, without kubectl.kubernetes.io/last-applied-configuration. limit defaults to 50 and offset to 0."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns complete cluster details (can be very large). If false (default), returns only essential fields."),
	),
	mcp.WithBoolean("name_only",
		mcp.Description("If true, returns only cluster names and servers inside items. Takes precedence over detailed."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of clusters to return (default: 50). Use with offset to page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of clusters to skip (default: 0)."),
	),
)

// ClusterSummary represents a simplified view of a cluster.
type ClusterSummary struct {
	Name             string                    `json:"name"`
	Server           string                    `json:"server"`
	ServerVersion    string                    `json:"serverVersion,omitempty"`
	ConnectionStatus v1alpha1.ConnectionStatus `json:"connectionStatus"`
}

// ClusterIdentifier contains minimal cluster identification.
type ClusterIdentifier struct {
	Name   string `json:"name"`
	Server string `json:"server"`
}

// makeListCluster returns the handler for this tool bound to the shared client in d.
func makeListCluster(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit, offset, err := pageArgs(request, defaultPageLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return listClusterHandler(ctx, d.Client, request.GetBool("detailed", false), request.GetBool("name_only", false), limit, offset)
	}
}

func listClusterHandler(
	ctx context.Context,
	argoClient client.Interface,
	detailed bool,
	nameOnly bool,
	limit, offset int,
) (*mcp.CallToolResult, error) {
	clusters, err := argoClient.ListClusters(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list clusters: %v", err)), nil
	}
	var items []v1alpha1.Cluster
	if clusters != nil {
		items = clusters.Items
	}
	if items == nil {
		items = []v1alpha1.Cluster{}
	}
	sliced, total, hasMore := pageSlice(items, limit, offset)
	notice := pageNotice(offset, len(sliced), total, hasMore, "")
	if nameOnly {
		ids := make([]ClusterIdentifier, len(sliced))
		for i, cluster := range sliced {
			ids[i] = ClusterIdentifier{Name: cluster.Name, Server: cluster.Server}
		}
		return writeCompact(page{Items: ids, TotalItems: total, HasMore: hasMore, Notice: notice})
	}
	if detailed {
		full := make([]v1alpha1.Cluster, len(sliced))
		for i, cluster := range sliced {
			full[i] = strippedCluster(cluster)
		}
		return writeCompact(page{Items: full, TotalItems: total, HasMore: hasMore, Notice: notice})
	}
	summaries := make([]ClusterSummary, len(sliced))
	for i, cluster := range sliced {
		summaries[i] = ClusterSummary{
			Name:             cluster.Name,
			Server:           cluster.Server,
			ServerVersion:    cluster.Info.ServerVersion,
			ConnectionStatus: cluster.Info.ConnectionState.Status,
		}
	}
	return writeCompact(page{Items: summaries, TotalItems: total, HasMore: hasMore, Notice: notice})
}

func strippedCluster(cluster v1alpha1.Cluster) v1alpha1.Cluster {
	sanitized := client.SanitizeCluster(&cluster)
	if sanitized.Annotations != nil {
		ann := make(map[string]string, len(sanitized.Annotations))
		for k, v := range sanitized.Annotations {
			ann[k] = v
		}
		sanitized.Annotations = ann
	}
	sanitized.Annotations = stripAnnotations(sanitized.Annotations)
	return *sanitized
}
