package tools

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// GetAppManifestsTool defines the get_application_manifests tool schema.
var GetAppManifestsTool = mcp.NewTool("get_application_manifests",
	mcp.WithDescription("Retrieves rendered Kubernetes manifests for an ArgoCD application. detailed=false (the default) returns a page of manifest strings with managedFields and the last-applied annotation removed. limit defaults to 10 and offset to 0. commands and integrity fields are included only when detailed=true. revision is the git revision, not a page control. Secret data is already hidden by the Argo CD server."),
	mcp.WithReadOnlyHintAnnotation(true),
	mcp.WithDestructiveHintAnnotation(false),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to retrieve manifests for."),
	),
	mcp.WithString("revision",
		mcp.Description("The git revision to retrieve manifests for. If not specified, uses the currently deployed revision. This is not a page control."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
	mcp.WithBoolean("detailed",
		mcp.Description("If true, the page also includes commands and integrity fields. If false (default), those fields are omitted. Paging still applies."),
	),
	mcp.WithNumber("limit",
		mcp.Description("Maximum number of manifests to return (default: 10). Use with offset to page."),
	),
	mcp.WithNumber("offset",
		mcp.Description("Number of manifests to skip (default: 0)."),
	),
)

// manifestView is one page of stripped manifest strings plus the sibling
// fields an agent needs to know where the page came from.
type manifestView struct {
	page
	Namespace             string                               `json:"namespace,omitempty"`
	Server                string                               `json:"server,omitempty"`
	Revision              string                               `json:"revision,omitempty"`
	SourceType            string                               `json:"sourceType,omitempty"`
	Commands              []string                             `json:"commands,omitempty"`
	VerifyResult          string                               `json:"verifyResult,omitempty"`
	SourceIntegrityResult *v1alpha1.SourceIntegrityCheckResult `json:"sourceIntegrityResult,omitempty"`
}

// makeGetApplicationManifests returns the handler for this tool bound to the shared client in d.
func makeGetApplicationManifests(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit, offset, err := pageArgs(request, defaultManifestLimit)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return getApplicationManifestsHandler(
			ctx,
			d.Client,
			request.GetString("name", ""),
			request.GetString("revision", ""),
			request.GetString("app_namespace", ""),
			request.GetBool("detailed", false),
			limit,
			offset,
		)
	}
}

// getApplicationManifestsHandler handles the core logic for getting application manifests.
// This is separated out to enable testing with mocked clients.
func getApplicationManifestsHandler(
	ctx context.Context,
	argoClient client.Interface,
	appName string,
	revision string,
	appNamespace string,
	detailed bool,
	limit, offset int,
) (*mcp.CallToolResult, error) {
	if appName == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}

	manifests, err := argoClient.GetApplicationManifests(ctx, appName, revision, appNamespace)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get application manifests: %v", err)), nil
	}
	if manifests == nil {
		return writeCompact(nil)
	}
	sliced, total, hasMore := pageSlice(manifests.Manifests, limit, offset)
	items := make([]string, len(sliced))
	for i, raw := range sliced {
		items[i] = stripManifestNoise(raw)
	}
	view := manifestView{
		page: page{
			Items:      items,
			TotalItems: total,
			HasMore:    hasMore,
			Notice:     pageNotice(offset, len(items), total, hasMore, ""),
		},
		Namespace:  manifests.Namespace,
		Server:     manifests.Server,
		Revision:   manifests.Revision,
		SourceType: manifests.SourceType,
	}
	if detailed {
		view.Commands = manifests.Commands
		//lint:ignore SA1019 deprecated field still populated by the Argo CD server for backward compatibility
		view.VerifyResult = manifests.VerifyResult
		view.SourceIntegrityResult = manifests.SourceIntegrityResult
	}
	return writeCompact(view)
}
