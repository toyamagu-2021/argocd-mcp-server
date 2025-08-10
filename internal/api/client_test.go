package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		server      string
		wantErr     bool
		errContains string
	}{
		{
			name:    "valid credentials",
			token:   "test-token",
			server:  "argocd.example.com",
			wantErr: false,
		},
		{
			name:        "missing token",
			token:       "",
			server:      "argocd.example.com",
			wantErr:     true,
			errContains: "ARGOCD_AUTH_TOKEN and ARGOCD_SERVER",
		},
		{
			name:        "missing server",
			token:       "test-token",
			server:      "",
			wantErr:     true,
			errContains: "ARGOCD_AUTH_TOKEN and ARGOCD_SERVER",
		},
		{
			name:    "server with https prefix",
			token:   "test-token",
			server:  "https://argocd.example.com",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv("ARGOCD_AUTH_TOKEN", tt.token)
			os.Setenv("ARGOCD_SERVER", tt.server)
			defer os.Unsetenv("ARGOCD_AUTH_TOKEN")
			defer os.Unsetenv("ARGOCD_SERVER")

			client, err := NewClient()
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewClient() error = nil, wantErr %v", tt.wantErr)
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("NewClient() error = %v, want error containing %v", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("NewClient() unexpected error = %v", err)
				}
				if client == nil {
					t.Error("NewClient() returned nil client")
				}
			}
		})
	}
}

func TestClient_Get(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}

		// Check path
		if r.URL.Path == "/api/v1/applications" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"items": []}`))
		} else if r.URL.Path == "/api/v1/notfound" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "not found"}`))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "server error"}`))
		}
	}))
	defer server.Close()

	// Set environment variables
	os.Setenv("ARGOCD_AUTH_TOKEN", "test-token")
	os.Setenv("ARGOCD_SERVER", server.URL)
	defer os.Unsetenv("ARGOCD_AUTH_TOKEN")
	defer os.Unsetenv("ARGOCD_SERVER")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		wantLen int
	}{
		{
			name:    "successful get",
			path:    "/applications",
			wantErr: false,
			wantLen: 13, // {"items": []}
		},
		{
			name:    "not found",
			path:    "/notfound",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			resp, err := client.Get(ctx, tt.path)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Get() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("Get() unexpected error = %v", err)
				}
				if tt.wantLen > 0 && len(resp) != tt.wantLen {
					t.Errorf("Get() response length = %d, want %d", len(resp), tt.wantLen)
				}
			}
		})
	}
}

func TestClient_Post(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check method
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "unauthorized"}`))
			return
		}

		// Check content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "synced"}`))
	}))
	defer server.Close()

	// Set environment variables
	os.Setenv("ARGOCD_AUTH_TOKEN", "test-token")
	os.Setenv("ARGOCD_SERVER", server.URL)
	defer os.Unsetenv("ARGOCD_AUTH_TOKEN")
	defer os.Unsetenv("ARGOCD_SERVER")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	body := map[string]interface{}{
		"prune":  true,
		"dryRun": false,
	}

	resp, err := client.Post(ctx, "/applications/test-app/sync", body)
	if err != nil {
		t.Errorf("Post() unexpected error = %v", err)
	}

	if len(resp) == 0 {
		t.Error("Post() returned empty response")
	}
}

func TestClient_Delete(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check method
		if r.Method != http.MethodDelete {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Check authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Set environment variables
	os.Setenv("ARGOCD_AUTH_TOKEN", "test-token")
	os.Setenv("ARGOCD_SERVER", server.URL)
	defer os.Unsetenv("ARGOCD_AUTH_TOKEN")
	defer os.Unsetenv("ARGOCD_SERVER")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	ctx := context.Background()
	_, err = client.Delete(ctx, "/applications/test-app")
	if err != nil {
		t.Errorf("Delete() unexpected error = %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) >= len(substr) && contains(s[1:], substr)
}
