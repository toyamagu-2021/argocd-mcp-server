package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client"
	"github.com/toyamagu-2021/argocd-mcp-server/internal/argocd/client/mock"
	"go.uber.org/mock/gomock"
)

func TestGetApplicationManagedResources_Schema(t *testing.T) {
	assert.Equal(t, "get_application_managed_resources", GetApplicationManagedResourcesTool.Name)
	assert.Contains(t, GetApplicationManagedResourcesTool.Description, "ManagedResources")
	assert.Equal(t, []string{"name"}, GetApplicationManagedResourcesTool.InputSchema.Required)
	require.NotNil(t, GetApplicationManagedResourcesTool.Annotations.ReadOnlyHint)
	assert.True(t, *GetApplicationManagedResourcesTool.Annotations.ReadOnlyHint)
	require.NotNil(t, GetApplicationManagedResourcesTool.Annotations.DestructiveHint)
	assert.False(t, *GetApplicationManagedResourcesTool.Annotations.DestructiveHint)

	props := GetApplicationManagedResourcesTool.InputSchema.Properties
	for _, key := range []string{"name", "resource_namespace", "resource_name", "version", "group", "kind", "app_namespace", "project", "detailed", "limit", "offset"} {
		prop, ok := props[key].(map[string]any)
		require.True(t, ok, key)
		assert.NotEmpty(t, prop["description"], key)
	}
	version, _ := props["version"].(map[string]any)
	assert.Contains(t, version["description"], "not a git revision")
	detailed, ok := props["detailed"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, detailed["description"], "default")
	assert.Contains(t, GetApplicationManagedResourcesTool.Description, "detailed=false")
}

func TestGetApplicationManagedResourcesHandler(t *testing.T) {
	opts := client.ManagedResourcesOptions{
		Name: "app", ResourceNamespace: "prod", ResourceName: "web",
		Version: "v1", Group: "apps", Kind: "Deployment",
		AppNamespace: "team-a", Project: "proj",
	}
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationManagedResources(gomock.Any(), opts).Return(&applicationpkg.ManagedResourcesResponse{
		Items: []*v1alpha1.ResourceDiff{{Kind: "Deployment", Name: "web"}},
	}, nil)

	res, err := getApplicationManagedResourcesHandler(context.Background(), m, opts, false, 50, 0)
	require.NoError(t, err)
	assert.False(t, res.IsError)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Contains(t, text.Text, "web")

	res, err = getApplicationManagedResourcesHandler(context.Background(), m, client.ManagedResourcesOptions{}, false, 50, 0)
	require.NoError(t, err)
	assert.True(t, res.IsError)
}

func TestGetApplicationManagedResources_SummaryOmitsState(t *testing.T) {
	full := sampleManagedResources(1)
	ctrl := gomock.NewController(t)
	m := mock.NewMockInterface(ctrl)
	m.EXPECT().GetApplicationManagedResources(gomock.Any(), client.ManagedResourcesOptions{Name: "app"}).
		Return(full, nil).Times(2)

	summaryRes, err := getApplicationManagedResourcesHandler(context.Background(), m, client.ManagedResourcesOptions{Name: "app"}, false, 50, 0)
	require.NoError(t, err)
	require.False(t, summaryRes.IsError)
	summaryText, ok := mcp.AsTextContent(summaryRes.Content[0])
	require.True(t, ok)
	for _, forbidden := range []string{"targetState", "liveState", "predictedLiveState", "normalizedLiveState", "\"diff\""} {
		assert.NotContains(t, summaryText.Text, forbidden)
	}
	assert.Contains(t, summaryText.Text, `"modified"`)
	assert.Contains(t, summaryText.Text, "guestbook-ui")

	detailedRes, err := getApplicationManagedResourcesHandler(context.Background(), m, client.ManagedResourcesOptions{Name: "app"}, true, 50, 0)
	require.NoError(t, err)
	detailedText, ok := mcp.AsTextContent(detailedRes.Content[0])
	require.True(t, ok)
	assert.Contains(t, detailedText.Text, "targetState")
	assert.Contains(t, detailedText.Text, "liveState")
	assert.Contains(t, detailedText.Text, "predictedLiveState")
	assert.Contains(t, detailedText.Text, "normalizedLiveState")
	assert.Greater(t, len(detailedText.Text), len(summaryText.Text))

	filtered := client.ManagedResourcesOptions{Name: "app", Kind: "Deployment"}
	m.EXPECT().GetApplicationManagedResources(gomock.Any(), filtered).Return(full, nil)
	filteredRes, err := getApplicationManagedResourcesHandler(context.Background(), m, filtered, false, 50, 0)
	require.NoError(t, err)
	filteredText, ok := mcp.AsTextContent(filteredRes.Content[0])
	require.True(t, ok)
	assert.Contains(t, filteredText.Text, "targetState")
}

