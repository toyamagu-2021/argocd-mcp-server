package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
)

// SyncAppTool defines the sync_application tool schema
var SyncAppTool = mcp.NewTool("sync_application",
	mcp.WithDescription("Triggers a sync operation for a specific ArgoCD application. Optional revision syncs a single-source app to a git revision, tag, or Helm chart version. Optional revisions do the same for a multi-source app, one entry per source in spec.sources order. Optional resources limits the sync to those resources. Optional sync_options replace the Application's sync options for this operation."),
	mcp.WithDestructiveHintAnnotation(true),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("The name of the application to sync."),
	),
	mcp.WithBoolean("prune",
		mcp.Description("Whether to delete resources that are no longer defined in the source (default: false)."),
	),
	mcp.WithBoolean("dry_run",
		mcp.Description("Preview the sync operation without making actual changes (default: false)."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
	mcp.WithString("revision",
		mcp.Description("Git revision, tag, or Helm chart version to sync a single-source Application to. Omit to use spec.source.targetRevision. For a multi-source Application use revisions. Syncing to a revision other than the spec target fails when auto-sync is enabled, unless dry_run is true."),
	),
	mcp.WithArray("revisions",
		mcp.Description("Per-source revisions for a multi-source Application, in spec.sources order (position 1 is the first source). An empty entry keeps that source's targetRevision. Omit to use each source's targetRevision. Single-source Applications should use revision; Argo CD ignores this list for them."),
		mcp.WithStringItems(),
	),
	mcp.WithArray("resources",
		mcp.Description("Limit the sync to these resources (partial sync). Omit to sync every resource. Each item requires kind and name. group is the API group and is empty for core resources such as ConfigMap. namespace is the resource namespace."),
		mcp.Items(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"group":     map[string]any{"type": "string", "description": "API group. Empty for core resources."},
				"kind":      map[string]any{"type": "string", "description": "Resource kind, for example Deployment."},
				"name":      map[string]any{"type": "string", "description": "Resource name."},
				"namespace": map[string]any{"type": "string", "description": "Resource namespace. Empty matches the name in any namespace."},
			},
			"required": []string{"kind", "name"},
		}),
	),
	mcp.WithArray("sync_options",
		mcp.Description("Per-sync Argo CD sync options, the same strings as `argocd app sync --sync-option` (for example CreateNamespace=true, PruneLast=true, ServerSideApply=true, Replace=true, ApplyOutOfSyncOnly=true). When set, they replace spec.syncPolicy.syncOptions for this operation. They are not merged. Omit to use the Application's sync options."),
		mcp.WithStringItems(),
	),
)

// makeSyncApplication returns the handler for this tool bound to the shared client in d.
func makeSyncApplication(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		revisions, err := stringSliceArg(request, "revisions")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		options, err := stringSliceArg(request, "sync_options")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		resources, err := syncResourcesArg(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return syncApplicationHandler(ctx, d.Client, client.SyncOptions{
			Name:         request.GetString("name", ""),
			Revision:     request.GetString("revision", ""),
			Revisions:    revisions,
			Prune:        request.GetBool("prune", false),
			DryRun:       request.GetBool("dry_run", false),
			AppNamespace: request.GetString("app_namespace", ""),
			Resources:    resources,
			Options:      options,
		})
	}
}

// syncApplicationHandler handles the core logic for syncing an application.
// This is separated out to enable testing with mocked clients.
func syncApplicationHandler(
	ctx context.Context,
	argoClient client.Interface,
	opts client.SyncOptions,
) (*mcp.CallToolResult, error) {
	if opts.Name == "" {
		return mcp.NewToolResultError("Application name is required"), nil
	}
	for i, r := range opts.Resources {
		if r.Kind == "" || r.Name == "" {
			return mcp.NewToolResultError(fmt.Sprintf("resources[%d] requires kind and name", i)), nil
		}
	}

	app, err := argoClient.SyncApplication(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to sync application: %v", err)), nil
	}

	jsonData, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}

func stringSliceArg(request mcp.CallToolRequest, key string) ([]string, error) {
	val, ok := request.GetArguments()[key]
	if !ok || val == nil {
		return nil, nil
	}
	var items []any
	switch v := val.(type) {
	case []string:
		items = make([]any, len(v))
		for i, s := range v {
			items[i] = s
		}
	case []any:
		items = v
	default:
		return nil, fmt.Errorf("%s must be an array of strings", key)
	}
	out := make([]string, len(items))
	for i, item := range items {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be a string", key, i)
		}
		if key == "sync_options" && s == "" {
			return nil, fmt.Errorf("%s[%d] must not be empty", key, i)
		}
		out[i] = s
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func syncResourcesArg(request mcp.CallToolRequest) ([]client.SyncResource, error) {
	val, ok := request.GetArguments()["resources"]
	if !ok || val == nil {
		return nil, nil
	}
	items, ok := val.([]any)
	if !ok {
		return nil, fmt.Errorf("resources must be an array")
	}
	out := make([]client.SyncResource, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("resources[%d] must be an object", i)
		}
		res, err := syncResourceFromMap(m)
		if err != nil {
			return nil, fmt.Errorf("resources[%d]: %w", i, err)
		}
		out = append(out, res)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func syncResourceFromMap(m map[string]any) (client.SyncResource, error) {
	group, err := mapString(m, "group")
	if err != nil {
		return client.SyncResource{}, err
	}
	kind, err := mapString(m, "kind")
	if err != nil {
		return client.SyncResource{}, err
	}
	name, err := mapString(m, "name")
	if err != nil {
		return client.SyncResource{}, err
	}
	namespace, err := mapString(m, "namespace")
	if err != nil {
		return client.SyncResource{}, err
	}
	return client.SyncResource{Group: group, Kind: kind, Name: name, Namespace: namespace}, nil
}

func mapString(m map[string]any, key string) (string, error) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", key)
	}
	return s, nil
}
