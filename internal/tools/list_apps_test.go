package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHandleListApplications(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		envVars       map[string]string
		wantError     bool
		errorContains string
	}{
		{
			name: "missing environment variables",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "list_application",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "",
				"ARGOCD_SERVER":     "",
			},
			wantError:     true,
			errorContains: "server address is required",
		},
		{
			name: "with filter parameters",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "list_application",
					Arguments: map[string]interface{}{
						"project":   "default",
						"cluster":   "in-cluster",
						"namespace": "argocd",
						"selector":  "app=test",
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "failed to list applications",
		},
		{
			name: "empty filter parameters",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "list_application",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "test-server.com",
			},
			wantError:     true, // Will fail because argocd CLI is not actually called
			errorContains: "failed to list applications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleListApplications(context.Background(), tt.request)

			// Check error expectation
			if tt.wantError {
				if err != nil {
					t.Errorf("HandleListApplications() returned error = %v, but error was not expected in result", err)
				}
				if result == nil {
					t.Fatal("HandleListApplications() returned nil result")
				}
				// Check if error is in the result
				if !result.IsError {
					t.Errorf("HandleListApplications() expected error result, but got success")
				}
				// Check error content is present (we know it's an error result)
				if tt.errorContains != "" && len(result.Content) == 0 {
					t.Errorf("HandleListApplications() expected error content, but got empty")
				}
			} else {
				if err != nil {
					t.Errorf("HandleListApplications() error = %v, wantError %v", err, tt.wantError)
				}
			}
		})
	}
}

func TestListAppsTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if ListAppsTool.Name != "list_application" {
		t.Errorf("Expected tool name 'list_application', got %s", ListAppsTool.Name)
	}

	// Verify tool has description
	if ListAppsTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if ListAppsTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", ListAppsTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(ListAppsTool.InputSchema.Properties) == 0 {
		t.Error("Tool schema should have properties defined")
	}
}

