package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test the tool schema
func TestGetApplicationSetTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetApplicationSetTool.Name != "get_applicationset" {
		t.Errorf("Expected tool name 'get_applicationset', got %s", GetApplicationSetTool.Name)
	}

	assert.Contains(t, GetApplicationSetTool.Description, "detailed=false")

	// Check input schema exists
	if GetApplicationSetTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetApplicationSetTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(GetApplicationSetTool.InputSchema.Properties) == 0 {
		t.Error("Tool schema should have properties defined")
	}

	// Check name parameter exists
	props := GetApplicationSetTool.InputSchema.Properties
	if _, ok := props["name"]; !ok {
		t.Error("Expected 'name' property to be defined")
	}

	// Check that name is required
	required := GetApplicationSetTool.InputSchema.Required
	nameRequired := false
	for _, r := range required {
		if r == "name" {
			nameRequired = true
			break
		}
	}
	if !nameRequired {
		t.Error("Expected 'name' to be a required parameter")
	}
}

// Test the handler logic with mocked client
func TestGetApplicationSetHandler(t *testing.T) {
	tests := []struct {
		name        string
		appSetName  string
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:       "successful get",
			appSetName: "test-appset",
			setupMock: func(m *mock.MockInterface) {
				appSet := &v1alpha1.ApplicationSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-appset",
						Namespace: "argocd",
						Labels: map[string]string{
							"env": "prod",
						},
					},
					Spec: v1alpha1.ApplicationSetSpec{
						Template: v1alpha1.ApplicationSetTemplate{
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
						},
					},
				}
				m.EXPECT().GetApplicationSet(gomock.Any(), "test-appset").Return(appSet, nil)
			},
			wantError:   false,
			wantMessage: "test-appset",
		},
		{
			name:       "applicationset not found",
			appSetName: "non-existent",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationSet(gomock.Any(), "non-existent").Return(nil, assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to get ApplicationSet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := getApplicationSetHandler(context.Background(), mockClient, tt.appSetName, false)

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

				text := toolText(t, result)
				assert.NotContains(t, text, "totalItems")
				assert.NotContains(t, text, `"generators"`)
				assert.NotContains(t, text, "metadata")
				var summary applicationSetSummary
				require.NoError(t, json.Unmarshal([]byte(text), &summary))
				assert.Equal(t, tt.appSetName, summary.Name)
				assert.Equal(t, "argocd", summary.Namespace)
				assert.Equal(t, "default", summary.Project)
				assert.Equal(t, 0, summary.GeneratorCount)
			}
		})
	}
}

func TestGetApplicationSetSummarySize(t *testing.T) {
	values := "helm-values-marker:" + strings.Repeat("v", 2048)
	appSet := &v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "fat",
			Namespace:     "argocd",
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
			Annotations:   map[string]string{lastAppliedAnnotation: "applied"},
		},
		Spec: v1alpha1.ApplicationSetSpec{
			Generators: []v1alpha1.ApplicationSetGenerator{{}},
			Template: v1alpha1.ApplicationSetTemplate{
				Spec: v1alpha1.ApplicationSpec{
					Project: "default",
					Source: &v1alpha1.ApplicationSource{
						RepoURL: "https://example.com/repo",
						Helm:    &v1alpha1.ApplicationSourceHelm{Values: values},
					},
				},
			},
		},
	}
	before, err := json.MarshalIndent(appSet, "", "  ")
	require.NoError(t, err)
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationSet(gomock.Any(), "fat").Return(appSet, nil).Times(2)

	summaryRes, err := getApplicationSetHandler(context.Background(), m, "fat", false)
	require.NoError(t, err)
	summary := toolText(t, summaryRes)
	assert.NotContains(t, summary, "helm-values-marker")
	assert.NotContains(t, summary, `"generators"`)
	assert.NotContains(t, summary, "totalItems")
	assert.Contains(t, summary, `"generatorCount":1`)
	assert.Contains(t, summary, `"project":"default"`)
	assert.Contains(t, summary, `"name":"fat"`)
	valueBytes := strings.Count(string(before), values) * len(values)
	t.Logf("before_bytes=%d after_bytes=%d values_bytes=%d", len(before), len(summary), valueBytes)
	assert.Less(t, len(summary), len(before)/5)

	detailedRes, err := getApplicationSetHandler(context.Background(), m, "fat", true)
	require.NoError(t, err)
	detailed := toolText(t, detailedRes)
	assert.Contains(t, detailed, "helm-values-marker")
	assert.Contains(t, detailed, `"generators"`)
	assert.NotContains(t, detailed, "managedFields")
	assert.NotContains(t, detailed, lastAppliedAnnotation)
	assert.NotContains(t, detailed, "totalItems")
	assert.Less(t, len(detailed), len(before))
	var full v1alpha1.ApplicationSet
	require.NoError(t, json.Unmarshal([]byte(detailed), &full))
	assert.Equal(t, "fat", full.Name)
	assert.Equal(t, values, full.Spec.Template.Spec.Source.Helm.Values)
}
