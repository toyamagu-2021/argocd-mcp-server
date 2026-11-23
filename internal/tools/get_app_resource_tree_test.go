package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
)

// Test the tool schema
func TestGetApplicationResourceTreeTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetApplicationResourceTreeTool.Name != "get_application_resource_tree" {
		t.Errorf("Expected tool name 'get_application_resource_tree', got %s", GetApplicationResourceTreeTool.Name)
	}

	assert.Contains(t, GetApplicationResourceTreeTool.Description, "detailed=false")
	assert.Contains(t, GetApplicationResourceTreeTool.Description, "50")
	assert.Contains(t, GetApplicationResourceTreeTool.Description, "systemInfo")

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

			result, err := getApplicationResourceTreeHandler(context.Background(), mockClient, tt.appName, tt.appNamespace, tt.project, false, 50, 0)

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

				assert.NotContains(t, textContent.Text, "\n")
				assert.Contains(t, textContent.Text, `"totalItems":4`)
				assert.Contains(t, textContent.Text, "my-service")
				assert.Contains(t, textContent.Text, "my-deployment")
				assert.Contains(t, textContent.Text, "orphaned-config")
				assert.Contains(t, textContent.Text, `"orphaned":true`)
				assert.NotContains(t, textContent.Text, "nginx:latest")
				assert.NotContains(t, textContent.Text, "Service is healthy")
				assert.Equal(t, 1, strings.Count(textContent.Text, "service-uid"))
				assert.Equal(t, 1, strings.Count(textContent.Text, "deployment-uid"))
			}
		})
	}
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

	result, err := getApplicationResourceTreeHandler(context.Background(), mockClient, "empty-app", "argocd", "default", false, 50, 0)

	require.Nil(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	require.Len(t, result.Content, 1)
	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)
	assert.Equal(t, `{"items":[],"totalItems":0,"hasMore":false}`, textContent.Text)
}

func TestGetApplicationResourceTreeSummaryPage(t *testing.T) {
	tree := sampleResourceTree()
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationResourceTree(gomock.Any(), "guestbook", "", "").Return(tree, nil).Times(3)

	summaryRes, err := getApplicationResourceTreeHandler(context.Background(), m, "guestbook", "", "", false, 50, 0)
	require.NoError(t, err)
	summaryText := toolText(t, summaryRes)
	for _, forbidden := range []string{"systemInfo", "networkingInfo", "images", "image-marker", "networking-marker", "sysinfo-marker"} {
		assert.NotContains(t, summaryText, forbidden)
	}
	assert.Contains(t, summaryText, `"totalItems":40`)
	assert.Contains(t, summaryText, `"hasMore":false`)
	assert.Contains(t, summaryText, "node-0")
	assert.Contains(t, summaryText, "host-a")
	assert.Contains(t, summaryText, "host-b")
	assert.NotContains(t, summaryText, "parent-uid")

	var summary struct {
		Items []struct {
			Name     string `json:"name"`
			Orphaned bool   `json:"orphaned"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(summaryText), &summary))
	require.Len(t, summary.Items, 40)
	for i, item := range summary.Items {
		assert.Equal(t, i >= 30, item.Orphaned, item.Name)
	}

	pageRes, err := getApplicationResourceTreeHandler(context.Background(), m, "guestbook", "", "", false, 1, 0)
	require.NoError(t, err)
	pageText := toolText(t, pageRes)
	assert.Contains(t, pageText, `"hasMore":true`)
	assert.Contains(t, pageText, "Showing items 0-0 of 40. Pass offset=1 to fetch more.")
	assert.Contains(t, pageText, "host-a")
	assert.Contains(t, pageText, "host-b")
	assert.NotContains(t, pageText, "node-1")

	detailedRes, err := getApplicationResourceTreeHandler(context.Background(), m, "guestbook", "", "", true, 50, 0)
	require.NoError(t, err)
	detailedText := toolText(t, detailedRes)
	assert.Contains(t, detailedText, "networkingInfo")
	assert.Contains(t, detailedText, "networking-marker")
	assert.Contains(t, detailedText, "image-marker")
	assert.Contains(t, detailedText, "systemInfo")
	assert.Contains(t, detailedText, "sysinfo-marker")
}

func TestGetApplicationResourceTreeSummarySize(t *testing.T) {
	tree := sampleResourceTree()
	before, err := json.MarshalIndent(tree, "", "  ")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationResourceTree(gomock.Any(), "guestbook", "", "").Return(tree, nil)
	res, err := getApplicationResourceTreeHandler(context.Background(), m, "guestbook", "", "", false, 50, 0)
	require.NoError(t, err)
	text := toolText(t, res)
	systemInfoBytes := strings.Count(string(before), resourceTreeSystemInfoPad) * len(resourceTreeSystemInfoPad)
	t.Logf("before_bytes=%d after_bytes=%d systeminfo_bytes=%d", len(before), len(text), systemInfoBytes)
	assert.Less(t, len(text), len(before)/5)
	assert.Greater(t, systemInfoBytes, len(text)*4)
	assert.Contains(t, text, "node-0")
	assert.NotContains(t, text, "sysinfo-marker")
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.False(t, res.IsError)
	require.Len(t, res.Content, 1)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	return text.Text
}

var resourceTreeSystemInfoPad = "sysinfo-marker:" + strings.Repeat("S", 4096)

func sampleResourceTree() *v1alpha1.ApplicationTree {
	pad := resourceTreeSystemInfoPad
	info := corev1.NodeSystemInfo{
		MachineID: pad, SystemUUID: pad, BootID: pad, KernelVersion: pad, OSImage: pad,
		ContainerRuntimeVersion: pad, KubeletVersion: pad,
		OperatingSystem: pad, Architecture: pad,
	}
	nodes := make([]v1alpha1.ResourceNode, 30)
	for i := range nodes {
		nodes[i] = v1alpha1.ResourceNode{
			ResourceRef: v1alpha1.ResourceRef{
				Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default",
				Name: "node-" + strconv.Itoa(i), UID: "uid-" + strconv.Itoa(i),
			},
			Health: &v1alpha1.HealthStatus{Status: "Healthy"},
		}
	}
	nodes[0].Images = []string{"image-marker"}
	nodes[0].NetworkingInfo = &v1alpha1.ResourceNetworkingInfo{ExternalURLs: []string{"http://networking-marker.example"}}
	nodes[1].ParentRefs = []v1alpha1.ResourceRef{{Kind: "Deployment", Name: "node-0", UID: "parent-uid"}}
	orphaned := make([]v1alpha1.ResourceNode, 10)
	for i := range orphaned {
		orphaned[i] = v1alpha1.ResourceNode{
			ResourceRef: v1alpha1.ResourceRef{Version: "v1", Kind: "ConfigMap", Namespace: "default", Name: "orphan-" + strconv.Itoa(i)},
		}
	}
	return &v1alpha1.ApplicationTree{
		Nodes:         nodes,
		OrphanedNodes: orphaned,
		Hosts: []v1alpha1.HostInfo{
			{Name: "host-a", SystemInfo: info},
			{Name: "host-b", SystemInfo: info},
		},
	}
}