func TestListApplicationsHandler(t *testing.T) {
	tests := []struct {
		name        string
		project     string
		cluster     string
		namespace   string
		selector    string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
		wantApps    int
	}{
		{
			name:      "successful list with no filters",
			project:   "",
			cluster:   "",
			namespace: "",
			selector:  "",
			setupMock: func(m *mock.MockInterface) {
				appList := &v1alpha1.ApplicationList{
					Items: []v1alpha1.Application{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "app1",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "default",
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://kubernetes.default.svc",
									Namespace: "default",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "app2",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "myproject",
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://kubernetes.default.svc",
									Namespace: "kube-system",
								},
							},
						},
					},
				}
				m.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)
			},
			wantError: false,
			wantApps:  2,
		},
		{
			name:      "filter by project",
			project:   "myproject",
			cluster:   "",
			namespace: "",
			selector:  "",
			setupMock: func(m *mock.MockInterface) {
				appList := &v1alpha1.ApplicationList{
					Items: []v1alpha1.Application{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "app1",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "default",
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://kubernetes.default.svc",
									Namespace: "default",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "app2",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "myproject",
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://kubernetes.default.svc",
									Namespace: "kube-system",
								},
							},
						},
					},
				}
				m.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)
			},
			wantError: false,
			wantApps:  1,
		},
		{
			name:      "filter by namespace",
			project:   "",
			cluster:   "",
			namespace: "kube-system",
			selector:  "",
			setupMock: func(m *mock.MockInterface) {
				appList := &v1alpha1.ApplicationList{
					Items: []v1alpha1.Application{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "app1",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "default",
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://kubernetes.default.svc",
									Namespace: "default",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "app2",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "myproject",
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://kubernetes.default.svc",
									Namespace: "kube-system",
								},
							},
						},
					},
				}
				m.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)
			},
			wantError: false,
			wantApps:  1,
		},
		{
			name:      "no applications found",
			project:   "nonexistent",
			cluster:   "",
			namespace: "",
			selector:  "",
			setupMock: func(m *mock.MockInterface) {
				appList := &v1alpha1.ApplicationList{
					Items: []v1alpha1.Application{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "app1",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "default",
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://kubernetes.default.svc",
									Namespace: "default",
								},
							},
						},
					},
				}
				m.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)
			},
			wantError:   false,
			wantMessage: "No applications found matching the criteria.",
		},
		{
			name:      "with label selector",
			project:   "",
			cluster:   "",
			namespace: "",
			selector:  "env=production",
			setupMock: func(m *mock.MockInterface) {
				appList := &v1alpha1.ApplicationList{
					Items: []v1alpha1.Application{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "prod-app",
								Namespace: "argocd",
								Labels: map[string]string{
									"env": "production",
								},
							},
							Spec: v1alpha1.ApplicationSpec{
								Project: "production",
								Destination: v1alpha1.ApplicationDestination{
									Server:    "https://prod-cluster.example.com",
									Namespace: "default",
								},
							},
						},
					},
				}
				m.EXPECT().ListApplications(gomock.Any(), "env=production").Return(appList, nil)
			},
			wantError: false,
			wantApps:  1,
		},
		{
			name:      "API error",
			project:   "",
			cluster:   "",
			namespace: "",
			selector:  "",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().ListApplications(gomock.Any(), "").Return(nil, assert.AnError)
			},
			wantError:   false, // The handler returns an error result, not an error
			wantMessage: "Failed to list applications",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			ctx := context.Background()
			result, err := listApplicationsHandler(ctx, mockClient, tt.project, tt.cluster, tt.namespace, tt.selector, false, false, "json", []string{})

			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check the result content
			if tt.wantMessage != "" {
				// Expecting a specific message
				require.Len(t, result.Content, 1)
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)
				assert.Contains(t, textContent.Text, tt.wantMessage)
			} else if tt.wantApps > 0 {
				// Expecting JSON with applications
				require.Len(t, result.Content, 1)
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)

				var apps []ApplicationSummary
				err := json.Unmarshal([]byte(textContent.Text), &apps)
				require.NoError(t, err)
				assert.Len(t, apps, tt.wantApps)
			}
		})
	}
}

func TestListApplicationsHandler_DetailedOutput(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	// Setup mock with applications having status information
	appList := &v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/example/repo",
						Path:           "manifests",
						TargetRevision: "main",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
				Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeSynced,
					},
					Health: v1alpha1.HealthStatus{
						Status: "Healthy",
					},
				},
			},
		},
	}

	ctx := context.Background()

	t.Run("summary output (default)", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(ctx, mockClient, "", "", "", "", false, false, "json", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Len(t, result.Content, 1)
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var summaries []ApplicationSummary
		err = json.Unmarshal([]byte(textContent.Text), &summaries)
		require.NoError(t, err)
		require.Len(t, summaries, 1)

		summary := summaries[0]
		assert.Equal(t, "test-app", summary.Name)
		assert.Equal(t, "argocd", summary.Namespace)
		assert.Equal(t, "default", summary.Project)
		assert.Equal(t, "https://github.com/example/repo", summary.Source.RepoURL)
		assert.Equal(t, "manifests", summary.Source.Path)
		assert.Equal(t, "main", summary.Source.TargetRevision)
		assert.Equal(t, "https://kubernetes.default.svc", summary.Destination.Server)
		assert.Equal(t, "default", summary.Destination.Namespace)
		assert.Equal(t, "Synced", summary.SyncStatus)
		assert.Equal(t, "Healthy", summary.HealthStatus)
	})

	t.Run("detailed output", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(ctx, mockClient, "", "", "", "", true, false, "json", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Len(t, result.Content, 1)
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var apps []v1alpha1.Application
		err = json.Unmarshal([]byte(textContent.Text), &apps)
		require.NoError(t, err)
		require.Len(t, apps, 1)

		app := apps[0]
		assert.Equal(t, "test-app", app.Name)
		assert.Equal(t, "argocd", app.Namespace)
		assert.Equal(t, "default", app.Spec.Project)
		// Verify that full application structure is preserved
		assert.NotNil(t, app.Status)
		assert.Equal(t, v1alpha1.SyncStatusCodeSynced, app.Status.Sync.Status)
		assert.Equal(t, "Healthy", string(app.Status.Health.Status))
	})

	t.Run("name only output", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(ctx, mockClient, "", "", "", "", false, true, "json", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Len(t, result.Content, 1)
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var nameList ApplicationNameList
		err = json.Unmarshal([]byte(textContent.Text), &nameList)
		require.NoError(t, err)
		assert.Equal(t, 1, nameList.Count)
		assert.Equal(t, []string{"test-app"}, nameList.Names)
	})
}

