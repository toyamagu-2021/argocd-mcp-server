package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

// Test the tool schema
func TestGetAppManifestsTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetAppManifestsTool.Name != "get_application_manifests" {
		t.Errorf("Expected tool name 'get_application_manifests', got %s", GetAppManifestsTool.Name)
	}

	assert.Contains(t, GetAppManifestsTool.Description, "detailed=false")
	assert.Contains(t, GetAppManifestsTool.Description, "10")

	// Check input schema exists
	if GetAppManifestsTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetAppManifestsTool.InputSchema.Type)
	}

	// Check required parameters
	require.Contains(t, GetAppManifestsTool.InputSchema.Required, "name")
	require.NotContains(t, GetAppManifestsTool.InputSchema.Required, "revision") // revision is optional

	// Check parameter properties
	props := GetAppManifestsTool.InputSchema.Properties
	require.NotNil(t, props)

	// Check name parameter
	nameProp, ok := props["name"].(map[string]interface{})
	require.True(t, ok, "name property should be a map")
	assert.Equal(t, "string", nameProp["type"])
	assert.NotEmpty(t, nameProp["description"])

	// Check revision parameter (optional)
	revisionProp, ok := props["revision"].(map[string]interface{})
	require.True(t, ok, "revision property should be a map")
	assert.Equal(t, "string", revisionProp["type"])
	assert.NotEmpty(t, revisionProp["description"])
}

// Test the handler logic with mocked client
func TestGetApplicationManifestsHandler(t *testing.T) {
	// Sample manifest response - simulate what ArgoCD returns
	mockManifestResponse := &repoapiclient.ManifestResponse{
		Manifests: []string{
			"apiVersion: v1\nkind: Service\nmetadata:\n  name: test-service\n",
			"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test-deployment\n",
		},
		Namespace: "default",
		Server:    "https://kubernetes.default.svc",
		Revision:  "abc123",
	}

	tests := []struct {
		name        string
		appName     string
		revision    string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:     "successful operation without revision",
			appName:  "test-app",
			revision: "",
			setupMock: func(m *mock.MockInterface) {
				// The mock interface's GetApplicationManifests should just return the result
				// The real client implementation handles the GetApplication internally
				m.EXPECT().GetApplicationManifests(gomock.Any(), "test-app", "", "").Return(mockManifestResponse, nil)
			},
			wantError: false,
		},
		{
			name:     "successful operation with revision",
			appName:  "test-app",
			revision: "v1.0.0",
			setupMock: func(m *mock.MockInterface) {
				// The mock interface's GetApplicationManifests should just return the result
				// The real client implementation handles the GetApplication internally
				m.EXPECT().GetApplicationManifests(gomock.Any(), "test-app", "v1.0.0", "").Return(mockManifestResponse, nil)
			},
			wantError: false,
		},
		{
			name:     "application not found",
			appName:  "nonexistent-app",
			revision: "",
			setupMock: func(m *mock.MockInterface) {
				// The mock returns an error when manifests are requested
				m.EXPECT().GetApplicationManifests(gomock.Any(), "nonexistent-app", "", "").Return(nil, assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to get application manifests",
		},
		{
			name:     "empty application name",
			appName:  "",
			revision: "",
			setupMock: func(m *mock.MockInterface) {
				// No mock expectations as it should fail before calling the client
			},
			wantError:   true,
			wantMessage: "Application name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			if tt.setupMock != nil {
				tt.setupMock(mockClient)
			}

			result, err := getApplicationManifestsHandler(context.Background(), mockClient, tt.appName, tt.revision, "", false, 10, 0)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.wantMessage != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := result.Content[0].(mcp.TextContent)
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.wantMessage)
				}
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)
				require.Len(t, result.Content, 1)

				// Verify the response is valid JSON
				textContent, ok := result.Content[0].(mcp.TextContent)
				require.True(t, ok)
				var responseData map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &responseData)
				assert.NoError(t, err, "Response should be valid JSON")

				assert.Contains(t, responseData, "items")
				assert.Contains(t, responseData, "namespace")
				assert.Contains(t, responseData, "totalItems")
				assert.NotContains(t, responseData, "manifests")
				assert.Contains(t, textContent.Text, "test-service")
			}
		})
	}
}

