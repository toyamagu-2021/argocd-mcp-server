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

// UpdateApplicationTool replaces an Application spec.
var UpdateApplicationTool = mcp.NewTool("update_application",
	mcp.WithDescription("Replaces the spec of an existing ArgoCD application (ApplicationService.UpdateSpec). The server loads the Application and changes only spec, so metadata and status are not taken from this call. The spec object replaces the whole spec: fields you omit are cleared. Read the Application with get_application and send the full spec you want to keep. spec.project is required, and spec.source.repoURL or a spec.sources entry with repoURL is required. validate defaults to true."),
	mcp.WithDestructiveHintAnnotation(true),
	mcp.WithIdempotentHintAnnotation(true),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("Name of the Application to update."),
	),
	mcp.WithObject("spec",
		mcp.Required(),
		mcp.Description("Full Application spec. Must include project and either source or sources. This replaces spec entirely."),
	),
	mcp.WithBoolean("validate",
		mcp.Description("Ask Argo CD to validate the spec before saving (default: true). false skips that validation."),
	),
	mcp.WithString("app_namespace",
		mcp.Description("Namespace of the Application resource (its metadata.namespace). Only needed when Argo CD manages Applications outside its own namespace (apps-in-any-namespace). Defaults to the Argo CD namespace."),
	),
	mcp.WithString("project",
		mcp.Description("Project used for the update RBAC check. Omit to let Argo CD use the Application's current project."),
	),
)

// makeUpdateApplication returns the handler bound to the shared client in d.
func makeUpdateApplication(d Deps) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		spec, err := applicationSpecArg(request)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if err := boolArg(request, "validate"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return updateApplicationHandler(ctx, d.Client, client.UpdateSpecOptions{
			Name:         request.GetString("name", ""),
			AppNamespace: request.GetString("app_namespace", ""),
			Project:      request.GetString("project", ""),
			Spec:         spec,
			Validate:     request.GetBool("validate", true),
		})
	}
}

func updateApplicationHandler(ctx context.Context, argoClient client.Interface, opts client.UpdateSpecOptions) (*mcp.CallToolResult, error) {
	if opts.Name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	if opts.Spec == nil {
		return mcp.NewToolResultError("spec is required"), nil
	}
	if opts.Spec.Project == "" {
		return mcp.NewToolResultError("spec.project is required"), nil
	}
	if !specHasRepo(opts.Spec) {
		return mcp.NewToolResultError("spec.source.repoURL or spec.sources[].repoURL is required"), nil
	}
	spec, err := argoClient.UpdateApplicationSpec(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update application: %v", err)), nil
	}
	jsonData, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}

func applicationSpecArg(request mcp.CallToolRequest) (*v1alpha1.ApplicationSpec, error) {
	val, ok := request.GetArguments()["spec"]
	if !ok || val == nil {
		return nil, fmt.Errorf("spec is required")
	}
	raw, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("spec must be an object: %w", err)
	}
	if string(raw) == "null" || len(raw) == 0 || raw[0] != '{' {
		return nil, fmt.Errorf("spec must be an object")
	}
	var spec v1alpha1.ApplicationSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("spec must be an Application spec: %w", err)
	}
	return &spec, nil
}

func specHasRepo(spec *v1alpha1.ApplicationSpec) bool {
	if spec.Source != nil && spec.Source.RepoURL != "" {
		return true
	}
	for i := range spec.Sources {
		if spec.Sources[i].RepoURL != "" {
			return true
		}
	}
	return false
}

func boolArg(request mcp.CallToolRequest, key string) error {
	val, ok := request.GetArguments()[key]
	if !ok || val == nil {
		return nil
	}
	if _, ok := val.(bool); !ok {
		return fmt.Errorf("%s must be a boolean", key)
	}
	return nil
}
