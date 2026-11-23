package tools

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetAppTool_Schema(t *testing.T) {
	// Verify tool is properly defined
	if GetAppTool.Name != "get_application" {
		t.Errorf("Expected tool name 'get_application', got %s", GetAppTool.Name)
	}

	assert.Contains(t, GetAppTool.Description, "detailed=false")
	assert.Contains(t, GetAppTool.Description, "get_application_resource_tree")
	assert.Contains(t, GetAppTool.Description, "get_application_managed_resources")
	detailed, ok := GetAppTool.InputSchema.Properties["detailed"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, detailed["description"], "default")

	// Check input schema exists
	if GetAppTool.InputSchema.Type != "object" {
		t.Errorf("Expected schema type 'object', got %s", GetAppTool.InputSchema.Type)
	}

	// Check that we have properties defined
	if len(GetAppTool.InputSchema.Properties) == 0 {
		t.Error("Tool should have properties defined")
	}

	// Check that we have required fields defined
	if len(GetAppTool.InputSchema.Required) == 0 {
		t.Error("Tool should have required fields defined")
	}
}

func TestGetApplicationHandler(t *testing.T) {
	tests := []struct {
		name        string
		appName     string
		setupMock   func(*mock.MockInterface)
		detailed    bool
		wantError   bool
		wantMessage string
		checkApp    func(*testing.T, *v1alpha1.Application)
	}{
		{
			name:    "successful get application",
			appName: "test-app",
			setupMock: func(m *mock.MockInterface) {
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-app",
						Namespace: "argocd",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "default",
						Source: &v1alpha1.ApplicationSource{
							RepoURL:        "https://github.com/test/repo",
							TargetRevision: "HEAD",
							Path:           "manifests",
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "default",
						},
					},
					Status: v1alpha1.ApplicationStatus{
						Sync: v1alpha1.SyncStatus{
							Status: "Synced",
						},
						Health: v1alpha1.AppHealthStatus{
							Status: "Healthy",
						},
					},
				}
				m.EXPECT().GetApplication(gomock.Any(), "test-app", "").Return(app, nil)
			},
			detailed:  true,
			wantError: false,
			checkApp: func(t *testing.T, app *v1alpha1.Application) {
				assert.Equal(t, "test-app", app.Name)
				assert.Equal(t, "default", app.Spec.Project)
				assert.Equal(t, "https://github.com/test/repo", app.Spec.Source.RepoURL)
				assert.Equal(t, v1alpha1.SyncStatusCodeSynced, app.Status.Sync.Status)
				assert.Equal(t, health.HealthStatusCode("Healthy"), app.Status.Health.Status)
			},
		},
		{
			name:        "empty application name",
			appName:     "",
			setupMock:   func(m *mock.MockInterface) {},
			wantError:   false,
			wantMessage: "Application name is required",
		},
		{
			name:    "application not found",
			appName: "nonexistent-app",
			setupMock: func(m *mock.MockInterface) {
				m.EXPECT().GetApplication(gomock.Any(), "nonexistent-app", "").Return(nil, assert.AnError)
			},
			wantError:   false,
			wantMessage: "Failed to get application",
		},
		{
			name:    "get application with multi-source",
			appName: "multi-source-app",
			setupMock: func(m *mock.MockInterface) {
				app := &v1alpha1.Application{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "multi-source-app",
						Namespace: "argocd",
					},
					Spec: v1alpha1.ApplicationSpec{
						Project: "production",
						Sources: []v1alpha1.ApplicationSource{
							{
								RepoURL:        "https://github.com/test/repo1",
								TargetRevision: "v1.0.0",
								Path:           "app",
							},
							{
								RepoURL:        "https://github.com/test/repo2",
								TargetRevision: "main",
								Path:           "config",
							},
						},
						Destination: v1alpha1.ApplicationDestination{
							Server:    "https://prod-cluster.example.com",
							Namespace: "production",
						},
					},
				}
				m.EXPECT().GetApplication(gomock.Any(), "multi-source-app", "").Return(app, nil)
			},
			detailed:  true,
			wantError: false,
			checkApp: func(t *testing.T, app *v1alpha1.Application) {
				assert.Equal(t, "multi-source-app", app.Name)
				assert.Equal(t, "production", app.Spec.Project)
				assert.Len(t, app.Spec.Sources, 2)
				assert.Equal(t, "https://github.com/test/repo1", app.Spec.Sources[0].RepoURL)
				assert.Equal(t, "v1.0.0", app.Spec.Sources[0].TargetRevision)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mock.NewMockInterface(ctrl)
			tt.setupMock(mockClient)

			ctx := context.Background()
			result, err := getApplicationHandler(ctx, mockClient, tt.appName, "", tt.detailed)

			if tt.wantError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Check the result content
			require.Len(t, result.Content, 1)
			textContent, ok := mcp.AsTextContent(result.Content[0])
			require.True(t, ok)

			if tt.wantMessage != "" {
				// Expecting an error message
				assert.Contains(t, textContent.Text, tt.wantMessage)
			} else if tt.checkApp != nil {
				// Expecting JSON with application details
				var app v1alpha1.Application
				err := json.Unmarshal([]byte(textContent.Text), &app)
				require.NoError(t, err)
				tt.checkApp(t, &app)
			}
		})
	}
}