func TestListApplicationsHandler_ComplexFiltering(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	// Setup mock with multiple applications
	appList := &v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app1",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "project1",
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://cluster1.example.com",
						Namespace: "ns1",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app2",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "project1",
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://cluster2.example.com",
						Namespace: "ns1",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app3",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "project2",
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://cluster1.example.com",
						Namespace: "ns2",
					},
				},
			},
		},
	}

	mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

	ctx := context.Background()

	// Filter by project and cluster
	result, err := listApplicationsHandler(ctx, mockClient, "project1", "https://cluster1.example.com", "", "", false, false, "json", []string{})
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var apps []ApplicationSummary
	err = json.Unmarshal([]byte(textContent.Text), &apps)
	require.NoError(t, err)
	assert.Len(t, apps, 1)
	assert.Equal(t, "app1", apps[0].Name)
}

func TestListApplicationsHandler_TSVFormat(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	// Setup mock with multiple applications with various data patterns
	appList := &v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app1",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/example/repo.git",
						Path:           "manifests",
						TargetRevision: "main",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
				Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeSynced,
					},
					Health: v1alpha1.HealthStatus{
						Status: "Healthy",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app2",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "monitoring",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://charts.helm.sh/stable",
						Chart:          "prometheus",
						TargetRevision: "15.18.0",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://prod-cluster.company.com:6443",
						Namespace: "monitoring",
					},
				},
				Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeOutOfSync,
					},
					Health: v1alpha1.HealthStatus{
						Status: "Degraded",
					},
					OperationState: &v1alpha1.OperationState{
						Phase:   "Running",
						Message: "Syncing application resources",
						StartedAt: metav1.Time{
							Time: time.Date(2023, 12, 1, 10, 0, 0, 0, time.UTC),
						},
					},
				},
			},
		},
	}

	ctx := context.Background()

	t.Run("TSV minimal output (default - 4 fields)", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "tsv", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Len(t, result.Content, 1)
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		lines := strings.Split(strings.TrimSpace(textContent.Text), "\n")
		require.Len(t, lines, 3) // Header + 2 data rows

		// Check header (only 4 minimal fields)
		expectedHeader := "name\tproject\tsyncStatus\thealthStatus"
		assert.Equal(t, expectedHeader, lines[0])

		// Check first app data (4 fields)
		app1Fields := strings.Split(lines[1], "\t")
		require.Len(t, app1Fields, 4)
		assert.Equal(t, "app1", app1Fields[0])    // name
		assert.Equal(t, "default", app1Fields[1]) // project
		assert.Equal(t, "Synced", app1Fields[2])  // syncStatus
		assert.Equal(t, "Healthy", app1Fields[3]) // healthStatus

		// Check second app data (4 fields)
		app2Fields := strings.Split(lines[2], "\t")
		require.Len(t, app2Fields, 4)
		assert.Equal(t, "app2", app2Fields[0])       // name
		assert.Equal(t, "monitoring", app2Fields[1]) // project
		assert.Equal(t, "OutOfSync", app2Fields[2])  // syncStatus
		assert.Equal(t, "Degraded", app2Fields[3])   // healthStatus
	})

	t.Run("TSV name only output", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(ctx, mockClient, "", "", "", "", false, true, "tsv", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Len(t, result.Content, 1)
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		lines := strings.Split(strings.TrimSpace(textContent.Text), "\n")
		require.Len(t, lines, 2) // No header, just names

		assert.Equal(t, "app1", lines[0])
		assert.Equal(t, "app2", lines[1])
	})

	t.Run("TSV detailed output (falls back to JSON)", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(ctx, mockClient, "", "", "", "", true, false, "tsv", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		require.Len(t, result.Content, 1)
		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		// Should be JSON format for detailed output
		var apps []v1alpha1.Application
		err = json.Unmarshal([]byte(textContent.Text), &apps)
		require.NoError(t, err)
		assert.Len(t, apps, 2)
	})
}