func TestGetApplicationManifestsPagingAndSize(t *testing.T) {
	pad := "managedfields-marker:" + strings.Repeat("m", 2048)
	manifests := make([]string, 15)
	for i := range manifests {
		manifests[i] = `{"kind":"ConfigMap","metadata":{"name":"cm-` + itoaSmall(i) + `","managedFields":[{"manager":"` + pad + `"}]}}`
	}
	resp := &repoapiclient.ManifestResponse{
		Manifests:  manifests,
		Namespace:  "default",
		Server:     "https://kubernetes.default.svc",
		Revision:   "abc123",
		SourceType: "Helm",
		Commands:   []string{"helm template"},
		SourceIntegrityResult: &v1alpha1.SourceIntegrityCheckResult{
			Checks: []v1alpha1.SourceIntegrityCheckResultItem{{Name: "signature"}},
		},
	}
	before, err := json.MarshalIndent(resp, "", "  ")
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationManifests(gomock.Any(), "app", "main", "").Return(resp, nil).Times(2)

	summaryRes, err := getApplicationManifestsHandler(context.Background(), m, "app", "main", "", false, defaultManifestLimit, 0)
	require.NoError(t, err)
	summary := toolText(t, summaryRes)
	var page struct {
		Items      []string `json:"items"`
		TotalItems int      `json:"totalItems"`
		HasMore    bool     `json:"hasMore"`
		Notice     string   `json:"notice"`
		Namespace  string   `json:"namespace"`
		Revision   string   `json:"revision"`
		SourceType string   `json:"sourceType"`
	}
	require.NoError(t, json.Unmarshal([]byte(summary), &page))
	assert.Len(t, page.Items, 10)
	assert.Equal(t, 15, page.TotalItems)
	assert.True(t, page.HasMore)
	assert.Contains(t, page.Notice, "offset=10")
	assert.Equal(t, "default", page.Namespace)
	assert.Equal(t, "abc123", page.Revision)
	assert.Equal(t, "Helm", page.SourceType)
	assert.NotContains(t, summary, "managedFields")
	assert.NotContains(t, summary, "managedfields-marker")
	assert.NotContains(t, summary, "helm template")
	assert.NotContains(t, summary, "signature")
	assert.Contains(t, page.Items[0], `"kind":"ConfigMap"`)
	fieldBytes := strings.Count(string(before), pad) * len(pad)
	t.Logf("before_bytes=%d after_bytes=%d managedfields_bytes=%d", len(before), len(summary), fieldBytes)
	assert.Less(t, len(summary), len(before)/2)

	detailed := toolText(t, mustManifests(t, m, true))
	assert.Contains(t, detailed, "helm template")
	assert.Contains(t, detailed, "signature")
	assert.NotContains(t, detailed, "managedFields")
	assert.Contains(t, detailed, `"hasMore":true`)
}

func TestGetApplicationManifestsYAMLStrip(t *testing.T) {
	raw := "kind: ConfigMap\nmetadata:\n  name: yaml-cm\n  annotations:\n    keep-me: kept\n    " + lastAppliedAnnotation + ": blob\n"
	resp := &repoapiclient.ManifestResponse{Manifests: []string{raw}, Namespace: "default"}
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationManifests(gomock.Any(), "app", "", "").Return(resp, nil)

	res, err := getApplicationManifestsHandler(context.Background(), m, "app", "", "", false, defaultManifestLimit, 0)
	require.NoError(t, err)
	text := toolText(t, res)
	var page struct {
		Items []string `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &page))
	require.Len(t, page.Items, 1)
	assert.NotContains(t, page.Items[0], lastAppliedAnnotation)
	assert.Contains(t, page.Items[0], "keep-me")
	assert.False(t, strings.HasPrefix(strings.TrimSpace(page.Items[0]), "{"))
	assert.Contains(t, page.Items[0], "yaml-cm")
}

func mustManifests(t *testing.T, m *mock.MockInterface, detailed bool) *mcp.CallToolResult {
	t.Helper()
	res, err := getApplicationManifestsHandler(context.Background(), m, "app", "main", "", detailed, defaultManifestLimit, 0)
	require.NoError(t, err)
	return res
}
