package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test the tool handler with environment variables
func TestHandleGetApplicationEvents(t *testing.T) {
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
					Name: "get_application_events",
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
			name: "missing application name",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_application_events",
					Arguments: map[string]interface{}{},
				},
			},
			envVars: map[string]string{
				"ARGOCD_AUTH_TOKEN": "test-token",
				"ARGOCD_SERVER":     "argocd.example.com:443",
			},
			wantError:     true,
			errorContains: "Application name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			// Execute handler
			result, err := HandleGetApplicationEvents(context.Background(), tt.request)

			// Check expectations
			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.errorContains != "" && len(result.Content) > 0 {
					content, ok := mcp.AsTextContent(result.Content[0])
					require.True(t, ok)
					assert.Contains(t, content.Text, tt.errorContains)
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
func TestGetAppEventsTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetAppEventsTool.Name != "get_application_events" {
		t.Errorf("Expected tool name 'get_application_events', got %s", GetAppEventsTool.Name)
	}

	// Verify tool has description
	if GetAppEventsTool.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetAppEventsTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetAppEventsTool.InputSchema.Type)
	}

	// Check required parameters
	required := GetAppEventsTool.InputSchema.Required
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("Expected required parameter 'name', got %v", required)
	}

	// Check all parameters are defined
	properties := GetAppEventsTool.InputSchema.Properties
	expectedParams := []string{"name", "resource_namespace", "resource_name", "resource_uid", "app_namespace", "project"}
	for _, param := range expectedParams {
		if _, ok := properties[param]; !ok {
			t.Errorf("Expected parameter '%s' to be defined", param)
		}
	}
}

// Test the handler logic with mocked client
func TestGetApplicationEventsHandler(t *testing.T) {
	// Mock event data
	mockEvents := &v1.EventList{
		Items: []v1.Event{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-event-1",
					Namespace: "default",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:      "Pod",
					Name:      "test-pod",
					Namespace: "default",
				},
				Type:    "Warning",
				Reason:  "FailedScheduling",
				Message: "0/3 nodes are available: 3 Insufficient cpu.",
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-event-2",
					Namespace: "default",
				},
				InvolvedObject: v1.ObjectReference{
					Kind:      "Pod",
					Name:      "test-pod",
					Namespace: "default",
				},
				Type:    "Normal",
				Reason:  "Scheduled",
				Message: "Successfully assigned default/test-pod to node-1",
			},
		},
	}

	tests := []struct {
		name              string
		appName           string
		resourceNamespace string
		resourceName      string
		resourceUID       string
		appNamespace      string
		project           string
		setupMock         func(*mock.MockInterface)
		wantError         bool
		wantMessage       string
	}{
		{
			name:    "successful get events - minimal params",
			appName: "test-app",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationEvents(
					gomock.Any(),
					"test-app",
					"",
					"",
					"",
					"",
					"",
				).Return(mockEvents, nil)
			},
			wantError: false,
		},
		{
			name:              "successful get events - with filters",
			appName:           "test-app",
			resourceNamespace: "default",
			resourceName:      "test-pod",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationEvents(
					gomock.Any(),
					"test-app",
					"default",
					"test-pod",
					"",
					"",
					"",
				).Return(mockEvents, nil)
			},
			wantError: false,
		},
		{
			name:         "successful get events - with app namespace and project",
			appName:      "test-app",
			appNamespace: "argocd",
			project:      "default",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationEvents(
					gomock.Any(),
					"test-app",
					"",
					"",
					"",
					"argocd",
					"default",
				).Return(mockEvents, nil)
			},
			wantError: false,
		},
		{
			name:    "application not found",
			appName: "non-existent-app",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationEvents(
					gomock.Any(),
					"non-existent-app",
					"",
					"",
					"",
					"",
					"",
				).Return(nil, assert.AnError)
			},
			wantError:   true,
			wantMessage: "Failed to get application events",
		},
		{
			name:    "empty application name",
			appName: "",
			setupMock: func(m *mock.MockInterface) {
				// No mock expectation since validation should fail first
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

			result, err := getApplicationEventsHandler(
				context.Background(),
				mockClient,
				tt.appName,
				tt.resourceNamespace,
				tt.resourceName,
				tt.resourceUID,
				tt.appNamespace,
				tt.project,
			)

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

				// Verify the response contains events
				require.Len(t, result.Content, 1)
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)

				// Verify JSON structure
				var events v1.EventList
				err := json.Unmarshal([]byte(textContent.Text), &events)
				require.NoError(t, err)
				assert.Len(t, events.Items, 2)
				assert.Equal(t, "test-event-1", events.Items[0].Name)
				assert.Equal(t, "test-event-2", events.Items[1].Name)
			}
		})
	}
}
