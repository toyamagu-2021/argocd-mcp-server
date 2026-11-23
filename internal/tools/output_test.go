package tools

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestOutputDefaults(t *testing.T) {
	assert.Equal(t, 50, defaultPageLimit)
	assert.Equal(t, 10, defaultManifestLimit)
	assert.Equal(t, 20, defaultTailLines)
	assert.Equal(t, "kubectl.kubernetes.io/last-applied-configuration", lastAppliedAnnotation)
}

func TestPageArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		omitArgs   bool
		def        int
		wantLimit  int
		wantOffset int
		wantErr    string
	}{
		{name: "missing", def: defaultPageLimit, wantLimit: 50, wantOffset: 0},
		{name: "nil arguments", omitArgs: true, def: defaultPageLimit, wantLimit: 50, wantOffset: 0},
		{name: "null limit and offset", args: map[string]any{"limit": nil, "offset": nil}, def: defaultPageLimit, wantLimit: 50, wantOffset: 0},
		{name: "float whole numbers", args: map[string]any{"limit": 10.0, "offset": 2.0}, def: defaultPageLimit, wantLimit: 10, wantOffset: 2},
		{name: "int and int64", args: map[string]any{"limit": 3, "offset": int64(4)}, def: defaultPageLimit, wantLimit: 3, wantOffset: 4},
		{name: "offset omitted", args: map[string]any{"limit": 7}, def: defaultPageLimit, wantLimit: 7, wantOffset: 0},
		{name: "limit zero", args: map[string]any{"limit": 0}, def: defaultPageLimit, wantErr: "limit must be at least 1"},
		{name: "limit zero float", args: map[string]any{"limit": 0.0}, def: defaultPageLimit, wantErr: "limit must be at least 1"},
		{name: "limit negative int", args: map[string]any{"limit": -1}, def: defaultPageLimit, wantErr: "limit must be zero or greater"},
		{name: "limit negative int64", args: map[string]any{"limit": int64(-2)}, def: defaultPageLimit, wantErr: "limit must be zero or greater"},
		{name: "limit negative float", args: map[string]any{"limit": -1.0}, def: defaultPageLimit, wantErr: "limit must be a non-negative integer"},
		{name: "limit fraction", args: map[string]any{"limit": 1.5}, def: defaultPageLimit, wantErr: "limit must be a non-negative integer"},
		{name: "limit string", args: map[string]any{"limit": "10"}, def: defaultPageLimit, wantErr: "limit must be a non-negative integer"},
		{name: "offset negative int", args: map[string]any{"offset": -1}, def: defaultPageLimit, wantErr: "offset must be zero or greater"},
		{name: "offset fraction", args: map[string]any{"offset": 1.5}, def: defaultPageLimit, wantErr: "offset must be a non-negative integer"},
		{name: "offset bool", args: map[string]any{"offset": true}, def: defaultPageLimit, wantErr: "offset must be a non-negative integer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			if !tt.omitArgs {
				req.Params.Arguments = tt.args
			}
			limit, offset, err := pageArgs(req, tt.def)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLimit, limit)
			assert.Equal(t, tt.wantOffset, offset)
		})
	}
}

func TestPageSlice(t *testing.T) {
	items := []string{"a", "b", "c"}
	sliced, total, hasMore := pageSlice(items, 2, 0)
	assert.Equal(t, []string{"a", "b"}, sliced)
	assert.Equal(t, 3, total)
	assert.True(t, hasMore)

	sliced, total, hasMore = pageSlice(items, 2, 1)
	assert.Equal(t, []string{"b", "c"}, sliced)
	assert.Equal(t, 3, total)
	assert.False(t, hasMore)

	originalOffset := 5
	sliced, total, hasMore = pageSlice(items, 10, originalOffset)
	assert.Empty(t, sliced)
	assert.Equal(t, 3, total)
	assert.False(t, hasMore)
	assert.Equal(t, 5, originalOffset)
	assert.Equal(t, "offset 5 is past 3 items.", pageNotice(originalOffset, len(sliced), total, hasMore, ""))
}

func TestPageSliceKeepsNils(t *testing.T) {
	one := 1
	two := 2
	items := []*int{&one, nil, &two}
	sliced, total, hasMore := pageSlice(items, 10, 0)
	assert.Equal(t, 3, total)
	assert.False(t, hasMore)
	require.Len(t, sliced, 3)
	assert.Nil(t, sliced[1])
	assert.Equal(t, 2, *sliced[2])
}

func TestPageSliceEmptyIsNonNilWhenInputIs(t *testing.T) {
	sliced, total, hasMore := pageSlice([]string{}, 50, 0)
	assert.Equal(t, 0, total)
	assert.False(t, hasMore)
	assert.NotNil(t, sliced)
	assert.Empty(t, sliced)

	var nilItems []string
	sliced, total, hasMore = pageSlice(nilItems, 50, 0)
	assert.Equal(t, 0, total)
	assert.False(t, hasMore)
	assert.Nil(t, sliced)
}