func TestListApplicationsHandler_TSVFormat_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	t.Run("TSV format with special characters", func(t *testing.T) {
		// App with tabs, newlines, and carriage returns in fields
		appList := &v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-with-special-chars",
						Namespace: "argocd",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "test\tproject",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/example/repo.git",
							Path:           "path\nwith\nnewlines",
							TargetRevision: "branch\rwith\rreturns",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: "OutOfSync",
						},
						Health: v1alpha1.HealthStatus{
							Status: "Progressing",
						},
						OperationState: &v1alpha1.OperationState{
							Phase:   "Running",
							Message: "Message\twith\ttabs\nand\nnewlines\rand\rreturns",
						},
					},
				},
			},
		}

		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "tsv", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		lines := strings.Split(strings.TrimSpace(textContent.Text), "\n")
		require.Len(t, lines, 2) // Header + 1 data row

		// Check that special characters are properly escaped - now only 4 minimal fields
		fields := strings.Split(lines[1], "\t")
		require.Len(t, fields, 4) // Only 4 minimal fields now
		assert.Equal(t, "app-with-special-chars", fields[0]) // name
		assert.Equal(t, "test\\tproject", fields[1])         // project with escaped tab
		assert.Equal(t, "OutOfSync", fields[2])             // syncStatus
		assert.Equal(t, "Progressing", fields[3])           // healthStatus
	})

	t.Run("TSV format with empty applications list", func(t *testing.T) {
		emptyAppList := &v1alpha1.ApplicationList{Items: []v1alpha1.Application{}}
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(emptyAppList, nil)

		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "tsv", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)
		assert.Equal(t, "No applications found matching the criteria.", textContent.Text)
	})

	t.Run("TSV format with missing source", func(t *testing.T) {
		appList := &v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-no-source",
						Namespace: "argocd",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source:  nil, // Missing source
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: "",
						},
						Health: v1alpha1.HealthStatus{
							Status: "",
						},
					},
				},
			},
		}

		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "tsv", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		lines := strings.Split(textContent.Text, "\n")
		// Remove last empty line if exists
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		require.Len(t, lines, 2) // Header + 1 data row

		// Verify structure has correct number of fields (now minimal 4-field format)
		headerTabCount := strings.Count(lines[0], "\t")
		dataTabCount := strings.Count(lines[1], "\t")
		require.Equal(t, headerTabCount, dataTabCount) // Both should have same number of tabs
		require.Equal(t, 3, dataTabCount)             // Should have 3 tabs = 4 fields

		fields := strings.Split(lines[1], "\t")
		require.Equal(t, 4, len(fields)) // 4 minimal fields (name, project, syncStatus, healthStatus)

		// Verify key fields
		assert.Equal(t, "app-no-source", fields[0]) // name
		assert.Equal(t, "default", fields[1])       // project
		assert.Equal(t, "", fields[2])             // syncStatus (empty)
		assert.Equal(t, "", fields[3])             // healthStatus (empty)
		// No chart field in minimal format
	})

	t.Run("TSV format backward compatibility", func(t *testing.T) {
		appList := &v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "legacy-app",
						Namespace: "argocd",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/example/repo.git",
							TargetRevision: "HEAD",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					},
				},
			},
		}

		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		// Test that JSON format still works
		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "json", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		var apps []ApplicationSummary
		err = json.Unmarshal([]byte(textContent.Text), &apps)
		require.NoError(t, err)
		assert.Len(t, apps, 1)
		assert.Equal(t, "legacy-app", apps[0].Name)
	})
}