func TestGetApplicationSummaryAndDetailed(t *testing.T) {
	app := sampleFatApplication()
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplication(gomock.Any(), "guestbook", "").Return(app, nil).Times(2)

	summaryRes, err := getApplicationHandler(context.Background(), m, "guestbook", "", false)
	require.NoError(t, err)
	require.False(t, summaryRes.IsError)
	summaryText, ok := mcp.AsTextContent(summaryRes.Content[0])
	require.True(t, ok)
	assert.NotContains(t, summaryText.Text, "\n")
	for _, forbidden := range []string{"\"spec\"", "\"resources\"", "managedFields", "helm-values-marker", "param-should-not-leak", "sources-helm-marker", "history-rev", "status-resource-0", lastAppliedAnnotation} {
		assert.NotContains(t, summaryText.Text, forbidden)
	}
	assert.Contains(t, summaryText.Text, `"historyCount":10`)
	assert.Contains(t, summaryText.Text, `"syncRevision":"abc123"`)
	assert.Contains(t, summaryText.Text, "https://github.com/example/multi")
	assert.Contains(t, summaryText.Text, "condition-message")
	assert.Contains(t, summaryText.Text, `"Healthy":20`)
	assert.Contains(t, summaryText.Text, `"Degraded":10`)
	assert.Contains(t, summaryText.Text, `"Synced":20`)
	assert.Contains(t, summaryText.Text, `"OutOfSync":10`)

	var summary ApplicationSummary
	require.NoError(t, json.Unmarshal([]byte(summaryText.Text), &summary))
	assert.Equal(t, 10, summary.HistoryCount)
	assert.Equal(t, []string{"rev-a", "rev-b"}, summary.SyncRevisions)
	assert.Equal(t, "needs a look", summary.HealthMessage)
	require.Len(t, summary.Sources, 1)
	assert.Equal(t, "https://github.com/example/multi", summary.Sources[0].RepoURL)
	assert.Empty(t, summary.Sources[0].Chart)

	detailedRes, err := getApplicationHandler(context.Background(), m, "guestbook", "", true)
	require.NoError(t, err)
	require.False(t, detailedRes.IsError)
	detailedText, ok := mcp.AsTextContent(detailedRes.Content[0])
	require.True(t, ok)
	assert.NotContains(t, detailedText.Text, "\n")
	assert.Contains(t, detailedText.Text, `"spec"`)
	assert.Contains(t, detailedText.Text, "helm-values-marker")
	assert.Contains(t, detailedText.Text, "param-should-not-leak")
	assert.NotContains(t, detailedText.Text, "managedFields")
	assert.NotContains(t, detailedText.Text, lastAppliedAnnotation)
	assert.Contains(t, detailedText.Text, "keep-me")
}

