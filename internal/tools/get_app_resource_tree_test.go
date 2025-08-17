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
)

// Test the tool handler with environment variables
func TestHandleGetApplicationResourceTree(t *testing.T) {
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
					Name: "get_application_resource_tree",
					Arguments: map[string]interface{}{
						"name": "test-app",
					},
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
			name: "missing name parameter",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_application_resource_tree",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "argocd.example.com:443",
			},
			wantError:     true,
			errorContains: "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleGetApplicationResourceTree(context.Background(), tt.request)

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
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)
			}
		})
	}
}

// Test the tool schema
func TestGetApplicationResourceTreeTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetApplicationResourceTreeTool.Name != "get_application_resource_tree" {
		t.Errorf("Expected tool name 'get_application_resource_tree', got %s", GetApplicationResourceTreeTool.Name)
	}

	// Verify tool has description
	if GetApplicationResourceTreeTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetApplicationResourceTreeTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetApplicationResourceTreeTool.InputSchema.Type)
	}

	// Check required parameters
	requiredParams := GetApplicationResourceTreeTool.InputSchema.Required
	if len(requiredParams) != 1 || requiredParams[0] != "name" {
		t.Errorf("Expected required parameter 'name', got %v", requiredParams)
	}

	// Check properties
	properties := GetApplicationResourceTreeTool.InputSchema.Properties
	if _, ok := properties["name"]; !ok {
		t.Error("Expected 'name' property in schema")
	}
	if _, ok := properties["app_namespace"]; !ok {
		t.Error("Expected 'app_namespace' property in schema")
	}
	if _, ok := properties["project"]; !ok {
		t.Error("Expected 'project' property in schema")
	}
}

// Test the handler logic with mocked client
func TestGetApplicationResourceTreeHandler(t *testing.T) {
	// Create a sample ApplicationTree
	sampleTree := &v1alpha1.ApplicationTree{
		Nodes: []v1alpha1.ResourceNode{
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "",
					Version:   "v1",
					Kind:      "Service",
					Namespace: "default",
					Name:      "my-service",
					UID:       "service-uid",
				},
				Health: &v1alpha1.HealthStatus{
					Status:  "Healthy",
					Message: "Service is healthy",
				},
			},
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "apps",
					Version:   "v1",
					Kind:      "Deployment",
					Namespace: "default",
					Name:      "my-deployment",
					UID:       "deployment-uid",
				},
				ParentRefs: []v1alpha1.ResourceRef{
					{
						Group:     "",
						Version:   "v1",
						Kind:      "Service",
						Namespace: "default",
						Name:      "my-service",
						UID:       "service-uid",
					},
				},
				Health: &v1alpha1.HealthStatus{
					Status:  "Healthy",
					Message: "Deployment has minimum availability",
				},
				Images: []string{"nginx:latest"},
			},
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "",
					Version:   "v1",
					Kind:      "Pod",
					Namespace: "default",
					Name:      "my-deployment-abc123",
					UID:       "pod-uid",
				},
				ParentRefs: []v1alpha1.ResourceRef{
					{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Deployment",
						Namespace: "default",
						Name:      "my-deployment",
						UID:       "deployment-uid",
					},
				},
				Health: &v1alpha1.HealthStatus{
					Status:  "Healthy",
					Message: "Pod is running",
				},
			},
		},
		OrphanedNodes: []v1alpha1.ResourceNode{
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "",
					Version:   "v1",
					Kind:      "ConfigMap",
					Namespace: "default",
					Name:      "orphaned-config",
					UID:       "config-uid",
				},
			},
		},
	}

	tests := []struct {
		name         string
		appName      string
		appNamespace string
		project      string
		setupMock    func(*mock.MockInterface)
		wantError    bool
		wantMessage  string
	}{
		{
			name:         "successful resource tree retrieval",
			appName:      "test-app",
			appNamespace: "argocd",
			project:      "default",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationResourceTree(gomock.Any(), "test-app", "argocd", "default").Return(sampleTree, nil)
			},
			wantError: false,
		},
		{
			name:         "resource tree retrieval with auto-detect",
			appName:      "test-app",
			appNamespace: "",
			project:      "",
			setupMock: func(m *mock.MockInterface) {
				// When namespace/project are empty, the client will fetch them from the app
				m.EXPECT().GetApplicationResourceTree(gomock.Any(), "test-app", "", "").Return(sampleTree, nil)
			},
			wantError: false,
		},
		{
			name:         "resource tree retrieval fails",
			appName:      "test-app",
			appNamespace: "argocd",
			project:      "default",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationResourceTree(gomock.Any(), "test-app", "argocd", "default").Return(nil, assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to get application resource tree",
		},
		{
			name:         "application not found",
			appName:      "non-existent-app",
			appNamespace: "argocd",
			project:      "default",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationResourceTree(gomock.Any(), "non-existent-app", "argocd", "default").Return(nil, assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to get application resource tree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := getApplicationResourceTreeHandler(context.Background(), mockClient, tt.appName, tt.appNamespace, tt.project)

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

				// Verify the response is valid JSON
				require.Len(t, result.Content, 1)
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)

				var tree v1alpha1.ApplicationTree
				err := json.Unmarshal([]byte(textContent.Text), &tree)
				require.NoError(t, err)

				// Verify the tree contains expected data
				assert.Len(t, tree.Nodes, 3)
				assert.Len(t, tree.OrphanedNodes, 1)
			}
		})
	}
}

