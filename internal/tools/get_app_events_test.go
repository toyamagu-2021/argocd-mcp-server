package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	eventspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/events"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Test the tool schema
func TestGetAppEventsTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetAppEventsTool.Name != "get_application_events" {
		t.Errorf("Expected tool name 'get_application_events', got %s", GetAppEventsTool.Name)
	}

	assert.Contains(t, GetAppEventsTool.Description, "detailed=false")
	assert.Contains(t, GetAppEventsTool.Description, "50")
	assert.Contains(t, GetAppEventsTool.Description, "resource_name, resource_namespace, or resource_uid")

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
	expectedParams := []string{"name", "resource_namespace", "resource_name", "resource_uid", "app_namespace", "project", "detailed", "limit", "offset"}
	for _, param := range expectedParams {
		if _, ok := properties[param]; !ok {
			t.Errorf("Expected parameter '%s' to be defined", param)
		}
	}
}

// Test the handler logic with mocked client
func TestGetApplicationEventsHandler(t *testing.T) {
	// Mock event data
	mockEvents := &eventspkg.EventList{
		Items: []eventspkg.Event{
			{
				Metadata: metav1.ObjectMeta{
					Name:      "test-event-1",
					Namespace: "default",
				},
				InvolvedObject: eventspkg.ObjectReference{
					Kind:      "Pod",
					Name:      "test-pod",
					Namespace: "default",
				},
				Type:    "Warning",
				Reason:  "FailedScheduling",
				Message: "0/3 nodes are available: 3 Insufficient cpu.",
			},
			{
				Metadata: metav1.ObjectMeta{
					Name:      "test-event-2",
					Namespace: "default",
				},
				InvolvedObject: eventspkg.ObjectReference{
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
				m.EXPECT().GetApplicationEvents(gomock.Any(), client.EventsOptions{Name: "test-app"}).Return(mockEvents, nil)
			},
			wantError: false,
		},
		{
			name:              "successful get events - with filters",
			appName:           "test-app",
			resourceNamespace: "default",
			resourceName:      "test-pod",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationEvents(gomock.Any(), client.EventsOptions{Name: "test-app", ResourceNamespace: "default", ResourceName: "test-pod"}).Return(mockEvents, nil)
			},
			wantError: false,
		},
		{
			name:         "successful get events - with app namespace and project",
			appName:      "test-app",
			appNamespace: "argocd",
			project:      "default",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationEvents(gomock.Any(), client.EventsOptions{Name: "test-app", AppNamespace: "argocd", Project: "default"}).Return(mockEvents, nil)
			},
			wantError: false,
		},
		{
			name:    "application not found",
			appName: "non-existent-app",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplicationEvents(gomock.Any(), client.EventsOptions{Name: "non-existent-app"}).Return(nil, assert.AnError)
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

			result, err := getApplicationEventsHandler(context.Background(), mockClient, client.EventsOptions{
				Name:              tt.appName,
				ResourceNamespace: tt.resourceNamespace,
				ResourceName:      tt.resourceName,
				ResourceUID:       tt.resourceUID,
				AppNamespace:      tt.appNamespace,
				Project:           tt.project,
			}, false, 50, 0)

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

				require.Len(t, result.Content, 1)
				textContent, ok := mcp.AsTextContent(result.Content[0])
				require.True(t, ok)
				assert.NotContains(t, textContent.Text, "\n")
				assert.Contains(t, textContent.Text, "FailedScheduling")
				assert.Contains(t, textContent.Text, "test-pod")
				assert.Contains(t, textContent.Text, `"totalItems":2`)
				assert.NotContains(t, textContent.Text, `"metadata"`)
				assert.NotContains(t, textContent.Text, "test-event-1")
			}
		})
	}
}