func TestPageNotice(t *testing.T) {
	tests := []struct {
		name     string
		offset   int
		returned int
		total    int
		hasMore  bool
		filters  string
		want     string
	}{
		{name: "first page is complete", offset: 0, returned: 3, total: 3, want: ""},
		{name: "empty input", offset: 0, returned: 0, total: 0, want: ""},
		{name: "past the end", offset: 5, returned: 0, total: 3, want: "offset 5 is past 3 items."},
		{
			name: "past the end with managed-resources filters", offset: 5, returned: 0, total: 3,
			filters: managedResourceFilters,
			want:    "offset 5 is past 3 items. Narrow with kind, resource_name, resource_namespace, group, or version.",
		},
		{
			name: "has more without filters", offset: 0, returned: 1, total: 3, hasMore: true,
			want: "Showing items 0-0 of 3. Pass offset=1 to fetch more.",
		},
		{
			name: "has more with filters", offset: 0, returned: 1, total: 3, hasMore: true,
			filters: managedResourceFilters,
			want:    "Showing items 0-0 of 3. Pass offset=1 to fetch more, or narrow with kind, resource_name, resource_namespace, group, or version.",
		},
		{
			name: "showing items 1-2 of 3", offset: 1, returned: 2, total: 3,
			want: "Showing items 1-2 of 3.",
		},
		{
			name: "showing items 1-2 of 3 keeps filters off the last page", offset: 1, returned: 2, total: 3,
			filters: managedResourceFilters,
			want:    "Showing items 1-2 of 3.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, pageNotice(tt.offset, tt.returned, tt.total, tt.hasMore, tt.filters))
		})
	}
}

func TestTailNotice(t *testing.T) {
	assert.Equal(t, "", tailNotice(19, 20))
	assert.Equal(t, "", tailNotice(0, defaultTailLines))
	const full = "Showing the last 20 lines (tail_lines=20). The server does not report the total. Pass a larger tail_lines to see more."
	assert.Equal(t, full, tailNotice(20, 20))
	assert.Equal(t, "Showing the last 21 lines (tail_lines=20). The server does not report the total. Pass a larger tail_lines to see more.", tailNotice(21, 20))
}

func TestWriteCompact(t *testing.T) {
	res, err := writeCompact(page{Items: []string{}, TotalItems: 0})
	require.NoError(t, err)
	require.False(t, res.IsError)
	text, ok := mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Equal(t, `{"items":[],"totalItems":0,"hasMore":false}`, text.Text)

	res, err = writeCompact(page{Items: []string(nil)})
	require.NoError(t, err)
	text, ok = mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Equal(t, `{"items":null,"totalItems":0,"hasMore":false}`, text.Text)

	res, err = writeCompact(make(chan int))
	require.NoError(t, err)
	require.True(t, res.IsError)
	text, ok = mcp.AsTextContent(res.Content[0])
	require.True(t, ok)
	assert.Contains(t, text.Text, "Failed to format response:")
}

func TestStripAnnotationsAndObjectMeta(t *testing.T) {
	assert.Nil(t, stripAnnotations(nil))
	assert.Nil(t, stripAnnotations(map[string]string{}))
	assert.Nil(t, stripAnnotations(map[string]string{lastAppliedAnnotation: "blob"}))
	kept := stripAnnotations(map[string]string{"keep": "yes", lastAppliedAnnotation: "blob"})
	assert.Equal(t, map[string]string{"keep": "yes"}, kept)

	stripObjectMeta(nil)
	meta := &metav1.ObjectMeta{
		Name:          "n",
		ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
		Annotations:   map[string]string{"keep": "yes", lastAppliedAnnotation: "blob"},
	}
	stripObjectMeta(meta)
	assert.Equal(t, "n", meta.Name)
	assert.Nil(t, meta.ManagedFields)
	assert.Equal(t, map[string]string{"keep": "yes"}, meta.Annotations)

	only := &metav1.ObjectMeta{Annotations: map[string]string{lastAppliedAnnotation: "blob"}}
	stripObjectMeta(only)
	assert.Nil(t, only.Annotations)
	assert.Nil(t, only.ManagedFields)
}

func TestStripManifestNoise(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty", raw: "", want: ""},
		{name: "not json", raw: "not-json", want: "not-json"},
		{
			name: "yaml is unchanged",
			raw:  "kind: Deployment\nmetadata:\n  annotations:\n    kubectl.kubernetes.io/last-applied-configuration: blob\n",
			want: "kind: Deployment\nmetadata:\n  annotations:\n    kubectl.kubernetes.io/last-applied-configuration: blob\n",
		},
		{name: "json array", raw: "[1,2]", want: "[1,2]"},
		{name: "json null", raw: "null", want: "null"},
		{
			name: "strips managed fields and last-applied, keeps other annotations",
			raw:  `{"kind":"Deployment","metadata":{"name":"guestbook-ui","annotations":{"keep-me":"yes","kubectl.kubernetes.io/last-applied-configuration":"blob"},"managedFields":[{"manager":"kubectl"}]}}`,
			want: `{"kind":"Deployment","metadata":{"annotations":{"keep-me":"yes"},"name":"guestbook-ui"}}`,
		},
		{
			name: "drops empty annotations",
			raw:  `{"metadata":{"annotations":{"kubectl.kubernetes.io/last-applied-configuration":"blob"},"managedFields":[{"manager":"kubectl"}]}}`,
			want: `{"metadata":{}}`,
		},
		{
			name: "non-object metadata stays",
			raw:  `{"metadata":"nope"}`,
			want: `{"metadata":"nope"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripManifestNoise(tt.raw))
		})
	}
}
