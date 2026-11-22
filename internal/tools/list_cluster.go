package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// ListClusterTool provides MCP tool for listing all ArgoCD clusters
var ListClusterTool = mcp.NewTool("list_cluster",
	mcp.WithDescription("Lists all ArgoCD clusters configured in the system. Use name_only=true to get just cluster names and servers for a compact view."),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, returns complete cluster details including all configuration data (can be very large). If false (default), returns only essential fields. Recommended: keep this as false to avoid fetching excessive data."),
	),
	mcp.WithBoolean("name_only",
		mcp.Description("If true, returns only cluster names/servers. Takes precedence over 'detailed' option. Useful for getting a quick list of cluster identifiers."),
	),
)

// ClusterSummary represents a simplified view of a cluster
type ClusterSummary struct {
	Name             string                    `json:"name"`
	Server           string                    `json:"server"`
	ServerVersion    string                    `json:"serverVersion,omitempty"`
	ConnectionStatus v1alpha1.ConnectionStatus `json:"connectionStatus"`
}

// HandleListCluster handles MCP tool requests for listing ArgoCD clusters
// makeListCluster returns the handler for this tool bound to the shared client in d.
func makeListCluster(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract the detailed and name_only parameters
		detailed := request.GetBool("detailed", false)
		nameOnly := request.GetBool("name_only", false)

		argoClient := d.Client

		return listClusterHandler(ctx, argoClient, detailed, nameOnly)
	}
}

// ClusterNameList represents a list of cluster identifiers
type ClusterNameList struct {
	Clusters []ClusterIdentifier `json:"clusters"`
	Count    int                 `json:"count"`
}

// ClusterIdentifier contains minimal cluster identification
type ClusterIdentifier struct {
	Name   string `json:"name"`
	Server string `json:"server"`
}

func listClusterHandler(
	ctx context.Context,
	argoClient client.Interface,
	detailed bool,
	nameOnly bool,
) (*mcp.CallToolResult, error) {
	clusters, err := argoClient.ListClusters(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list clusters: %v", err)), nil
	}

	if len(clusters.Items) == 0 {
		return mcp.NewToolResultText("No clusters found."), nil
	}

	var jsonData []byte

	if nameOnly {
		// Return only cluster names and servers
		identifiers := make([]ClusterIdentifier, 0, len(clusters.Items))
		for _, cluster := range clusters.Items {
			identifiers = append(identifiers, ClusterIdentifier{
				Name:   cluster.Name,
				Server: cluster.Server,
			})
		}
		nameList := ClusterNameList{
			Clusters: identifiers,
			Count:    len(identifiers),
		}
		jsonData, err = json.MarshalIndent(nameList, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
	} else if detailed {
		// Return full cluster details
		jsonData, err = json.MarshalIndent(clusters, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
		}
	} else {
		// Return summarized cluster information
		summaries := make([]ClusterSummary, 0, len(clusters.Items))
		for _, cluster := range clusters.Items {
			summary := ClusterSummary{
				Name:             cluster.Name,
				Server:           cluster.Server,
				ConnectionStatus: cluster.Info.ConnectionState.Status,
			}

			// Add server version if available
			if cluster.Info.ServerVersion != "" {
				summary.ServerVersion = cluster.Info.ServerVersion
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