func TestGetApplicationEventsSummaryPage(t *testing.T) {
	list := sampleEvents(40)
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationEvents(gomock.Any(), client.EventsOptions{Name: "guestbook"}).Return(list, nil).Times(3)

	summaryRes, err := getApplicationEventsHandler(context.Background(), m, client.EventsOptions{Name: "guestbook"}, false, 50, 0)
	require.NoError(t, err)
	summary := toolText(t, summaryRes)
	assert.NotContains(t, summary, `"metadata"`)
	assert.NotContains(t, summary, "managedFields")
	assert.NotContains(t, summary, "managed-fields-marker")
	assert.Contains(t, summary, eventMessageMarker)
	assert.Contains(t, summary, `"totalItems":40`)
	assert.Contains(t, summary, `"hasMore":false`)
	assert.Contains(t, summary, "2026-01-02T03:04:05Z")
	assert.Contains(t, summary, `"involvedName":"web"`)
	assert.Contains(t, summary, `"sourceComponent":"kubelet"`)
	assert.GreaterOrEqual(t, len(eventMessageBody), 400)

	pastRes, err := getApplicationEventsHandler(context.Background(), m, client.EventsOptions{Name: "guestbook"}, false, 50, 100)
	require.NoError(t, err)
	assert.Equal(t, `{"items":[],"totalItems":40,"hasMore":false,"notice":"offset 100 is past 40 items. Narrow with resource_name, resource_namespace, or resource_uid."}`, toolText(t, pastRes))

	detailedRes, err := getApplicationEventsHandler(context.Background(), m, client.EventsOptions{Name: "guestbook"}, true, 50, 0)
	require.NoError(t, err)
	detailed := toolText(t, detailedRes)
	assert.NotContains(t, detailed, "managedFields")
	assert.NotContains(t, detailed, lastAppliedAnnotation)
	assert.Contains(t, detailed, eventMessageMarker)
	assert.Contains(t, detailed, `"metadata"`)
}

func TestGetApplicationEventsSummarySize(t *testing.T) {
	list := sampleEvents(40)
	before, err := json.MarshalIndent(list, "", "  ")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationEvents(gomock.Any(), client.EventsOptions{Name: "guestbook"}).Return(list, nil)
	res, err := getApplicationEventsHandler(context.Background(), m, client.EventsOptions{Name: "guestbook"}, false, 50, 0)
	require.NoError(t, err)
	text := toolText(t, res)
	managedBytes := strings.Count(string(before), eventManagedPad) * len(eventManagedPad)
	t.Logf("before_bytes=%d after_bytes=%d managedfields_bytes=%d", len(before), len(text), managedBytes)
	assert.Less(t, len(text), len(before)/5)
	assert.Greater(t, managedBytes, len(text))
	assert.Contains(t, text, eventMessageMarker)
	assert.NotContains(t, text, "managed-fields-marker")
}

const eventMessageMarker = "event-message-marker:"

var eventMessageBody = eventMessageMarker + strings.Repeat("m", 400-len(eventMessageMarker))

var eventManagedPad = "managed-fields-marker" + strings.Repeat("F", 4096)

func sampleEvents(n int) *eventspkg.EventList {
	when := metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	items := make([]eventspkg.Event, n)
	for i := range items {
		items[i] = eventspkg.Event{
			Metadata: metav1.ObjectMeta{
				Name: "event-" + strconv.Itoa(i),
				ManagedFields: []metav1.ManagedFieldsEntry{{
					Manager:  "kubectl",
					FieldsV1: metav1.NewFieldsV1(`{"managed-fields-marker":"` + eventManagedPad + `"}`),
				}},
				Annotations: map[string]string{
					"keep-me":             "yes",
					lastAppliedAnnotation: "applied-blob",
				},
			},
			InvolvedObject: eventspkg.ObjectReference{Kind: "Pod", Namespace: "default", Name: "web"},
			Reason:         "BackOff",
			Message:        eventMessageBody,
			Source:         eventspkg.EventSource{Component: "kubelet"},
			FirstTimestamp: when,
			LastTimestamp:  when,
			Count:          3,
			Type:           "Warning",
		}
	}
	return &eventspkg.EventList{Items: items}
}
