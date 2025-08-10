package tools

import (
	"context"
	"encoding/json"
	"testing"

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
	if ListAppsTool.InputSchema.Properties == nil || len(ListAppsTool.InputSchema.Properties) == 0 {
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
			result, err := listApplicationsHandler(ctx, mockClient, tt.project, tt.cluster, tt.namespace, tt.selector)

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

				var apps []v1alpha1.Application
				err := json.Unmarshal([]byte(textContent.Text), &apps)
				require.NoError(t, err)
				assert.Len(t, apps, tt.wantApps)
			}
		})
	}
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
	result, err := listApplicationsHandler(ctx, mockClient, "project1", "https://cluster1.example.com", "", "")
	require.NoError(t, err)
	require.NotNil(t, result)

	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var apps []v1alpha1.Application
	err = json.Unmarshal([]byte(textContent.Text), &apps)
	require.NoError(t, err)
	assert.Len(t, apps, 1)
	assert.Equal(t, "app1", apps[0].Name)
}
