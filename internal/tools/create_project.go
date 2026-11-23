package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// CreateProjectTool defines the create_project tool schema
var CreateProjectTool = mcp.NewTool("create_project",
	mcp.WithDescription("Creates a new ArgoCD project with specified configuration. Projects provide logical grouping of applications with access controls and deployment restrictions. The result is the full project as compact JSON, without managedFields or the last-applied annotation."),
	mcp.WithDestructiveHintAnnotation(true),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the project to create. Must be unique within the ArgoCD instance."),
	),
	mcp.WithString("description",
		mcp.Description("Optional description of the project's purpose."),
	),
	mcp.WithString("source_repos",
		mcp.Description("Comma-separated list of repository URLs that applications in this project can deploy from. Use '*' to allow all repositories. Default: '*'"),
	),
	mcp.WithString("destination_server",
		mcp.Description("Kubernetes API server URL. Use 'https://kubernetes.default.svc' for in-cluster. Default: 'https://kubernetes.default.svc'"),
	),
	mcp.WithString("destination_namespace",
		mcp.Description("Target namespace for deployments. Use '*' to allow all namespaces. Default: '*'"),
	),
	mcp.WithString("cluster_resource_whitelist",
		mcp.Description("Comma-separated list of cluster resources in format 'group:kind' (e.g., 'apps:Deployment,batch:Job'). Leave empty to deny all cluster resources."),
	),
	mcp.WithString("namespace_resource_whitelist",
		mcp.Description("Comma-separated list of namespace resources in format 'group:kind' (e.g., 'apps:Deployment,:Service'). Leave empty to allow all namespace resources."),
	),
	mcp.WithString("namespace_resource_blacklist",
		mcp.Description("Comma-separated list of namespace resources to deny in format 'group:kind' (e.g., ':ResourceQuota,:LimitRange')."),
	),
	mcp.WithBoolean("upsert",
		mcp.Description("If true, update the project if it already exists. If false, fail if project exists. Default: false."),
	),
)

// makeCreateProject returns the handler for this tool bound to the shared client in d.
func makeCreateProject(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Extract required parameter
		name := request.GetString("name", "")
		if name == "" {
			return mcp.NewToolResultError("Project name is required"), nil
		}

		// Extract optional parameters
		description := request.GetString("description", "")
		sourceReposStr := request.GetString("source_repos", "*")
		destServer := request.GetString("destination_server", "https://kubernetes.default.svc")
		destNamespace := request.GetString("destination_namespace", "*")
		clusterWhitelistStr := request.GetString("cluster_resource_whitelist", "")
		namespaceWhitelistStr := request.GetString("namespace_resource_whitelist", "")
		namespaceBlacklistStr := request.GetString("namespace_resource_blacklist", "")
		upsert := request.GetBool("upsert", false)

		// Build the project spec
		project := &v1alpha1.AppProject{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "argoproj.io/v1alpha1",
				Kind:       "AppProject",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: v1alpha1.AppProjectSpec{
				Description: description,
			},
		}

		// Parse source repositories
		if sourceReposStr != "" {
			project.Spec.SourceRepos = parseCommaSeparated(sourceReposStr)
		}
		if len(project.Spec.SourceRepos) == 0 {
			project.Spec.SourceRepos = []string{"*"}
		}

		// Set destination
		project.Spec.Destinations = []v1alpha1.ApplicationDestination{
			{
				Server:    destServer,
				Namespace: destNamespace,
			},
		}

		// Parse cluster resource whitelist
		if clusterWhitelistStr != "" {
			project.Spec.ClusterResourceWhitelist = parseClusterResourceRestrictions(clusterWhitelistStr)
		}

		// Parse namespace resource whitelist
		if namespaceWhitelistStr != "" {
			project.Spec.NamespaceResourceWhitelist = parseGroupKinds(namespaceWhitelistStr)
		}

		// Parse namespace resource blacklist
		if namespaceBlacklistStr != "" {
			project.Spec.NamespaceResourceBlacklist = parseGroupKinds(namespaceBlacklistStr)
		}

		argoClient := d.Client

		// Use the handler function with the real client
		return createProjectHandler(ctx, argoClient, project, upsert)
	}
}

// createProjectHandler handles the core logic for creating a project.
// This is separated out to enable testing with mocked clients.
func createProjectHandler(
	ctx context.Context,
	argoClient client.Interface,
	project *v1alpha1.AppProject,
	upsert bool,
) (*mcp.CallToolResult, error) {
	createdProject, err := argoClient.CreateProject(ctx, project, upsert)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create project: %v", err)), nil
	}

	if createdProject == nil {
		return writeCompact(nil)
	}
	stripObjectMeta(&createdProject.ObjectMeta)
	return writeCompact(createdProject)
}

// parseCommaSeparated splits a comma-separated string into a slice
func parseCommaSeparated(input string) []string {
	if input == "" {
		return []string{}
	}
	parts := strings.Split(input, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// parseClusterResourceRestrictions parses a comma-separated list of group:kind strings
// into cluster resource whitelist entries.
func parseClusterResourceRestrictions(input string) []v1alpha1.ClusterResourceRestrictionItem {
	kinds := parseGroupKinds(input)
	items := make([]v1alpha1.ClusterResourceRestrictionItem, 0, len(kinds))
	for _, kind := range kinds {
		items = append(items, v1alpha1.ClusterResourceRestrictionItem{
			Group: kind.Group,
			Kind:  kind.Kind,
		})
	}
	return items
}

// parseGroupKinds parses a comma-separated list of group:kind strings
func parseGroupKinds(input string) []metav1.GroupKind {
	if input == "" {
		return []metav1.GroupKind{}
	}
	parts := strings.Split(input, ",")
	result := make([]metav1.GroupKind, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		// Split by colon
		gk := strings.Split(trimmed, ":")
		if len(gk) == 2 {
			result = append(result, metav1.GroupKind{
				Group: strings.TrimSpace(gk[0]),
				Kind:  strings.TrimSpace(gk[1]),
			})
		} else if len(gk) == 1 {
			// Assume it's just the kind with empty group (core resources)
			result = append(result, metav1.GroupKind{
				Group: "",
				Kind:  strings.TrimSpace(gk[0]),
			})
		}
	}
	return result
}
