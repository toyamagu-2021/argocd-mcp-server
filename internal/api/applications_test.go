package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd"
)

func TestApplicationsAPI_ListApplications(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Check query parameters
		query := r.URL.Query()
		project := query.Get("projects")
		selector := query.Get("selector")

		apps := []argocd.Application{
			{
				Metadata: argocd.Metadata{
					Name:      "app1",
					Namespace: "argocd",
				},
				Spec: argocd.Spec{
					Project: project,
					Destination: argocd.Destination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
			},
		}

		if selector == "env=prod" {
			// Filter based on selector
			apps = []argocd.Application{}
		}

		response := struct {
			Items []argocd.Application `json:"items"`
		}{
			Items: apps,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Set environment variables
	os.Setenv("ARGOCD_AUTH_TOKEN", "test-token")
	os.Setenv("ARGOCD_SERVER", server.URL)
	defer os.Unsetenv("ARGOCD_AUTH_TOKEN")
	defer os.Unsetenv("ARGOCD_SERVER")

	client, _ := NewClient()
	api := NewApplicationsAPI(client)

	tests := []struct {
		name      string
		project   string
		cluster   string
		namespace string
		selector  string
		wantLen   int
	}{
		{
			name:    "list all applications",
			wantLen: 1,
		},
		{
			name:    "filter by project",
			project: "default",
			wantLen: 1,
		},
		{
			name:     "filter by selector with no matches",
			selector: "env=prod",
			wantLen:  0,
		},
		{
			name:      "filter by namespace client-side",
			namespace: "default",
			wantLen:   1,
		},
		{
			name:      "filter by namespace no match",
			namespace: "kube-system",
			wantLen:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			apps, err := api.ListApplications(ctx, tt.project, tt.cluster, tt.namespace, tt.selector)
			if err != nil {
				t.Errorf("ListApplications() error = %v", err)
			}
			if len(apps) != tt.wantLen {
				t.Errorf("ListApplications() returned %d apps, want %d", len(apps), tt.wantLen)
			}
		})
	}
}

func TestApplicationsAPI_GetApplication(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/applications/test-app" {
			app := argocd.Application{
				Metadata: argocd.Metadata{
					Name:      "test-app",
					Namespace: "argocd",
				},
				Spec: argocd.Spec{
					Project: "default",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(app)
		} else if r.URL.Path == "/api/v1/applications/not-found" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()

	// Set environment variables
	os.Setenv("ARGOCD_AUTH_TOKEN", "test-token")
	os.Setenv("ARGOCD_SERVER", server.URL)
	defer os.Unsetenv("ARGOCD_AUTH_TOKEN")
	defer os.Unsetenv("ARGOCD_SERVER")

	client, _ := NewClient()
	api := NewApplicationsAPI(client)

	tests := []struct {
		name    string
		appName string
		wantErr bool
	}{
		{
			name:    "get existing application",
			appName: "test-app",
			wantErr: false,
		},
		{
			name:    "get non-existent application",
			appName: "not-found",
			wantErr: true,
		},
		{
			name:    "empty app name",
			appName: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			app, err := api.GetApplication(ctx, tt.appName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetApplication() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("GetApplication() unexpected error = %v", err)
				}
				if app == nil {
					t.Error("GetApplication() returned nil app")
				} else if app.Metadata.Name != tt.appName {
					t.Errorf("GetApplication() app name = %v, want %v", app.Metadata.Name, tt.appName)
				}
			}
		})
	}
}

func TestApplicationsAPI_SyncApplication(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path == "/api/v1/applications/test-app/sync" {
			// Decode request body
			var syncReq SyncRequest
			if err := json.NewDecoder(r.Body).Decode(&syncReq); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// Return success
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "syncing"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Set environment variables
	os.Setenv("ARGOCD_AUTH_TOKEN", "test-token")
	os.Setenv("ARGOCD_SERVER", server.URL)
	defer os.Unsetenv("ARGOCD_AUTH_TOKEN")
	defer os.Unsetenv("ARGOCD_SERVER")

	client, _ := NewClient()
	api := NewApplicationsAPI(client)

	tests := []struct {
		name    string
		appName string
		prune   bool
		dryRun  bool
		wantErr bool
	}{
		{
			name:    "sync application",
			appName: "test-app",
			prune:   false,
			dryRun:  false,
			wantErr: false,
		},
		{
			name:    "sync with prune",
			appName: "test-app",
			prune:   true,
			dryRun:  false,
			wantErr: false,
		},
		{
			name:    "dry run sync",
			appName: "test-app",
			prune:   false,
			dryRun:  true,
			wantErr: false,
		},
		{
			name:    "empty app name",
			appName: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := api.SyncApplication(ctx, tt.appName, tt.prune, tt.dryRun)
			if tt.wantErr {
				if err == nil {
					t.Errorf("SyncApplication() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("SyncApplication() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestApplicationsAPI_DeleteApplication(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path == "/api/v1/applications/test-app" {
			// Check cascade parameter
			cascade := r.URL.Query().Get("cascade")
			if cascade == "false" {
				// Non-cascading delete
			}
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Set environment variables
	os.Setenv("ARGOCD_AUTH_TOKEN", "test-token")
	os.Setenv("ARGOCD_SERVER", server.URL)
	defer os.Unsetenv("ARGOCD_AUTH_TOKEN")
	defer os.Unsetenv("ARGOCD_SERVER")

	client, _ := NewClient()
	api := NewApplicationsAPI(client)

	tests := []struct {
		name    string
		appName string
		cascade bool
		wantErr bool
	}{
		{
			name:    "delete with cascade",
			appName: "test-app",
			cascade: true,
			wantErr: false,
		},
		{
			name:    "delete without cascade",
			appName: "test-app",
			cascade: false,
			wantErr: false,
		},
		{
			name:    "empty app name",
			appName: "",
			wantErr: true,
		},
		{
			name:    "non-existent app",
			appName: "not-found",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := api.DeleteApplication(ctx, tt.appName, tt.cascade)
			if tt.wantErr {
				if err == nil {
					t.Errorf("DeleteApplication() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("DeleteApplication() unexpected error = %v", err)
				}
			}
		})
	}
}