// Test the resource tree with complex hierarchy
func TestGetApplicationResourceTreeHandler_ComplexTree(t *testing.T) {
	// Create a complex ApplicationTree with multiple levels
	complexTree := &v1alpha1.ApplicationTree{
		Nodes: []v1alpha1.ResourceNode{
			// Root application
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "argoproj.io",
					Version:   "v1alpha1",
					Kind:      "Application",
					Namespace: "argocd",
					Name:      "root-app",
					UID:       "root-uid",
				},
			},
			// Service
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "",
					Version:   "v1",
					Kind:      "Service",
					Namespace: "production",
					Name:      "web-service",
					UID:       "service-uid",
				},
				ParentRefs: []v1alpha1.ResourceRef{
					{
						Group:     "argoproj.io",
						Version:   "v1alpha1",
						Kind:      "Application",
						Namespace: "argocd",
						Name:      "root-app",
						UID:       "root-uid",
					},
				},
			},
			// StatefulSet
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "apps",
					Version:   "v1",
					Kind:      "StatefulSet",
					Namespace: "production",
					Name:      "database",
					UID:       "statefulset-uid",
				},
				ParentRefs: []v1alpha1.ResourceRef{
					{
						Group:     "argoproj.io",
						Version:   "v1alpha1",
						Kind:      "Application",
						Namespace: "argocd",
						Name:      "root-app",
						UID:       "root-uid",
					},
				},
			},
			// PersistentVolumeClaim
			{
				ResourceRef: v1alpha1.ResourceRef{
					Group:     "",
					Version:   "v1",
					Kind:      "PersistentVolumeClaim",
					Namespace: "production",
					Name:      "database-pvc",
					UID:       "pvc-uid",
				},
				ParentRefs: []v1alpha1.ResourceRef{
					{
						Group:     "apps",
						Version:   "v1",
						Kind:      "StatefulSet",
						Namespace: "production",
						Name:      "database",
						UID:       "statefulset-uid",
					},
				},
			},
		},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)
	mockClient.EXPECT().GetApplicationResourceTree(gomock.Any(), "complex-app", "", "").Return(complexTree, nil)

	result, err := getApplicationResourceTreeHandler(context.Background(), mockClient, "complex-app", "", "")

	require.Nil(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Verify the response contains the complex tree structure
	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var tree v1alpha1.ApplicationTree
	err = json.Unmarshal([]byte(textContent.Text), &tree)
	require.NoError(t, err)

	// Verify the hierarchy
	assert.Len(t, tree.Nodes, 4)

	// Check for root application
	var rootApp *v1alpha1.ResourceNode
	for i := range tree.Nodes {
		if tree.Nodes[i].Kind == "Application" {
			rootApp = &tree.Nodes[i]
			break
		}
	}
	require.NotNil(t, rootApp)
	assert.Equal(t, "root-app", rootApp.Name)

	// Check for StatefulSet with parent reference
	var statefulSet *v1alpha1.ResourceNode
	for i := range tree.Nodes {
		if tree.Nodes[i].Kind == "StatefulSet" {
			statefulSet = &tree.Nodes[i]
			break
		}
	}
	require.NotNil(t, statefulSet)
	assert.Equal(t, "database", statefulSet.Name)
	assert.Len(t, statefulSet.ParentRefs, 1)

	// Check for PVC with parent reference to StatefulSet
	var pvc *v1alpha1.ResourceNode
	for i := range tree.Nodes {
		if tree.Nodes[i].Kind == "PersistentVolumeClaim" {
			pvc = &tree.Nodes[i]
			break
		}
	}
	require.NotNil(t, pvc)
	assert.Equal(t, "database-pvc", pvc.Name)
	assert.Len(t, pvc.ParentRefs, 1)
	assert.Equal(t, "StatefulSet", pvc.ParentRefs[0].Kind)
}

// Test empty resource tree
func TestGetApplicationResourceTreeHandler_EmptyTree(t *testing.T) {
	emptyTree := &v1alpha1.ApplicationTree{
		Nodes:         []v1alpha1.ResourceNode{},
		OrphanedNodes: []v1alpha1.ResourceNode{},
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)
	mockClient.EXPECT().GetApplicationResourceTree(gomock.Any(), "empty-app", "argocd", "default").Return(emptyTree, nil)

	result, err := getApplicationResourceTreeHandler(context.Background(), mockClient, "empty-app", "argocd", "default")

	require.Nil(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Verify the response contains an empty tree
	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	var tree v1alpha1.ApplicationTree
	err = json.Unmarshal([]byte(textContent.Text), &tree)
	require.NoError(t, err)

	assert.Len(t, tree.Nodes, 0)
	assert.Len(t, tree.OrphanedNodes, 0)
}
