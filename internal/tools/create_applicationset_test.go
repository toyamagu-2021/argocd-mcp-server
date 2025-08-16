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

func TestHandleCreateApplicationSet(t *testing.T) {
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
					Name: "create_applicationset",
					Arguments: map[string]interface{}{
						"name":       "test-appset",
						"generators": `[{"list":{"elements":[{"cluster":"test"}]}}]`,
						"template":   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
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
			name: "missing required name",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_applicationset",
					Arguments: map[string]interface{}{
						"generators": `[{"list":{"elements":[{"cluster":"test"}]}}]`,
						"template":   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
			},
			wantError:     true,
			errorContains: "name is required",
		},
		{
			name: "missing required generators",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_applicationset",
					Arguments: map[string]interface{}{
						"name":     "test-appset",
						"template": `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
			},
			wantError:     true,
			errorContains: "generators is required",
		},
		{
			name: "missing required template",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_applicationset",
					Arguments: map[string]interface{}{
						"name":       "test-appset",
						"generators": `[{"list":{"elements":[{"cluster":"test"}]}}]`,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
			},
			wantError:     true,
			errorContains: "template is required",
		},
		{
			name: "invalid generators JSON",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_applicationset",
					Arguments: map[string]interface{}{
						"name":       "test-appset",
						"generators": `invalid json`,
						"template":   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
			},
			wantError:     true,
			errorContains: "Failed to parse generators",
		},
		{
			name: "invalid template JSON",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name: "create_applicationset",
					Arguments: map[string]interface{}{
						"name":       "test-appset",
						"generators": `[{"list":{"elements":[{"cluster":"test"}]}}]`,
						"template":   `invalid json`,
					},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "localhost:8080",
			},
			wantError:     true,
			errorContains: "Failed to parse template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleCreateApplicationSet(context.Background(), tt.request)

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

func TestCreateApplicationSetTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if CreateApplicationSetTool.Name != "create_applicationset" {
		t.Errorf("Expected tool name 'create_applicationset', got %s", CreateApplicationSetTool.Name)
	}

	// Verify tool has description
	if CreateApplicationSetTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if CreateApplicationSetTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", CreateApplicationSetTool.InputSchema.Type)
	}

	// Check required parameters
	requiredParams := CreateApplicationSetTool.InputSchema.Required
	expectedRequired := []string{"name", "generators", "template"}
	assert.ElementsMatch(t, expectedRequired, requiredParams, "Required parameters mismatch")

	// Check all parameters are defined
	properties := CreateApplicationSetTool.InputSchema.Properties
	expectedProperties := []string{
		"name", "namespace", "project", "generators", "template",
		"sync_policy", "strategy", "go_template", "upsert", "dry_run",
	}
	for _, prop := range expectedProperties {
		if _, ok := properties[prop]; !ok {
			t.Errorf("Expected property '%s' not found in schema", prop)
		}
	}
}

func TestCreateApplicationSetHandler(t *testing.T) {
	tests := []struct {
		name        string
		appSetName  string
		namespace   string
		project     string
		generators  string
		template    string
		syncPolicy  string
		strategy    string
		goTemplate  bool
		upsert      bool
		dryRun      bool
		setupMock   func(*mock.MockInterface)
		wantError   bool
		wantMessage string
	}{
		{
			name:       "successful creation",
			appSetName: "test-appset",
			namespace:  "argocd",
			project:    "default",
			generators: `[{"list":{"elements":[{"cluster":"test"}]}}]`,
			template:   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
			syncPolicy: "",
			strategy:   "",
			goTemplate: false,
			upsert:     false,
			dryRun:     false,
			setupMock: func(m *mock.MockInterface) {
				expectedAppSet := &v1alpha1.ApplicationSet{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ApplicationSet",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-appset",
						Namespace: "argocd",
					},
				}
				m.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), false, false).Return(expectedAppSet, nil)
			},
			wantError: false,
		},
		{
			name:       "successful creation with upsert",
			appSetName: "test-appset",
			namespace:  "argocd",
			project:    "my-project",
			generators: `[{"list":{"elements":[{"cluster":"test"}]}}]`,
			template:   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
			syncPolicy: "",
			strategy:   "",
			goTemplate: false,
			upsert:     true,
			dryRun:     false,
			setupMock: func(m *mock.MockInterface) {
				expectedAppSet := &v1alpha1.ApplicationSet{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ApplicationSet",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-appset",
						Namespace: "argocd",
						Labels: map[string]string{
							"argocd.argoproj.io/project": "my-project",
						},
					},
				}
				m.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), true, false).Return(expectedAppSet, nil)
			},
			wantError: false,
		},
		{
			name:       "successful dry run",
			appSetName: "test-appset",
			namespace:  "argocd",
			project:    "default",
			generators: `[{"list":{"elements":[{"cluster":"test"}]}}]`,
			template:   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
			syncPolicy: "",
			strategy:   "",
			goTemplate: false,
			upsert:     false,
			dryRun:     true,
			setupMock: func(m *mock.MockInterface) {
				expectedAppSet := &v1alpha1.ApplicationSet{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "argoproj.io/v1alpha1",
						Kind:       "ApplicationSet",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-appset",
						Namespace: "argocd",
					},
				}
				m.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), false, true).Return(expectedAppSet, nil)
			},
			wantError: false,
		},
		{
			name:       "with sync policy",
			appSetName: "test-appset",
			namespace:  "argocd",
			project:    "default",
			generators: `[{"list":{"elements":[{"cluster":"test"}]}}]`,
			template:   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
			syncPolicy: `{"preserveResourcesOnDeletion":true}`,
			strategy:   "",
			goTemplate: false,
			upsert:     false,
			dryRun:     false,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), false, false).DoAndReturn(
					func(ctx context.Context, appSet *v1alpha1.ApplicationSet, upsert, dryRun bool) (*v1alpha1.ApplicationSet, error) {
						// Verify sync policy was set
						if appSet.Spec.SyncPolicy == nil || !appSet.Spec.SyncPolicy.PreserveResourcesOnDeletion {
							t.Error("Expected sync policy to be set with PreserveResourcesOnDeletion=true")
						}
						return appSet, nil
					})
			},
			wantError: false,
		},
		{
			name:       "with strategy",
			appSetName: "test-appset",
			namespace:  "argocd",
			project:    "default",
			generators: `[{"list":{"elements":[{"cluster":"test"}]}}]`,
			template:   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
			syncPolicy: "",
			strategy:   `{"type":"RollingSync","rollingSync":{"steps":[{"matchExpressions":[{"key":"cluster","operator":"In","values":["test"]}]}]}}`,
			goTemplate: false,
			upsert:     false,
			dryRun:     false,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), false, false).DoAndReturn(
					func(ctx context.Context, appSet *v1alpha1.ApplicationSet, upsert, dryRun bool) (*v1alpha1.ApplicationSet, error) {
						// Verify strategy was set
						if appSet.Spec.Strategy == nil || appSet.Spec.Strategy.Type != "RollingSync" {
							t.Error("Expected strategy to be set with Type=RollingSync")
						}
						return appSet, nil
					})
			},
			wantError: false,
		},
		{
			name:       "creation fails",
			appSetName: "test-appset",
			namespace:  "argocd",
			project:    "default",
			generators: `[{"list":{"elements":[{"cluster":"test"}]}}]`,
			template:   `{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
			syncPolicy: "",
			strategy:   "",
			goTemplate: false,
			upsert:     false,
			dryRun:     false,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), false, false).Return(nil, assert.AnError)
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := createApplicationSetHandler(
				context.Background(),
				mockClient,
				tt.appSetName,
				tt.namespace,
				tt.project,
				tt.generators,
				tt.template,
				tt.syncPolicy,
				tt.strategy,
				tt.goTemplate,
				tt.upsert,
				tt.dryRun,
			)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
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

func TestCreateApplicationSetHandler_InvalidJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	tests := []struct {
		name       string
		syncPolicy string
		strategy   string
		wantError  string
	}{
		{
			name:       "invalid sync policy JSON",
			syncPolicy: `invalid json`,
			strategy:   "",
			wantError:  "Failed to parse sync_policy",
		},
		{
			name:       "invalid strategy JSON",
			syncPolicy: "",
			strategy:   `invalid json`,
			wantError:  "Failed to parse strategy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := createApplicationSetHandler(
				context.Background(),
				mockClient,
				"test-appset",
				"argocd",
				"default",
				`[{"list":{"elements":[{"cluster":"test"}]}}]`,
				`{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
				tt.syncPolicy,
				tt.strategy,
				false,
				false,
				false,
			)

			require.Nil(t, err)
			require.NotNil(t, result)
			assert.True(t, result.IsError)
			require.Len(t, result.Content, 1)
			textContent, ok := mcp.AsTextContent(result.Content[0])
			require.True(t, ok)
			assert.Contains(t, textContent.Text, tt.wantError)
		})
	}
}

func TestCreateApplicationSetHandler_ResponseFormatting(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := mock.NewMockInterface(ctrl)

	expectedAppSet := &v1alpha1.ApplicationSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "ApplicationSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-appset",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSetSpec{
			GoTemplate: false,
		},
	}

	mockClient.EXPECT().CreateApplicationSet(gomock.Any(), gomock.Any(), false, false).Return(expectedAppSet, nil)

	result, err := createApplicationSetHandler(
		context.Background(),
		mockClient,
		"test-appset",
		"argocd",
		"default",
		`[{"list":{"elements":[{"cluster":"test"}]}}]`,
		`{"metadata":{"name":"{{cluster}}-app"},"spec":{"source":{"repoURL":"https://github.com/test/repo","path":"apps","targetRevision":"HEAD"},"destination":{"server":"{{cluster}}","namespace":"default"},"project":"default"}}`,
		"",
		"",
		false,
		false,
		false,
	)

	require.Nil(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
	require.Len(t, result.Content, 1)

	textContent, ok := mcp.AsTextContent(result.Content[0])
	require.True(t, ok)

	// Verify the response is valid JSON
	var responseAppSet v1alpha1.ApplicationSet
	err = json.Unmarshal([]byte(textContent.Text), &responseAppSet)
	require.NoError(t, err)
	assert.Equal(t, "test-appset", responseAppSet.Name)
	assert.Equal(t, "argocd", responseAppSet.Namespace)
}
