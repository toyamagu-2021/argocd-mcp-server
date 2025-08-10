package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/errors"
)

// ApplicationsAPI provides application-related API operations
type ApplicationsAPI struct {
	client *Client
}

// NewApplicationsAPI creates a new ApplicationsAPI instance
func NewApplicationsAPI(client *Client) *ApplicationsAPI {
	return &ApplicationsAPI{
		client: client,
	}
}

// ListApplications retrieves a list of applications
func (a *ApplicationsAPI) ListApplications(ctx context.Context, project, cluster, namespace, selector string) ([]argocd.Application, error) {
	// Build query parameters
	params := url.Values{}
	if project != "" {
		params.Add("projects", project)
	}
	if selector != "" {
		params.Add("selector", selector)
	}
	// Note: cluster and namespace filtering might need to be done client-side
	// as the API might not support these parameters directly

	path := "/applications"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := a.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	// The API returns an object with items array
	var result struct {
		Items []argocd.Application `json:"items"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, errors.NewParsingError("failed to unmarshal applications list", err, nil)
	}

	// Apply client-side filtering if needed
	filteredApps := result.Items
	if cluster != "" || namespace != "" {
		var filtered []argocd.Application
		for _, app := range result.Items {
			// Apply cluster filter
			if cluster != "" && app.Spec.Destination.Name != cluster && app.Spec.Destination.Server != cluster {
				continue
			}
			// Apply namespace filter
			if namespace != "" && app.Spec.Destination.Namespace != namespace {
				continue
			}
			filtered = append(filtered, app)
		}
		filteredApps = filtered
	}

	return filteredApps, nil
}

// GetApplication retrieves details of a specific application
func (a *ApplicationsAPI) GetApplication(ctx context.Context, appName string) (*argocd.Application, error) {
	if appName == "" {
		return nil, errors.NewValidationError("application name is required", nil)
	}

	path := fmt.Sprintf("/applications/%s", url.PathEscape(appName))
	resp, err := a.client.Get(ctx, path)
	if err != nil {
		return nil, err
	}

	var app argocd.Application
	if err := json.Unmarshal(resp, &app); err != nil {
		return nil, errors.NewParsingError("failed to unmarshal application", err, nil)
	}

	return &app, nil
}

// SyncRequest represents the request body for syncing an application
type SyncRequest struct {
	Revision  string             `json:"revision,omitempty"`
	Prune     bool               `json:"prune"`
	DryRun    bool               `json:"dryRun"`
	Strategy  *Strategy          `json:"strategy,omitempty"`
	Resources []ResourceSelector `json:"resources,omitempty"`
}

// Strategy represents sync strategy
type Strategy struct {
	Hook  *HookStrategy  `json:"hook,omitempty"`
	Apply *ApplyStrategy `json:"apply,omitempty"`
}

// HookStrategy represents hook strategy
type HookStrategy struct {
	Force bool `json:"force,omitempty"`
}

// ApplyStrategy represents apply strategy
type ApplyStrategy struct {
	Force bool `json:"force,omitempty"`
}

// ResourceSelector represents a resource selector for sync
type ResourceSelector struct {
	Group     string `json:"group,omitempty"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// SyncApplication triggers a sync operation for an application
func (a *ApplicationsAPI) SyncApplication(ctx context.Context, appName string, prune bool, dryRun bool) error {
	if appName == "" {
		return errors.NewValidationError("application name is required", nil)
	}

	syncReq := SyncRequest{
		Prune:  prune,
		DryRun: dryRun,
	}

	path := fmt.Sprintf("/applications/%s/sync", url.PathEscape(appName))
	_, err := a.client.Post(ctx, path, syncReq)
	if err != nil {
		return err
	}

	return nil
}

// DeleteApplication deletes an application
func (a *ApplicationsAPI) DeleteApplication(ctx context.Context, appName string, cascade bool) error {
	if appName == "" {
		return errors.NewValidationError("application name is required", nil)
	}

	// Build query parameters
	params := url.Values{}
	if !cascade {
		params.Add("cascade", "false")
	}

	path := fmt.Sprintf("/applications/%s", url.PathEscape(appName))
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	_, err := a.client.Delete(ctx, path)
	if err != nil {
		return err
	}

	return nil
}
