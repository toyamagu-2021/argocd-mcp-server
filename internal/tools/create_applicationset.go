package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var CreateApplicationSetTool = mcp.NewTool("create_applicationset",
	mcp.WithDescription("Create a new ApplicationSet in ArgoCD"),
	mcp.WithString("name",
		mcp.Required(),
		mcp.Description("Name of the ApplicationSet"),
	),
	mcp.WithString("namespace",
		mcp.Description("Namespace for the ApplicationSet (default: argocd)"),
	),
	mcp.WithString("project",
		mcp.Description("ArgoCD project name (default: default)"),
	),
	mcp.WithString("generators",
		mcp.Required(),
		mcp.Description("List of generators for the ApplicationSet (JSON string format)"),
	),
	mcp.WithString("template",
		mcp.Required(),
		mcp.Description("Application template (JSON string format)"),
	),
	mcp.WithString("sync_policy",
		mcp.Description("Sync policy configuration (JSON string format)"),
	),
	mcp.WithString("strategy",
		mcp.Description("Deployment strategy configuration (JSON string format)"),
	),
	mcp.WithBoolean("go_template",
		mcp.Description("Enable Go templating (default: false)"),
	),
	mcp.WithBoolean("upsert",
		mcp.Description("Create or update if exists (default: false)"),
	),
	mcp.WithBoolean("dry_run",
		mcp.Description("Perform a dry run without creating the ApplicationSet (default: false)"),
	),
)

func HandleCreateApplicationSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.GetString("name", "")
	if name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}

	namespace := request.GetString("namespace", "argocd")
	project := request.GetString("project", "default")

	generatorsStr := request.GetString("generators", "")
	if generatorsStr == "" {
		return mcp.NewToolResultError("generators is required"), nil
	}

	templateStr := request.GetString("template", "")
	if templateStr == "" {
		return mcp.NewToolResultError("template is required"), nil
	}

	syncPolicyStr := request.GetString("sync_policy", "")
	strategyStr := request.GetString("strategy", "")
	goTemplate := request.GetBool("go_template", false)
	upsert := request.GetBool("upsert", false)
	dryRun := request.GetBool("dry_run", false)

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

	return createApplicationSetHandler(ctx, argoClient, name, namespace, project, generatorsStr, templateStr, syncPolicyStr, strategyStr, goTemplate, upsert, dryRun)
}

func createApplicationSetHandler(
	ctx context.Context,
	argoClient client.Interface,
	name string,
	namespace string,
	project string,
	generatorsStr string,
	templateStr string,
	syncPolicyStr string,
	strategyStr string,
	goTemplate bool,
	upsert bool,
	dryRun bool,
) (*mcp.CallToolResult, error) {
	// Parse generators JSON string
	var appSetGenerators []v1alpha1.ApplicationSetGenerator
	if err := json.Unmarshal([]byte(generatorsStr), &appSetGenerators); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse generators: %v", err)), nil
	}

	// Parse template JSON string
	var appSetTemplate v1alpha1.ApplicationSetTemplate
	if err := json.Unmarshal([]byte(templateStr), &appSetTemplate); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to parse template: %v", err)), nil
	}

	// Create ApplicationSet spec
	appSetSpec := v1alpha1.ApplicationSetSpec{
		GoTemplate: goTemplate,
		Generators: appSetGenerators,
		Template:   appSetTemplate,
	}

	// Add optional sync policy
	if syncPolicyStr != "" {
		var appSetSyncPolicy v1alpha1.ApplicationSetSyncPolicy
		if err := json.Unmarshal([]byte(syncPolicyStr), &appSetSyncPolicy); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse sync_policy: %v", err)), nil
		}
		appSetSpec.SyncPolicy = &appSetSyncPolicy
	}

	// Add optional strategy
	if strategyStr != "" {
		var appSetStrategy v1alpha1.ApplicationSetStrategy
		if err := json.Unmarshal([]byte(strategyStr), &appSetStrategy); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to parse strategy: %v", err)), nil
		}
		appSetSpec.Strategy = &appSetStrategy
	}

	// Create ApplicationSet object
	appSet := &v1alpha1.ApplicationSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "ApplicationSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appSetSpec,
	}

	// Set project label if specified
	if project != "" && project != "default" {
		appSet.ObjectMeta.Labels = map[string]string{
			"argocd.argoproj.io/project": project,
		}
	}

	// Create the ApplicationSet
	result, err := argoClient.CreateApplicationSet(ctx, appSet, upsert, dryRun)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create ApplicationSet: %v", err)), nil
	}

	// Format response
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonData)), nil
}
