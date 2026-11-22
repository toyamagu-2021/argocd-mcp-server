package tools

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

// Test the tool handler with environment variables
// Test the tool schema
func TestTerminateOperationTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if TerminateOperationTool.Name != "terminate_operation" {
		t.Errorf("Expected tool name 'terminate_operation', got %s", TerminateOperationTool.Name)
	}

	// Verify tool has description
	if TerminateOperationTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if TerminateOperationTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", TerminateOperationTool.InputSchema.Type)
	}

	// Check required parameters
	required := TerminateOperationTool.InputSchema.Required
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("Expected 'name' to be required, got %v", required)
	}

	// Check properties exist
	props := TerminateOperationTool.InputSchema.Properties
	if props == nil {
		t.Fatal("Expected properties to be defined")
	}

	// Check name property
	if _, ok := props["name"]; !ok {
		t.Error("Expected 'name' property to be defined")
	}

	// Check optional properties
	if _, ok := props["app_namespace"]; !ok {
		t.Error("Expected 'app_namespace' property to be defined")
	}

	if _, ok := props["project"]; !ok {
		t.Error("Expected 'project' property to be defined")
	}
}

// Test the handler logic with mocked client
func TestTerminateOperationHandler(t *testing.T) {
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
			name:    "successful operation termination",
			appName: "test-app",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().TerminateOperation(
					gomock.Any(),
					"test-app",
					"",
					"",
				).Return(nil)
			},
			wantError:   false,
			wantMessage: "Successfully terminated operation for application 'test-app'",
		},
		{
			name:         "successful operation termination with namespace",
			appName:      "test-app",
			appNamespace: "argocd",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().TerminateOperation(
					gomock.Any(),
					"test-app",
					"argocd",
					"",
				).Return(nil)
			},
			wantError:   false,
			wantMessage: "Successfully terminated operation for application 'test-app' in namespace 'argocd'",
		},
		{
			name:    "successful operation termination with project",
			appName: "test-app",
			project: "default",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().TerminateOperation(
					gomock.Any(),
					"test-app",
					"",
					"default",
				).Return(nil)
			},
			wantError:   false,
			wantMessage: "Successfully terminated operation for application 'test-app' (project: default)",
		},
		{
			name:         "successful operation termination with namespace and project",
			appName:      "test-app",
			appNamespace: "argocd",
			project:      "default",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().TerminateOperation(
					gomock.Any(),
					"test-app",
					"argocd",
					"default",
				).Return(nil)
			},
			wantError:   false,
			wantMessage: "Successfully terminated operation for application 'test-app' in namespace 'argocd' (project: default)",
		},
		{
			name:    "operation termination fails",
			appName: "test-app",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().TerminateOperation(
					gomock.Any(),
					"test-app",
					"",
					"",
				).Return(assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to terminate operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := terminateOperationHandler(
				context.Background(),
				mockClient,
				tt.appName,
				tt.appNamespace,
				tt.project,
			)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.wantMessage != "" && len(result.Content) > 0 {
					textContent, ok := mcp.AsTextContent(result.Content[0])
					if ok {
						assert.Contains(t, textContent.Text, tt.wantMessage)
					}
				}
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				if tt.wantMessage != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Equal(t, tt.wantMessage, textContent.Text)
				}
			}
		})
	}
}
