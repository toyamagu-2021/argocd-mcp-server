package tools

import (
	"context"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

func TestDeleteApplicationSetTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if DeleteApplicationSetTool.Name != "delete_applicationset" {
		t.Errorf("Expected tool name 'delete_applicationset', got %s", DeleteApplicationSetTool.Name)
	}

	// Verify tool has description
	if DeleteApplicationSetTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Verify the description contains warning about destructive operation
	if DeleteApplicationSetTool.Description != "" {
		assert.Contains(t, DeleteApplicationSetTool.Description, "destructive")
		t.Logf("Tool description: %s", DeleteApplicationSetTool.Description)
	}

	// Check input schema exists
	if DeleteApplicationSetTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", DeleteApplicationSetTool.InputSchema.Type)
	}

	// Check required parameters
	requiredParams := DeleteApplicationSetTool.InputSchema.Required
	assert.Contains(t, requiredParams, "name", "name should be a required parameter")

	// Check that appsetNamespace is optional (not in required list)
	assert.NotContains(t, requiredParams, "appsetNamespace", "appsetNamespace should be optional")
}

func TestDeleteApplicationSetHandler(t *testing.T) {
	tests := []struct {
		name            string
		appSetName      string
		appSetNamespace string
		setupMock       func(*mock.MockInterface)
		wantError       bool
		wantMessage     string
		errorContains   string
	}{
		{
			name:            "successful delete without namespace",
			appSetName:      "test-appset",
			appSetNamespace: "",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().DeleteApplicationSet(gomock.Any(), "test-appset", "").Return(nil)
			},
			wantError:   false,
			wantMessage: "ApplicationSet 'test-appset' deleted successfully",
		},
		{
			name:            "successful delete with namespace",
			appSetName:      "test-appset",
			appSetNamespace: "argocd",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().DeleteApplicationSet(gomock.Any(), "test-appset", "argocd").Return(nil)
			},
			wantError:   false,
			wantMessage: "ApplicationSet 'test-appset' in namespace 'argocd' deleted successfully",
		},
		{
			name:            "empty applicationset name",
			appSetName:      "",
			appSetNamespace: "",
			setupMock:       func(m *mock.MockInterface) {},
			wantError:       true,
			errorContains:   "ApplicationSet name is required",
		},
		{
			name:            "delete fails",
			appSetName:      "test-appset",
			appSetNamespace: "",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().DeleteApplicationSet(gomock.Any(), "test-appset", "").Return(fmt.Errorf("not found"))
			},
			wantError:     true,
			errorContains: "Failed to delete ApplicationSet: not found",
		},
		{
			name:            "delete fails with namespace",
			appSetName:      "test-appset",
			appSetNamespace: "custom-ns",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().DeleteApplicationSet(gomock.Any(), "test-appset", "custom-ns").Return(fmt.Errorf("permission denied"))
			},
			wantError:     true,
			errorContains: "Failed to delete ApplicationSet: permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := deleteApplicationSetHandler(context.Background(), mockClient, tt.appSetName, tt.appSetNamespace)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" {
					require.Len(t, result.Content, 1)
					textContent, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, textContent.Text, tt.errorContains)
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
					// Also verify the warning message about managed applications is included
					assert.Contains(t, textContent.Text, "All applications managed by this ApplicationSet will be deleted")
				}
			}
		})
	}
}