func TestListApplicationsHandler_OptionalFields(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	// Setup mock with test applications
	appList := &v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-app",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://github.com/example/repo",
						Path:           "manifests",
						TargetRevision: "main",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://kubernetes.default.svc",
						Namespace: "default",
					},
				},
				Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeSynced,
					},
					Health: v1alpha1.HealthStatus{
						Status: "Healthy",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app2",
					Namespace: "argocd",
				},
				Spec: v1alpha1.ApplicationSpec{
					Project: "monitoring",
					Source: &v1alpha1.ApplicationSource{
						RepoURL:        "https://charts.helm.sh/stable",
						Chart:          "prometheus",
						TargetRevision: "15.18.0",
					},
					Destination: v1alpha1.ApplicationDestination{
						Server:    "https://prod-cluster.company.com:6443",
						Namespace: "monitoring",
					},
				},
				Status: v1alpha1.ApplicationStatus{
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeOutOfSync,
					},
					Health: v1alpha1.HealthStatus{
						Status: "Degraded",
					},
					OperationState: &v1alpha1.OperationState{
						Phase:   "Running",
						Message: "Syncing application resources",
						StartedAt: metav1.Time{
							Time: time.Date(2023, 12, 1, 10, 0, 0, 0, time.UTC),
						},
					},
				},
			},
		},
	}

	t.Run("TSV with optional_fields - source group", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "tsv", []string{"source"})
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		lines := strings.Split(strings.TrimSpace(textContent.Text), "\n")
		require.Len(t, lines, 3) // Header + 2 data rows

		// Check header (4 minimal + 4 source fields)
		expectedHeader := "name\tproject\tsyncStatus\thealthStatus\trepoURL\tpath\ttargetRevision\tchart"
		assert.Equal(t, expectedHeader, lines[0])

		// Check first app data (8 fields)
		app1Fields := strings.Split(lines[1], "\t")
		require.Len(t, app1Fields, 8)
		assert.Equal(t, "test-app", app1Fields[0])                       // name
		assert.Equal(t, "default", app1Fields[1])                         // project
		assert.Equal(t, "Synced", app1Fields[2])                          // syncStatus
		assert.Equal(t, "Healthy", app1Fields[3])                         // healthStatus
		assert.Equal(t, "https://github.com/example/repo", app1Fields[4]) // repoURL
		assert.Equal(t, "manifests", app1Fields[5])                       // path
		assert.Equal(t, "main", app1Fields[6])                            // targetRevision
		assert.Equal(t, "", app1Fields[7])                                // chart (empty)

		// Check second app data (helm chart)
		app2Fields := strings.Split(lines[2], "\t")
		require.Len(t, app2Fields, 8)
		assert.Equal(t, "app2", app2Fields[0])                          // name
		assert.Equal(t, "monitoring", app2Fields[1])                    // project
		assert.Equal(t, "https://charts.helm.sh/stable", app2Fields[4]) // repoURL
		assert.Equal(t, "", app2Fields[5])                              // path (empty for helm)
		assert.Equal(t, "15.18.0", app2Fields[6])                       // targetRevision
		assert.Equal(t, "prometheus", app2Fields[7])                    // chart
	})

	t.Run("TSV with optional_fields - individual fields", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "tsv", []string{"namespace", "source-repo", "dest-namespace"})
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		lines := strings.Split(strings.TrimSpace(textContent.Text), "\n")
		require.Len(t, lines, 3) // Header + 2 data rows

		// Check header (4 minimal + 3 optional fields = 7 total)
		expectedHeader := "name\tproject\tsyncStatus\thealthStatus\tnamespace\trepoURL\tdestNamespace"
		assert.Equal(t, expectedHeader, lines[0])

		// Check first app data (7 fields)
		app1Fields := strings.Split(lines[1], "\t")
		require.Len(t, app1Fields, 7)
		assert.Equal(t, "test-app", app1Fields[0])                       // name
		assert.Equal(t, "default", app1Fields[1])                         // project
		assert.Equal(t, "Synced", app1Fields[2])                          // syncStatus
		assert.Equal(t, "Healthy", app1Fields[3])                         // healthStatus
		assert.Equal(t, "argocd", app1Fields[4])                          // namespace
		assert.Equal(t, "https://github.com/example/repo", app1Fields[5]) // repoURL
		assert.Equal(t, "default", app1Fields[6])                         // destNamespace

		// Check second app data
		app2Fields := strings.Split(lines[2], "\t")
		require.Len(t, app2Fields, 7)
		assert.Equal(t, "app2", app2Fields[0])                          // name
		assert.Equal(t, "monitoring", app2Fields[1])                    // project
		assert.Equal(t, "https://charts.helm.sh/stable", app2Fields[5]) // repoURL
		assert.Equal(t, "monitoring", app2Fields[6])                    // destNamespace
	})

	t.Run("TSV with optional_fields - operation fields", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "tsv", []string{"operation"})
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		lines := strings.Split(strings.TrimSpace(textContent.Text), "\n")
		require.Len(t, lines, 3) // Header + 2 data rows

		// Check header (4 minimal + 3 operation fields = 7 total)
		expectedHeader := "name\tproject\tsyncStatus\thealthStatus\topPhase\topMessage\topStartedAt"
		assert.Equal(t, expectedHeader, lines[0])

		// Check first app data (no operation)
		app1Fields := strings.Split(lines[1], "\t")
		require.Len(t, app1Fields, 7)
		assert.Equal(t, "test-app", app1Fields[0]) // name
		assert.Equal(t, "default", app1Fields[1]) // project
		assert.Equal(t, "Synced", app1Fields[2])  // syncStatus
		assert.Equal(t, "Healthy", app1Fields[3]) // healthStatus
		assert.Equal(t, "", app1Fields[4])        // opPhase (empty)
		assert.Equal(t, "", app1Fields[5])        // opMessage (empty)
		assert.Equal(t, "", app1Fields[6])        // opStartedAt (empty)

		// Check second app data (has operation)
		app2Fields := strings.Split(lines[2], "\t")
		require.Len(t, app2Fields, 7)
		assert.Equal(t, "app2", app2Fields[0])                          // name
		assert.Equal(t, "monitoring", app2Fields[1])                    // project
		assert.Equal(t, "OutOfSync", app2Fields[2])                     // syncStatus
		assert.Equal(t, "Degraded", app2Fields[3])                      // healthStatus
		assert.Equal(t, "Running", app2Fields[4])                       // opPhase
		assert.Equal(t, "Syncing application resources", app2Fields[5]) // opMessage
		assert.Contains(t, app2Fields[6], "2023-12-01")                 // opStartedAt
	})

	t.Run("TSV with empty optional_fields", func(t *testing.T) {
		mockClient.EXPECT().ListApplications(gomock.Any(), "").Return(appList, nil)

		result, err := listApplicationsHandler(context.Background(), mockClient, "", "", "", "", false, false, "tsv", []string{})
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := mcp.AsTextContent(result.Content[0])
		require.True(t, ok)

		lines := strings.Split(strings.TrimSpace(textContent.Text), "\n")
		require.Len(t, lines, 3) // Header + 2 data rows

		// Check header (only 4 minimal fields)
		expectedHeader := "name\tproject\tsyncStatus\thealthStatus"
		assert.Equal(t, expectedHeader, lines[0])

		// Verify minimal data size
		app1Fields := strings.Split(lines[1], "\t")
		require.Len(t, app1Fields, 4)
		app2Fields := strings.Split(lines[2], "\t")
		require.Len(t, app2Fields, 4)
	})
}
