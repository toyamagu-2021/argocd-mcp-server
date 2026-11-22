package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

// Test the tool handler with environment variables
// Test the tool schema
func TestGetApplicationLogsToolDefinition_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetApplicationLogsToolDefinition.Name != "get_application_logs" {
		t.Errorf("Expected tool name 'get_application_logs', got %s", GetApplicationLogsToolDefinition.Name)
	}

	// Verify tool has description
	if GetApplicationLogsToolDefinition.Description == "" {
		t.Error("Tool description should not be empty")
	}

	// Check input schema exists
	if GetApplicationLogsToolDefinition.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetApplicationLogsToolDefinition.InputSchema.Type)
	}

	// Check required parameters
	required := GetApplicationLogsToolDefinition.InputSchema.Required
	if len(required) != 1 || required[0] != "name" {
		t.Errorf("Expected required parameter 'name', got %v", required)
	}
}

// Mock log stream for testing
type mockLogStream struct {
	entries []*applicationpkg.LogEntry
	index   int
}

func (m *mockLogStream) Recv() (*applicationpkg.LogEntry, error) {
	if m.index >= len(m.entries) {
		content := ""
		last := true
		return &applicationpkg.LogEntry{
			Content: &content,
			Last:    &last,
		}, nil
	}
	entry := m.entries[m.index]
	m.index++
	return entry, nil
}

// Test the handler logic with mocked client
func TestGetApplicationLogsHandler(t *testing.T) {
	tests := []struct {
		name         string
		appName      string
		podName      string
		container    string
		namespace    string
		resourceName string
		kind         string
		group        string
		tailLines    int64
		sinceSeconds *int64
		follow       bool
		previous     bool
		filter       string
		appNamespace string
		project      string
		setupMock    func(*mock.MockInterface)
		wantError    bool
		wantLogCount int
		wantContains string
	}{
		{
			name:      "successful log retrieval",
			appName:   "test-app",
			podName:   "test-pod",
			container: "test-container",
			tailLines: 50,
			setupMock: func(m *mock.MockInterface) {
				content1 := "Log line 1"
				content2 := "Log line 2"
				timestamp1 := "2024-01-01T00:00:00Z"
				timestamp2 := "2024-01-01T00:00:01Z"
				podName := "test-pod"
				last := false

				logStream := &mockLogStream{
					entries: []*applicationpkg.LogEntry{
						{
							Content:      &content1,
							TimeStampStr: &timestamp1,
							PodName:      &podName,
							Last:         &last,
						},
						{
							Content:      &content2,
							TimeStampStr: &timestamp2,
							PodName:      &podName,
							Last:         &last,
						},
					},
				}
				m.EXPECT().GetApplicationLogs(gomock.Any(), client.LogsOptions{Name: "test-app", PodName: "test-pod", Container: "test-container", TailLines: int64(50)}).Return(logStream, nil)
			},
			wantError:    false,
			wantLogCount: 2,
			wantContains: "Log line 1",
		},
		{
			name:      "log retrieval fails",
			appName:   "test-app",
			podName:   "test-pod",
			tailLines: 100,
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationLogs(gomock.Any(), client.LogsOptions{Name: "test-app", PodName: "test-pod", TailLines: int64(100)}).Return(nil, fmt.Errorf("failed to get logs"))
			},
			wantError: true,
		},
		{
			name:         "with resource details",
			appName:      "test-app",
			resourceName: "test-deployment",
			kind:         "Deployment",
			group:        "apps",
			tailLines:    20,
			setupMock: func(m *mock.MockInterface) {
				content := "Deployment log"
				podName := "test-deployment-xyz"
				last := false

				logStream := &mockLogStream{
					entries: []*applicationpkg.LogEntry{
						{
							Content: &content,
							PodName: &podName,
							Last:    &last,
						},
					},
				}
				m.EXPECT().GetApplicationLogs(gomock.Any(), client.LogsOptions{Name: "test-app", ResourceName: "test-deployment", Kind: "Deployment", Group: "apps", TailLines: int64(20)}).Return(logStream, nil)
			},
			wantError:    false,
			wantLogCount: 1,
			wantContains: "Deployment log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			result, err := getApplicationLogsHandler(context.Background(), mockClient, client.LogsOptions{
				Name:         tt.appName,
				PodName:      tt.podName,
				Container:    tt.container,
				Namespace:    tt.namespace,
				ResourceName: tt.resourceName,
				Kind:         tt.kind,
				Group:        tt.group,
				TailLines:    tt.tailLines,
				SinceSeconds: tt.sinceSeconds,
				Follow:       tt.follow,
				Previous:     tt.previous,
				Filter:       tt.filter,
				AppNamespace: tt.appNamespace,
				Project:      tt.project,
			})

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				// Parse the response
				require.Len(t, result.Content, 1)
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)

				var response map[string]interface{}
				err := json.Unmarshal([]byte(textContent.Text), &response)
				require.NoError(t, err)

				// Check log count
				if tt.wantLogCount > 0 {
					logs, ok := response["logs"].([]interface{})
					require.True(t, ok)
					assert.Len(t, logs, tt.wantLogCount)
				}

				// Check content
				if tt.wantContains != "" {
					assert.Contains(t, textContent.Text, tt.wantContains)
				}
			}
		})
	}
}

// Test parameter extraction with CallToolRequest
func TestGetApplicationLogs_ParameterExtraction(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		validate func(t *testing.T, request mcp.CallToolRequest)
	}{
		{
			name: "numeric parameters",
			args: map[string]interface{}{
				"name":          "test-app",
				"tail_lines":    200.0,
				"since_seconds": 3600.0,
			},
			validate: func(t *testing.T, request mcp.CallToolRequest) {
				// Test that the request can extract numeric parameters correctly
				assert.Equal(t, "test-app", request.GetString("name", ""))
				assert.Equal(t, 200, request.GetInt("tail_lines", 0))
				assert.Equal(t, 3600, request.GetInt("since_seconds", 0))
			},
		},
		{
			name: "boolean parameters",
			args: map[string]interface{}{
				"name":     "test-app",
				"follow":   true,
				"previous": true,
			},
			validate: func(t *testing.T, request mcp.CallToolRequest) {
				// Test that the request can extract boolean parameters correctly
				assert.Equal(t, "test-app", request.GetString("name", ""))
				assert.True(t, request.GetBool("follow", false))
				assert.True(t, request.GetBool("previous", false))
			},
		},
		{
			name: "default values",
			args: map[string]interface{}{
				"name": "test-app",
			},
			validate: func(t *testing.T, request mcp.CallToolRequest) {
				// Test that default values are returned when parameters are not provided
				assert.Equal(t, "test-app", request.GetString("name", ""))
				assert.Equal(t, 100, request.GetInt("tail_lines", 100))
				assert.Equal(t, 0, request.GetInt("since_seconds", 0))
				assert.False(t, request.GetBool("follow", false))
				assert.False(t, request.GetBool("previous", false))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Name:      "get_application_logs",
					Arguments: tt.args,
				},
			}

			// Validate parameter extraction
			tt.validate(t, request)
		})
	}
}