func TestManagedResourcesSummarySize(t *testing.T) {
	full := sampleManagedResources(29)
	before, err := json.MarshalIndent(full, "", "  ")
	require.NoError(t, err)
	summary := formatManagedResources(full, client.ManagedResourcesOptions{Name: "app"}, false, 50, 0)
	summaryJSON, err := json.Marshal(summary)
	require.NoError(t, err)
	detailed := formatManagedResources(full, client.ManagedResourcesOptions{Name: "app"}, true, 50, 0)
	detailedJSON, err := json.Marshal(detailed)
	require.NoError(t, err)
	t.Logf("indented_full_bytes=%d compact_summary_bytes=%d compact_detailed_bytes=%d", len(before), len(summaryJSON), len(detailedJSON))
	assert.Greater(t, len(before), 80_000)
	assert.Less(t, len(summaryJSON), len(before)/10)
	assert.Less(t, len(detailedJSON), len(before))
	assert.NotContains(t, string(summaryJSON), "targetState")
	assert.NotContains(t, string(detailedJSON), "managedFields")
	assert.NotContains(t, string(detailedJSON), lastAppliedAnnotation)
	assert.Contains(t, string(summaryJSON), `"totalItems":29`)
	assert.Contains(t, string(summaryJSON), `"hasMore":false`)
}

func TestManagedResourcesPagingAndStrip(t *testing.T) {
	full := sampleManagedResources(3)
	page := formatManagedResources(full, client.ManagedResourcesOptions{Name: "app"}, false, 2, 1)
	body, err := json.Marshal(page)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"totalItems":3`)
	assert.Contains(t, string(body), `"hasMore":false`)
	assert.Contains(t, string(body), "Showing items 1-2 of 3")
	assert.NotContains(t, string(body), "\n")

	stripped := stripManifestNoise(full.Items[0].LiveState)
	assert.NotContains(t, stripped, "managedFields")
	assert.NotContains(t, stripped, lastAppliedAnnotation)
	assert.Contains(t, stripped, "keep-me")

	past := formatManagedResources(full, client.ManagedResourcesOptions{Name: "app"}, false, 50, 100)
	pastBody, err := json.Marshal(past)
	require.NoError(t, err)
	assert.Equal(t, `{"items":[],"totalItems":3,"hasMore":false,"notice":"offset 100 is past 3 items. Narrow with kind, resource_name, resource_namespace, group, or version."}`, string(pastBody))
}

func TestManagedResourcesDropsNilDiffs(t *testing.T) {
	full := sampleManagedResources(2)
	full.Items = []*v1alpha1.ResourceDiff{nil, full.Items[0], nil, full.Items[1]}

	summary := formatManagedResources(full, client.ManagedResourcesOptions{Name: "app"}, false, 50, 0)
	body, err := json.Marshal(summary)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"totalItems":2`)
	assert.NotContains(t, string(body), "null")

	detailed := formatManagedResources(full, client.ManagedResourcesOptions{Name: "app"}, true, 50, 0)
	body, err = json.Marshal(detailed)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"totalItems":2`)
	var parsed struct {
		Items []json.RawMessage `json:"items"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	assert.Len(t, parsed.Items, 2)
}

func sampleManagedResources(n int) *applicationpkg.ManagedResourcesResponse {
	// Four manifests with managedFields and last-applied-configuration, near a
	// measured 29-resource Argo CD response (~89.5KB).
	pad := strings.Repeat("x", 600)
	manifest := `{"kind":"Deployment","metadata":{"name":"guestbook-ui","annotations":{"keep-me":"yes","kubectl.kubernetes.io/last-applied-configuration":"` + pad + `"},"managedFields":[{"manager":"kubectl","fieldsV1":{"` + pad + `":{}}}]}}`
	items := make([]*v1alpha1.ResourceDiff, n)
	for i := range items {
		items[i] = &v1alpha1.ResourceDiff{
			Group:               "apps",
			Kind:                "Deployment",
			Namespace:           "default",
			Name:                "guestbook-ui",
			ResourceVersion:     "123",
			Modified:            i%2 == 0,
			TargetState:         manifest,
			LiveState:           manifest,
			PredictedLiveState:  manifest,
			NormalizedLiveState: manifest,
		}
	}
	return &applicationpkg.ManagedResourcesResponse{Items: items}
}
