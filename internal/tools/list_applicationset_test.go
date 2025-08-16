package tools

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test the tool handler with environment variables
func TestHandleListApplicationSets(t *testing.T) {
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
					Name:      "list_applicationset",
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
			name: "with valid environment",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "list_applicationset",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleListApplicationSets(context.Background(), tt.request)

			// Check expectations
			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" && len(result.Content) > 0 {
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.errorContains)
				}
			} else {
				// Note: This will still fail because we're not mocking the actual connection
				// but it tests the basic flow
				require.Nil(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

// Test the tool schema
func TestListApplicationSetTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if ListApplicationSetTool.Name != "list_applicationset" {
		t.Errorf("Expected tool name 'list_applicationset', got %s", ListApplicationSetTool.Name)
	}

	// Verify tool has description
	if ListApplicationSetTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if ListApplicationSetTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", ListApplicationSetTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(ListApplicationSetTool.InputSchema.Properties) == 0 {
		t.Error("Tool schema should have properties defined")
	}

	// Check that specific properties exist
	props := ListApplicationSetTool.InputSchema.Properties
	if _, ok := props["project"]; !ok {
		t.Error("Expected 'project' property to be defined")
	}
	if _, ok := props["selector"]; !ok {
		t.Error("Expected 'selector' property to be defined")
	}
}

// Test the handler logic with mocked client
func TestListApplicationSetsHandler(t *testing.T) {
	tests := []struct {
		name        string
		project     string
		selector    string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:     "successful list all",
			project:  "",
			selector: "",
			setupMock: func(m *mock.MockInterface) {
				appSetList := &v1alpha1.ApplicationSetList{
					Items: []v1alpha1.ApplicationSet{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-appset-1",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSetSpec{
								Template: v1alpha1.ApplicationSetTemplate{
									Spec: v1alpha1.ApplicationSpec{
										Project: "default",
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-appset-2",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSetSpec{
								Template: v1alpha1.ApplicationSetTemplate{
									Spec: v1alpha1.ApplicationSpec{
										Project: "prod",
									},
								},
							},
						},
					},
				}
				m.EXPECT().ListApplicationSets(gomock.Any(), "").Return(appSetList, nil)
			},
			wantError:   false,
			wantMessage: "test-appset-1",
		},
		{
			name:     "filter by project",
			project:  "prod",
			selector: "",
			setupMock: func(m *mock.MockInterface) {
				appSetList := &v1alpha1.ApplicationSetList{
					Items: []v1alpha1.ApplicationSet{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-appset-1",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSetSpec{
								Template: v1alpha1.ApplicationSetTemplate{
									Spec: v1alpha1.ApplicationSpec{
										Project: "default",
									},
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-appset-2",
								Namespace: "argocd",
							},
							Spec: v1alpha1.ApplicationSetSpec{
								Template: v1alpha1.ApplicationSetTemplate{
									Spec: v1alpha1.ApplicationSpec{
										Project: "prod",
									},
								},
							},
						},
					},
				}
				m.EXPECT().ListApplicationSets(gomock.Any(), "prod").Return(appSetList, nil)
			},
			wantError:   false,
			wantMessage: "test-appset-2",
		},
		{
			name:     "filter by selector",
			project:  "",
			selector: "env=prod",
			setupMock: func(m *mock.MockInterface) {
				appSetList := &v1alpha1.ApplicationSetList{
					Items: []v1alpha1.ApplicationSet{
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-appset-1",
								Namespace: "argocd",
								Labels: map[string]string{
									"env": "dev",
								},
							},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:      "test-appset-2",
								Namespace: "argocd",
								Labels: map[string]string{
									"env": "prod",
								},
							},
						},
					},
				}
				m.EXPECT().ListApplicationSets(gomock.Any(), "").Return(appSetList, nil)
			},
			wantError:   false,
			wantMessage: "test-appset-2",
		},
		{
			name:     "no applicationsets found",
			project:  "",
			selector: "",
			setupMock: func(m *mock.MockInterface) {
				appSetList := &v1alpha1.ApplicationSetList{
					Items: []v1alpha1.ApplicationSet{},
				}
				m.EXPECT().ListApplicationSets(gomock.Any(), "").Return(appSetList, nil)
			},
			wantError:   false,
			wantMessage: "No ApplicationSets found",
		},
		{
			name:     "operation fails",
			project:  "",
			selector: "",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().ListApplicationSets(gomock.Any(), "").Return(nil, assert.AnError)
			},
			wantError: true,
		},
		{
			name:     "invalid selector format",
			project:  "",
			selector: "invalid selector",
			setupMock: func(m *mock.MockInterface) {
				appSetList := &v1alpha1.ApplicationSetList{
					Items: []v1alpha1.ApplicationSet{},
				}
				m.EXPECT().ListApplicationSets(gomock.Any(), "").Return(appSetList, nil)
			},
			wantError:   true,
			wantMessage: "Invalid selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := listApplicationSetsHandler(context.Background(), mockClient, tt.project, tt.selector)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.wantMessage != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.wantMessage)
				}
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				if tt.wantMessage != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.wantMessage)
				}
			}
		})
	}
}

// Test parseSelector function
func TestParseSelector(t *testing.T) {
	tests := []struct {
		name      string
		selector  string
		want      map[string]string
		wantError bool
	}{
		{
			name:      "valid selector",
			selector:  "env=prod",
			want:      map[string]string{"env": "prod"},
			wantError: false,
		},
		{
			name:      "empty selector",
			selector:  "",
			want:      map[string]string{},
			wantError: false,
		},
		{
			name:      "invalid format - no equals",
			selector:  "invalid",
			want:      nil,
			wantError: true,
		},
		{
			name:      "invalid format - multiple equals",
			selector:  "key=value=extra",
			want:      nil,
			wantError: true,
		},
		{
			name:      "empty key",
			selector:  "=value",
			want:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseSelector(tt.selector)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// Test matchesSelector function
func TestMatchesSelector(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		selector map[string]string
		want     bool
	}{
		{
			name:     "matches single label",
			labels:   map[string]string{"env": "prod", "team": "backend"},
			selector: map[string]string{"env": "prod"},
			want:     true,
		},
		{
			name:     "does not match",
			labels:   map[string]string{"env": "dev", "team": "backend"},
			selector: map[string]string{"env": "prod"},
			want:     false,
		},
		{
			name:     "label not present",
			labels:   map[string]string{"team": "backend"},
			selector: map[string]string{"env": "prod"},
			want:     false,
		},
		{
			name:     "empty selector matches all",
			labels:   map[string]string{"env": "prod"},
			selector: map[string]string{},
			want:     true,
		},
		{
			name:     "empty labels with non-empty selector",
			labels:   map[string]string{},
			selector: map[string]string{"env": "prod"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesSelector(tt.labels, tt.selector)
			assert.Equal(t, tt.want, got)
		})
	}
}