func TestGetApplicationSummarySize(t *testing.T) {
	app := sampleFatApplication()
	before, err := json.MarshalIndent(app, "", "  ")
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplication(gomock.Any(), "guestbook", "").Return(app, nil)
	res, err := getApplicationHandler(context.Background(), m, "guestbook", "", false)
	require.NoError(t, err)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	after := len(text.Text)
	t.Logf("before_bytes=%d after_bytes=%d", len(before), after)
	assert.Less(t, after, len(before)/5)
}

func TestListApplicationSummaryOmitsGetFields(t *testing.T) {
	app := sampleFatApplication()
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().ListApplications(gomock.Any(), gomock.Any()).Return(&v1alpha1.ApplicationList{Items: []v1alpha1.Application{*app}}, nil)

	res, err := listApplicationsHandler(context.Background(), m, "", "", "", "", false, false, "json", nil, "")
	require.NoError(t, err)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Contains(t, text.Text, "guestbook")
	for _, key := range []string{"sources", "syncRevision", "syncRevisions", "healthMessage", "historyCount", "healthCounts", "syncCounts", "conditions"} {
		assert.NotContains(t, text.Text, `"`+key+`"`)
	}
	assert.NotContains(t, text.Text, "helm-values-marker")
	assert.NotContains(t, text.Text, "history-rev")
}

func sampleFatApplication() *v1alpha1.Application {
	values := "helm-values-marker:" + strings.Repeat("v", 2048-len("helm-values-marker:"))
	resources := make([]v1alpha1.ResourceStatus, 30)
	for i := range resources {
		status := v1alpha1.SyncStatusCodeSynced
		hs := health.HealthStatusHealthy
		if i >= 20 {
			status = v1alpha1.SyncStatusCodeOutOfSync
			hs = health.HealthStatusDegraded
		}
		resources[i] = v1alpha1.ResourceStatus{
			Kind:      "Deployment",
			Name:      "status-resource-" + strconv.Itoa(i),
			Namespace: "default",
			Status:    status,
			Health:    &v1alpha1.HealthStatus{Status: hs},
		}
	}
	history := make([]v1alpha1.RevisionHistory, 10)
	for i := range history {
		history[i] = v1alpha1.RevisionHistory{ID: int64(i), Revision: "history-rev-" + strconv.Itoa(i)}
	}
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "guestbook",
			Namespace: "argocd",
			ManagedFields: []metav1.ManagedFieldsEntry{{
				Manager: "kubectl",
			}},
			Annotations: map[string]string{
				"keep-me":             "yes",
				lastAppliedAnnotation: strings.Repeat("a", 128),
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/example/repo",
				Path:           "guestbook",
				TargetRevision: "HEAD",
				Helm: &v1alpha1.ApplicationSourceHelm{
					Values: values,
					Parameters: []v1alpha1.HelmParameter{{
						Name:  "image.tag",
						Value: "param-should-not-leak",
					}},
				},
			},
			Sources: v1alpha1.ApplicationSources{{
				RepoURL:        "https://github.com/example/multi",
				Path:           "extra",
				TargetRevision: "v1",
				Helm:           &v1alpha1.ApplicationSourceHelm{Values: "sources-helm-marker"},
			}},
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
		},
		Status: v1alpha1.ApplicationStatus{
			Sync: v1alpha1.SyncStatus{
				Status:    v1alpha1.SyncStatusCodeSynced,
				Revision:  "abc123",
				Revisions: []string{"rev-a", "rev-b"},
			},
			Health: v1alpha1.AppHealthStatus{
				Status:  health.HealthStatusHealthy,
				Message: "needs a look",
			},
			Resources: resources,
			History:   history,
			Conditions: []v1alpha1.ApplicationCondition{{
				Type:    "InvalidSpecError",
				Message: "condition-message",
			}},
		},
	}
}
