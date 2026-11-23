package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

// Test the tool schema
func TestGetApplicationLogsToolDefinition_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetApplicationLogsToolDefinition.Name != "get_application_logs" {
		t.Errorf("Expected tool name 'get_application_logs', got %s", GetApplicationLogsToolDefinition.Name)
	}

	assert.Contains(t, GetApplicationLogsToolDefinition.Description, "detailed=false")
	assert.Contains(t, GetApplicationLogsToolDefinition.Description, "20")

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
			}, false)

			if tt.wantError {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				require.Nil(t, err)
				require.NotNil(t, result)
				assert.False(t, result.IsError)

				require.Len(t, result.Content, 1)
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)
				assert.NotContains(t, textContent.Text, "{")
				if tt.wantLogCount > 0 {
					assert.Contains(t, textContent.Text, fmt.Sprintf("lines=%d\n", tt.wantLogCount))
				}
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
				assert.Equal(t, defaultTailLines, request.GetInt("tail_lines", defaultTailLines))
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

			tt.validate(t, request)
		})
	}
}

func TestGetApplicationLogs_DefaultTailAndText(t *testing.T) {
	line := strings.Repeat("x", 80)
	beforeLogs := make([]logEntry, 100)
	for i := range beforeLogs {
		beforeLogs[i] = logEntry{Content: line}
	}
	before, err := json.MarshalIndent(logResponse{Application: "guestbook", TotalLines: 100, Logs: beforeLogs}, "", "  ")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationLogs(gomock.Any(), client.LogsOptions{Name: "guestbook", TailLines: int64(defaultTailLines)}).
		Return(logStreamOf(line, defaultTailLines), nil)
	res := callTool(t, newTestServer(t, Deps{Client: m}), "get_application_logs", map[string]interface{}{"name": "guestbook"})
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.False(t, res.IsError)
	assert.NotContains(t, text.Text, "{")
	assert.Contains(t, text.Text, "lines=20\n")
	assert.Contains(t, text.Text, tailNotice(defaultTailLines, defaultTailLines))
	t.Logf("before_bytes=%d after_bytes=%d", len(before), len(text.Text))
	assert.Less(t, len(text.Text), len(before)/5)

	m.EXPECT().GetApplicationLogs(gomock.Any(), client.LogsOptions{Name: "guestbook", TailLines: int64(defaultTailLines)}).
		Return(logStreamOf(line, 3), nil)
	short := callTool(t, newTestServer(t, Deps{Client: m}), "get_application_logs", map[string]interface{}{"name": "guestbook"})
	shortText, ok := mcp.AsTextContent(short.Content[0])
	require.True(t, ok)
	assert.NotContains(t, shortText.Text, "Showing the last")
	assert.Contains(t, shortText.Text, "lines=3\n")

	m.EXPECT().GetApplicationLogs(gomock.Any(), client.LogsOptions{Name: "guestbook", TailLines: int64(defaultTailLines)}).
		Return(logStreamOf(line, defaultTailLines), nil)
	detailed := callTool(t, newTestServer(t, Deps{Client: m}), "get_application_logs", map[string]interface{}{"name": "guestbook", "detailed": true})
	detailedText, ok := mcp.AsTextContent(detailed.Content[0])
	require.True(t, ok)
	assert.NotContains(t, detailedText.Text, "\n")
	assert.Contains(t, detailedText.Text, `"logs"`)
	assert.Contains(t, detailedText.Text, `"notice"`)
}

func logStreamOf(content string, n int) *mockLogStream {
	entries := make([]*applicationpkg.LogEntry, n)
	last := false
	for i := range entries {
		line := content
		entries[i] = &applicationpkg.LogEntry{Content: &line, Last: &last}
	}
	return &mockLogStream{entries: entries}
}
